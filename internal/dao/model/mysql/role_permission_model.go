package mysql

import (
	bmodel "authRpc/internal/dao/model"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// RoleModel 管理权限模板，普通CRUD限定当前站点，平台跨站点根模板管理使用独立入口校验身份。
// 所有读写继承请求ctx，不提供用户角色绑定，也不因平台编码自动绕过普通CRUD边界。
type RoleModel struct {
	ctx context.Context
	db  *gorm.DB
}

// TemplatePermissionTreeData 为独立模板配置页提供目录和勾选快照，不依赖或伪造目标用户授权。
type TemplatePermissionTreeData struct {
	Template     *schema.RolePermission   // nil表示尚未创建；查询不创建、恢复或修改模板。
	TemplateType int                      // 1平台默认、2站点默认、3本站普通。
	BusinessCode string                   // 实际模板归属，平台固定99999999，站点不得跨归属。
	Groups       []*schema.PermissionMenu // 平台共享未删除展示分组。
	Menus        []*schema.Menu           // 当前有效目录及完整有效祖先链。
	Selectable   map[int64]bool           // 可勾选范围；未命中不作为权限项返回，必要祖先仅补结构，不读取操作者个人授权作为上限。
}

// GetTemplateTreeData 独立读取模板配置树：1仅平台10，2仅平台11，3仅本站唯一20；无目标用户依赖。
// 1可选全部有效目录，2取平台默认上限，3取平台与本站默认交集；必要上级缺失拒绝，不初始化模板。
// templateID=0时默认模板按归属自动定位，普通模板表示新建；正数必须匹配当前归属及用途，负数拒绝。
func (model *RoleModel) GetTemplateTreeData(templateType int, requested string, templateID int64) (*TemplatePermissionTreeData, error) {
	if templateID < 0 || (templateType != schema.RoleTemplateTypePlatformDefault && templateType != schema.RoleTemplateTypeRoot && templateType != schema.RoleTemplateTypeCustom) {
		return nil, status.Error(codes.InvalidArgument, "templateType必须为1/2/3，templateId不能为负数")
	}
	current, err := model.businessCode("")
	if err != nil {
		return nil, err
	}
	target := requested
	selectable := make(map[int64]bool)
	switch templateType {
	case schema.RoleTemplateTypePlatformDefault:
		target, err = model.businessCode(requested)
		if err != nil {
			return nil, err
		}
		if target != headInfo.PLATFORM_BUSINESS_CODE {
			return nil, status.Error(codes.PermissionDenied, "仅平台10可查询平台默认模板配置")
		}
		operator, err := model.permissionOperator(model.db, current)
		if err != nil {
			return nil, err
		}
		if operator.AccountType != schema.AdminInfoAccountType10 {
			return nil, status.Error(codes.PermissionDenied, "仅平台10可查询平台默认模板配置")
		}
	case schema.RoleTemplateTypeRoot:
		if _, err := model.rootTarget(target); err != nil {
			return nil, err
		}
		operator, err := model.permissionOperator(model.db, current)
		if err != nil {
			return nil, err
		}
		if operator.AccountType != schema.AdminInfoAccountType11 {
			return nil, status.Error(codes.PermissionDenied, "仅平台11可查询站点默认模板配置")
		}
		selectable, err = model.platformCeiling(model.db)
		if err != nil {
			return nil, err
		}
	case schema.RoleTemplateTypeCustom:
		target, err = model.businessCode(requested)
		if err != nil {
			return nil, err
		}
		if err := model.requireCustomManager(model.db); err != nil {
			return nil, err
		}
		selectable, err = model.siteCeiling(model.db, target)
		if err != nil {
			return nil, err
		}
	}
	var info *schema.RolePermission
	if templateType == schema.RoleTemplateTypeCustom {
		if templateID > 0 {
			info, err = model.findByID(model.db, templateID, target)
			if err != nil {
				return nil, err
			}
			if info.TemplateType != schema.RoleTemplateTypeCustom {
				return nil, status.Error(codes.PermissionDenied, "templateId必须为本站普通模板")
			}
		}
	} else {
		info, err = model.findDefaultTemplate(model.db, target, templateType)
		if err != nil {
			if status.Code(err) != codes.FailedPrecondition {
				return nil, err
			}
			// 默认模板软删除仍占唯一名额，不能当作可首次创建；普通软删除作用域同时排除非NULL的0。
			var count int64
			if err := model.db.Unscoped().Model(&schema.RolePermission{}).Where("business_code = ? AND template_type = ?", target, templateType).Count(&count).Error; err != nil {
				return nil, fmt.Errorf("query default template editor state: %w", err)
			}
			if count > 0 {
				return nil, status.Error(codes.FailedPrecondition, "默认模板已存在但不可读取，不能重复创建或自动恢复，请刷新或检查记录状态")
			}
			if templateID > 0 {
				return nil, status.Error(codes.NotFound, "指定默认模板不存在")
			}
			info, err = nil, nil
		} else if templateID > 0 && info.ID != templateID {
			return nil, status.Error(codes.NotFound, "templateId与当前默认模板不匹配")
		}
	}
	data := &TemplatePermissionTreeData{Template: info, TemplateType: templateType, BusinessCode: target, Selectable: selectable, Groups: make([]*schema.PermissionMenu, 0)}
	if err := model.db.Model(&schema.PermissionMenu{}).Where("business_code = ?", headInfo.PLATFORM_BUSINESS_CODE).Order("id ASC").Find(&data.Groups).Error; err != nil {
		return nil, fmt.Errorf("query template editor groups: %w", err)
	}
	permissions := &AdminPermissionModel{ctx: model.ctx, db: model.db}
	data.Menus, err = permissions.ListAvailableMenus()
	if err != nil {
		return nil, err
	}
	if templateType == schema.RoleTemplateTypePlatformDefault {
		for _, menu := range data.Menus {
			data.Selectable[menu.ID] = true
		}
	}
	return data, nil
}

// NewRoleModel 获取请求级数据库会话；只持有共享连接池的会话，不关闭连接池。
func NewRoleModel(ctx context.Context, svcCtx *svc.ServiceContext) (*RoleModel, error) {
	if ctx == nil || svcCtx == nil {
		return nil, errors.New("role model requires request context and service context")
	}
	db, err := svcCtx.DbSelect.GetDb(ctx, bmodel.DB)
	if err != nil {
		return nil, err
	}
	return &RoleModel{ctx: ctx, db: db}, nil
}

// businessCode 只允许当前站点；请求体归属可省略，显式传入时不得覆盖或跨站点查询。
func (model *RoleModel) businessCode(requested string) (string, error) {
	if model == nil || model.db == nil || model.ctx == nil {
		return "", errors.New("role model is not initialized")
	}
	current := headInfo.GetBusinessCode(model.ctx)
	if strings.TrimSpace(current) == "" {
		return "", status.Error(codes.Unauthenticated, "business-code不能为空")
	}
	if current != strings.TrimSpace(current) || !utf8.ValidString(current) || utf8.RuneCountInString(current) > 10 {
		return "", status.Error(codes.InvalidArgument, "business-code格式无效或超过10字符")
	}
	if requested != "" && requested != current {
		return "", status.Error(codes.PermissionDenied, "模板只能在当前站点范围内操作")
	}
	return current, nil
}

// validateStoredRole 拒绝损坏的类型、版本和菜单集合；只规范化返回快照，不在读取时写回数据库。
func validateStoredRole(info *schema.RolePermission, businessCode string) error {
	if info.BusinessCode != businessCode {
		return status.Error(codes.NotFound, "模板不存在或不属于当前站点")
	}
	if info.ID <= 0 || info.Version <= 0 ||
		info.TemplateType != schema.RoleTemplateTypeRoot && info.TemplateType != schema.RoleTemplateTypeCustom && info.TemplateType != schema.RoleTemplateTypePlatformDefault {
		return status.Error(codes.DataLoss, "模板类型或版本异常")
	}
	normalized, _, err := NormalizeMenuIDs(info.MenuIds)
	if err != nil {
		return status.Error(codes.DataLoss, "模板菜单ID格式异常")
	}
	info.MenuIds = normalized
	return nil
}

// findByID 按同站点及软删除作用域普通读取版本快照，查询错误不伪装成空模板。
func (model *RoleModel) findByID(db *gorm.DB, id int64, businessCode string) (*schema.RolePermission, error) {
	query := db.Model(&schema.RolePermission{}).Where("id = ? AND business_code = ?", id, businessCode)
	var info schema.RolePermission
	if err := query.Take(&info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.NotFound, "模板不存在或不属于当前站点")
		}
		return nil, fmt.Errorf("query role template: %w", err)
	}
	if err := validateStoredRole(&info, businessCode); err != nil {
		return nil, err
	}
	return &info, nil
}

// FindByID 仅本站唯一超级管理员20查询本站普通模板3，默认模板通过专用入口读取。
func (model *RoleModel) FindByID(id int64, requestedBusinessCode string) (*schema.RolePermission, error) {
	if id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "模板ID必须为正数")
	}
	businessCode, err := model.businessCode(requestedBusinessCode)
	if err != nil {
		return nil, err
	}
	if err := model.requireCustomManager(model.db); err != nil {
		return nil, err
	}
	info, err := model.findByID(model.db, id, businessCode)
	if err != nil {
		return nil, err
	}
	if info.TemplateType != schema.RoleTemplateTypeCustom {
		return nil, status.Error(codes.PermissionDenied, "默认模板须通过专用管理入口读取")
	}
	return info, nil
}

// GetEffectiveMenuIDs 为模板选择返回当前有效且处于两级默认上限内的权限ID及来源版本，仅本站唯一20可读类型3。
// 查询不改写来源模板；全部失效返回空集合，必要默认模板缺失、异常存储及数据库错误仍返回错误。
func (model *RoleModel) GetEffectiveMenuIDs(id int64, requestedBusinessCode string) ([]int64, int64, error) {
	info, err := model.FindByID(id, requestedBusinessCode)
	if err != nil {
		return nil, 0, err
	}
	allowed, err := model.siteCeiling(model.db, info.BusinessCode)
	if err != nil {
		return nil, 0, err
	}
	ids, err := ParseMenuIDs(info.MenuIds)
	if err != nil {
		return nil, 0, status.Error(codes.DataLoss, "模板菜单ID格式异常")
	}
	_, ids, err = model.filterTemplateMenuIDs(model.db, ids, allowed)
	if err != nil {
		return nil, 0, err
	}
	return ids, info.Version, nil
}

// filterTemplateMenuIDs 供模板选择和事务内复制共用，调用方须先校验来源归属、类型、版本及菜单ID格式。
// 过滤删除、禁用、不存在、祖先失效和上限外项；祖先只校验不授予，返回升序CSV/ID，全部失效返回空集合。
// 只读快照，不修改来源或锁定上级；格式/结构损坏、有效跨目录项和数据库错误不掩盖为空权限。
func (model *RoleModel) filterTemplateMenuIDs(db *gorm.DB, ids []int64, allowed map[int64]bool) (string, []int64, error) {
	if err := model.ctx.Err(); err != nil {
		return "", nil, err
	}
	permissions := &AdminPermissionModel{ctx: model.ctx, db: db}
	_, available, err := permissions.filterSavedMenuIDs(db, ids)
	if err != nil {
		return "", nil, err
	}
	kept := make([]int64, 0, len(available))
	parts := make([]string, 0, len(available))
	for _, id := range available {
		if allowed[id] {
			kept = append(kept, id)
			parts = append(parts, strconv.FormatInt(id, 10))
		}
	}
	return strings.Join(parts, ","), kept, nil
}

// findDefaultTemplate 按归属和类型普通读取唯一默认模板，更新由目标版本条件保护，不显式锁行。
// 缺失、删除或重复不自动创建/恢复，平台固定一份、站点每站点一份，不依赖固定主键。
func (model *RoleModel) findDefaultTemplate(db *gorm.DB, businessCode string, templateType int) (*schema.RolePermission, error) {
	roots := make([]*schema.RolePermission, 0, 2)
	query := db.Model(&schema.RolePermission{}).
		Where("business_code = ? AND template_type = ?", businessCode, templateType).
		Order("id ASC").Limit(2)
	if err := query.Find(&roots).Error; err != nil {
		return nil, fmt.Errorf("query site root template: %w", err)
	}
	if len(roots) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "默认模板尚未配置")
	}
	if len(roots) != 1 {
		return nil, status.Error(codes.DataLoss, "默认模板不唯一")
	}
	if err := validateStoredRole(roots[0], businessCode); err != nil {
		return nil, err
	}
	return roots[0], nil
}

// platformCeiling 读取平台默认模板的具体上限，超级管理员10自身全权限不由此记录授予。
// 缺失或损坏不回退为全部权限；上限按本次事务读取快照校验，不锁定上级记录到提交时刻。
func (model *RoleModel) platformCeiling(db *gorm.DB) (map[int64]bool, error) {
	platform, err := model.findDefaultTemplate(db, headInfo.PLATFORM_BUSINESS_CODE, schema.RoleTemplateTypePlatformDefault)
	if err != nil {
		return nil, err
	}
	ids, err := ParseMenuIDs(platform.MenuIds)
	if err != nil {
		return nil, status.Error(codes.DataLoss, "平台默认模板权限格式异常")
	}
	allowed := make(map[int64]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}
	return allowed, nil
}

// siteCeiling 按平台、站点顺序读取默认上限交集；操作者本人的授权不参与可下发集合计算。
// 普通读取两级快照，不裁剪任一CSV；不保证上级与目标跨记录串行，后续实际访问仍按最新有效上限计算。
func (model *RoleModel) siteCeiling(db *gorm.DB, businessCode string) (map[int64]bool, error) {
	if err := validateTargetSiteCode(businessCode); err != nil {
		return nil, err
	}
	upper, err := model.platformCeiling(db)
	if err != nil {
		return nil, err
	}
	site, err := model.findDefaultTemplate(db, businessCode, schema.RoleTemplateTypeRoot)
	if err != nil {
		return nil, err
	}
	ids, err := ParseMenuIDs(site.MenuIds)
	if err != nil {
		return nil, status.Error(codes.DataLoss, "站点默认模板权限格式异常")
	}
	allowed := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if upper[id] {
			allowed[id] = true
		}
	}
	return allowed, nil
}

// validateMenuCeiling 检查完整提交集合，不把当前有效的越权项静默截断，也不关联操作者本人的权限。
func validateMenuCeiling(ids []int64, allowed map[int64]bool) error {
	for _, id := range ids {
		if !allowed[id] {
			return status.Error(codes.PermissionDenied, "权限不能超过当前默认模板上限")
		}
	}
	return nil
}

// validateCustomMenus 自定义模板按平台与本站默认模板交集校验，具体菜单及完整祖先必须有效。
func (model *RoleModel) validateCustomMenus(tx *gorm.DB, allowed map[int64]bool, ids []int64) error {
	if err := validateMenuCeiling(ids, allowed); err != nil {
		return err
	}
	return model.validateRootMenuTree(tx, ids)
}

// validateCustomWrite 检查完整自定义模板快照；空menu_ids表示清空，不把0类型默认为根模板。
func validateCustomWrite(data *schema.RolePermission) ([]int64, error) {
	if data == nil {
		return nil, status.Error(codes.InvalidArgument, "模板data不能为空")
	}
	if data.TemplateType == schema.RoleTemplateTypeRoot || data.TemplateType == schema.RoleTemplateTypePlatformDefault {
		return nil, status.Error(codes.PermissionDenied, "普通模板接口只允许维护自定义模板")
	}
	if data.TemplateType != schema.RoleTemplateTypeCustom {
		return nil, status.Error(codes.InvalidArgument, "templateType必须为3，自定义模板不能缺省类型")
	}
	if strings.TrimSpace(data.RoleName) == "" || data.RoleName != strings.TrimSpace(data.RoleName) ||
		!utf8.ValidString(data.RoleName) || utf8.RuneCountInString(data.RoleName) > 255 {
		return nil, status.Error(codes.InvalidArgument, "模板名称必填，最多255字符，不能包含首尾空白")
	}
	normalized, ids, err := NormalizeMenuIDs(data.MenuIds)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "menuIds必须为正数ID的逗号字符串，规范化后不超过65535字节")
	}
	data.MenuIds = normalized
	return ids, nil
}

// roleVersionConflict 表示旧版本或并发修改，禁止服务端静默重试覆盖。
func roleVersionConflict() error {
	return status.Error(codes.Aborted, "模板已更新，请刷新后重试")
}

// rootTarget 要求平台上下文和显式目标站点；目标仅用于查询条件，不覆盖ctx或检查外部站点实体。
func (model *RoleModel) rootTarget(target string) (string, error) {
	current, err := model.businessCode("")
	if err != nil {
		return "", err
	}
	if current != headInfo.PLATFORM_BUSINESS_CODE {
		return "", status.Error(codes.PermissionDenied, "仅允许从平台上下文管理站点根模板")
	}
	if err := validateTargetSiteCode(target); err != nil {
		return "", err
	}
	return current, nil
}

// validateTargetSiteCode 校验目标归属，空值不表示默认平台，编码最多10字符且不接受首尾空白。
func validateTargetSiteCode(value string) error {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) ||
		utf8.RuneCountInString(value) > 10 || value == headInfo.PLATFORM_BUSINESS_CODE {
		return status.Error(codes.InvalidArgument, "目标站点编码必填，最多10字符，不能含首尾空白或为平台编码")
	}
	return nil
}

// validatePermissionOperator 只接受归属与ctx一致且正常的明确身份，不按枚举大小推导权限等级。
// 非平台的20、30、40沿用已有身份，不新增层级或免检权限；具体操作许可仍由API鉴权。
func validatePermissionOperator(operator *schema.AdminInfo, businessCode string) error {
	if operator == nil || operator.ID <= 0 || operator.Status != schema.AdminInfoStatus1 || operator.BusinessCode != businessCode {
		return status.Error(codes.PermissionDenied, "操作者不存在、停用或归属不符")
	}
	if businessCode == headInfo.PLATFORM_BUSINESS_CODE {
		if operator.AccountType == schema.AdminInfoAccountType10 || operator.AccountType == schema.AdminInfoAccountType11 {
			return nil
		}
	} else if operator.AccountType == schema.AdminInfoAccountType20 || operator.AccountType == schema.AdminInfoAccountType30 ||
		operator.AccountType == schema.AdminInfoAccountType40 {
		return nil
	}
	return status.Error(codes.PermissionDenied, "操作者身份不允许执行该权限管理入口")
}

// permissionOperator 普通读取真实账号必要身份字段，不锁账号；身份以本次读取时为准，不把metadata当JWT。
func (model *RoleModel) permissionOperator(db *gorm.DB, businessCode string) (*schema.AdminInfo, error) {
	operatorID := headInfo.GetTokenUid(model.ctx)
	if operatorID <= 0 || headInfo.GetTrace(model.ctx) == "" {
		return nil, status.Error(codes.Unauthenticated, "缺少有效操作者或Trace上下文")
	}
	var operator schema.AdminInfo
	query := db.Model(&schema.AdminInfo{}).Select("id", "status", "account_type", "business_code").Where("id = ?", operatorID)
	if err := query.Take(&operator).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.PermissionDenied, "操作者不存在或已删除")
		}
		return nil, fmt.Errorf("query permission operator: %w", err)
	}
	if err := validatePermissionOperator(&operator, businessCode); err != nil {
		return nil, err
	}
	return &operator, nil
}

// CreateSiteRootTemplate 显式创建本站唯一默认模板，完整集合必须属于平台默认模板，不读取操作者本人授权。
func (model *RoleModel) CreateSiteRootTemplate(target, menuIDs string) (*schema.RolePermission, error) {
	return model.writeDefaultTemplate(target, schema.RoleTemplateTypeRoot, menuIDs, 0)
}

// GetSiteRootTemplate 平台管理入口普通读取站点默认模板保存快照，不自动创建或裁剪历史集合。
func (model *RoleModel) GetSiteRootTemplate(target string) (*schema.RolePermission, error) {
	current, err := model.rootTarget(target)
	if err != nil {
		return nil, err
	}
	operator, err := model.permissionOperator(model.db, current)
	if err != nil {
		return nil, err
	}
	if operator.AccountType != schema.AdminInfoAccountType11 {
		return nil, status.Error(codes.PermissionDenied, "仅平台普通管理员可管理站点默认模板")
	}
	return model.findDefaultTemplate(model.db, target, schema.RoleTemplateTypeRoot)
}

// UpdateSiteRootTemplate 按已读版本覆盖站点默认模板，新集合检查目录有效性，不按操作者权限检查旧集合。
// 不联动下级CSV或版本，空字符串清空，审计与权限更新同事务。
func (model *RoleModel) UpdateSiteRootTemplate(target, menuIDs string, version int64) (int64, error) {
	if version <= 0 {
		return 0, status.Error(codes.InvalidArgument, "必须提交正数默认模板版本")
	}
	info, err := model.writeDefaultTemplate(target, schema.RoleTemplateTypeRoot, menuIDs, version)
	if err != nil {
		return 0, err
	}
	return info.Version, nil
}

// CreatePlatformDefaultTemplate 由10显式创建全平台唯一的权限集合，首次版本1；空CSV是无权限，不代表全部。
func (model *RoleModel) CreatePlatformDefaultTemplate(menuIDs string) (*schema.RolePermission, error) {
	return model.writeDefaultTemplate(headInfo.PLATFORM_BUSINESS_CODE, schema.RoleTemplateTypePlatformDefault, menuIDs, 0)
}

// GetPlatformDefaultTemplate 仅10读取平台默认模板保存快照，不自动初始化或返回虚构的全权限描述。
func (model *RoleModel) GetPlatformDefaultTemplate() (*schema.RolePermission, error) {
	current, err := model.businessCode("")
	if err != nil {
		return nil, err
	}
	if current != headInfo.PLATFORM_BUSINESS_CODE {
		return nil, status.Error(codes.PermissionDenied, "仅平台超级管理员10可管理平台默认模板")
	}
	operator, err := model.permissionOperator(model.db, current)
	if err != nil {
		return nil, err
	}
	if operator.AccountType != schema.AdminInfoAccountType10 {
		return nil, status.Error(codes.PermissionDenied, "仅平台超级管理员10可管理平台默认模板")
	}
	return model.findDefaultTemplate(model.db, current, schema.RoleTemplateTypePlatformDefault)
}

// UpdatePlatformDefaultTemplate 按版本覆盖平台默认权限，收权影响下级实际访问，不裁剪历史模板或用户CSV。
func (model *RoleModel) UpdatePlatformDefaultTemplate(menuIDs string, version int64) (int64, error) {
	if version <= 0 {
		return 0, status.Error(codes.InvalidArgument, "必须显式提交正数版本")
	}
	info, err := model.writeDefaultTemplate(headInfo.PLATFORM_BUSINESS_CODE, schema.RoleTemplateTypePlatformDefault, menuIDs, version)
	if err != nil {
		return 0, err
	}
	return info.Version, nil
}

// writeDefaultTemplate 集中默认模板创建/更新，version=0仅供显式创建入口，其他值进行版本比较。
// 仅平台普通管理员11操作站点默认模板，操作者只决定维护资格和审计身份，不读取其本人授权集合。
func (model *RoleModel) writeDefaultTemplate(target string, templateType int, menuIDs string, version int64) (*schema.RolePermission, error) {
	current, err := model.businessCode("")
	if err != nil {
		return nil, err
	}
	if current != headInfo.PLATFORM_BUSINESS_CODE {
		return nil, status.Error(codes.PermissionDenied, "仅平台账号可维护默认模板")
	}
	if templateType == schema.RoleTemplateTypeRoot {
		if err := validateTargetSiteCode(target); err != nil {
			return nil, err
		}
	} else if templateType != schema.RoleTemplateTypePlatformDefault || target != headInfo.PLATFORM_BUSINESS_CODE {
		return nil, status.Error(codes.InvalidArgument, "默认模板类型或归属无效")
	}
	if version < 0 || version == math.MaxInt64 {
		return nil, status.Error(codes.InvalidArgument, "version范围为0到MaxInt64-1")
	}
	normalized, ids, err := NormalizeMenuIDs(menuIDs)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "menuIds须为正数CSV，规范化后不超过65535字节")
	}
	var saved *schema.RolePermission
	err = model.db.Transaction(func(tx *gorm.DB) error {
		operator, err := model.permissionOperator(tx, current)
		if err != nil {
			return err
		}
		if templateType == schema.RoleTemplateTypePlatformDefault {
			if operator.AccountType != schema.AdminInfoAccountType10 {
				return status.Error(codes.PermissionDenied, "仅10可管理平台默认模板")
			}
		} else {
			if operator.AccountType != schema.AdminInfoAccountType11 {
				return status.Error(codes.PermissionDenied, "仅11可管理站点默认模板")
			}
			upper, err := model.platformCeiling(tx)
			if err != nil {
				return err
			}
			if err := validateMenuCeiling(ids, upper); err != nil {
				return err
			}
		}
		beforeIDs, beforeVersion := "", int64(0)
		if version == 0 {
			var count int64
			if err := tx.Unscoped().Model(&schema.RolePermission{}).Where("business_code = ? AND template_type = ?", target, templateType).Count(&count).Error; err != nil {
				return fmt.Errorf("query existing default template: %w", err)
			}
			if count != 0 {
				return status.Error(codes.AlreadyExists, "默认模板已存在，包含已软删除记录，不允许重复创建或恢复")
			}
			name := "站点默认模板"
			if templateType == schema.RoleTemplateTypePlatformDefault {
				name = "平台默认模板"
			}
			saved = &schema.RolePermission{RoleName: name, BusinessCode: target, TemplateType: templateType, MenuIds: normalized, Version: 1}
		} else {
			saved, err = model.findDefaultTemplate(tx, target, templateType)
			if err != nil {
				return err
			}
			if saved.Version != version {
				return roleVersionConflict()
			}
			beforeIDs, beforeVersion = saved.MenuIds, saved.Version
		}
		if err := model.validateRootMenuTree(tx, ids); err != nil {
			return err
		}
		if version == 0 {
			if err := tx.Create(saved).Error; err != nil {
				var duplicate *mysqlDriver.MySQLError
				if errors.Is(err, gorm.ErrDuplicatedKey) || errors.As(err, &duplicate) && duplicate.Number == 1062 {
					return status.Error(codes.AlreadyExists, "默认模板已存在，不允许重复创建")
				}
				return fmt.Errorf("create default template: %w", err)
			}
		} else {
			result := tx.Model(&schema.RolePermission{}).Where("id = ? AND business_code = ? AND template_type = ? AND version = ?", saved.ID, target, templateType, version).
				Updates(map[string]interface{}{"menu_ids": normalized, "version": gorm.Expr("version + 1")})
			if result.Error != nil {
				return fmt.Errorf("update default template: %w", result.Error)
			}
			if result.RowsAffected != 1 {
				return roleVersionConflict()
			}
			saved.MenuIds, saved.Version = normalized, version+1
		}
		operation, targetType := schema.PermissionAuditCreateSiteRootTemplate, schema.PermissionAuditTargetSiteRootTemplate
		if version != 0 {
			operation = schema.PermissionAuditUpdateSiteRootTemplate
		}
		if templateType == schema.RoleTemplateTypePlatformDefault {
			targetType, operation = schema.PermissionAuditTargetPlatformDefault, schema.PermissionAuditCreatePlatformDefault
			if version != 0 {
				operation = schema.PermissionAuditUpdatePlatformDefault
			}
		}
		audit := &schema.PermissionAudit{Operation: operation, OperatorID: operator.ID, OperatorAccountType: operator.AccountType, OperatorBusinessCode: operator.BusinessCode,
			BusinessCode: target, TargetType: targetType, TargetID: saved.ID, BeforeMenuIDs: beforeIDs, AfterMenuIDs: normalized,
			BeforeVersion: beforeVersion, AfterVersion: saved.Version, TraceID: headInfo.GetTrace(model.ctx)}
		if err := tx.Create(audit).Error; err != nil {
			return fmt.Errorf("audit default template write: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// validateRootMenuTree 普通读取所选共享菜单及完整祖先链，检查快照中未删除、启用、类型及服务名有效。
// 祖先仅用于有效性检查，不自动写入授权；缺失、停用、跨归属或循环结构均拒绝，空集合合法。
func (model *RoleModel) validateRootMenuTree(tx *gorm.DB, ids []int64) error {
	loaded := make(map[int64]*schema.Menu, len(ids))
	pending := append([]int64(nil), ids...)
	for len(pending) != 0 {
		if err := model.ctx.Err(); err != nil {
			return err
		}
		sort.Slice(pending, func(i, j int) bool { return pending[i] < pending[j] })
		var rows []schema.Menu
		if err := tx.Model(&schema.Menu{}).Select("id", "menu_pid", "menu_type", "status", "service_name", "business_code").
			Where("id IN ? AND business_code = ?", pending, headInfo.PLATFORM_BUSINESS_CODE).
			Order("id ASC").Find(&rows).Error; err != nil {
			return fmt.Errorf("query site root menu tree: %w", err)
		}
		if len(rows) != len(pending) {
			return status.Error(codes.FailedPrecondition, "菜单或祖先不存在、已删除或不属于共享平台目录")
		}
		for i := range rows {
			menu := &rows[i]
			if menu.Status != schema.MenuStatus1 || menu.MenuPid < 0 ||
				menu.MenuType != schema.MenuMenuType1 && menu.MenuType != schema.MenuMenuType2 {
				return status.Error(codes.FailedPrecondition, "菜单或祖先必须为已启用且结构有效的页面或按钮")
			}
			if err := ValidateServiceName(menu.ServiceName); err != nil {
				return status.Error(codes.FailedPrecondition, "菜单或祖先服务名无效")
			}
			loaded[menu.ID] = menu
		}
		next := make(map[int64]bool)
		for i := range rows {
			if parent := rows[i].MenuPid; parent > 0 && loaded[parent] == nil {
				next[parent] = true
			}
		}
		pending = pending[:0]
		for id := range next {
			pending = append(pending, id)
		}
	}
	return ValidateMenuParents(model.ctx, loaded)
}

// requireCustomManager 校验普通模板管理入口，仅本站唯一且正常的account_type20可使用，不读取本人权限集合。
// 30/40及平台10/11均无本站普通模板管理资格；具体操作许可继续由API校验，内部上限读取不调用此入口。
func (model *RoleModel) requireCustomManager(db *gorm.DB) error {
	current, err := model.businessCode("")
	if err != nil {
		return err
	}
	if err := validateTargetSiteCode(current); err != nil {
		return status.Error(codes.PermissionDenied, "自定义模板管理必须使用站点账号")
	}
	operator, err := model.permissionOperator(db, current)
	if err != nil {
		return err
	}
	if operator.AccountType != schema.AdminInfoAccountType20 {
		return status.Error(codes.PermissionDenied, "仅站点超级管理员20可管理本站普通模板")
	}
	return requireUniqueSiteSuper(db, current, operator.ID)
}

// InsertSchema 仅创建本站自定义模板，按读取到的默认上限校验，首次版本1；不锁上级或初始化默认模板。
func (model *RoleModel) InsertSchema(data *schema.RolePermission) error {
	ids, err := validateCustomWrite(data)
	if err != nil {
		return err
	}
	if data.ID != 0 || data.Version != 0 {
		return status.Error(codes.InvalidArgument, "新增模板不能指定ID或已有版本")
	}
	businessCode, err := model.businessCode(data.BusinessCode)
	if err != nil {
		return err
	}
	return model.db.Transaction(func(tx *gorm.DB) error {
		if err := model.requireCustomManager(tx); err != nil {
			return err
		}
		allowed, err := model.siteCeiling(tx, businessCode)
		if err != nil {
			return err
		}
		if err := model.validateCustomMenus(tx, allowed, ids); err != nil {
			return err
		}
		data.BusinessCode, data.Version = businessCode, 1
		data.CreatedAt, data.UpdatedAt = 0, 0
		if err := tx.Create(data).Error; err != nil {
			return fmt.Errorf("create custom role template: %w", err)
		}
		audit := &schema.PermissionAudit{Operation: schema.PermissionAuditCreateCustomTemplate,
			OperatorID: headInfo.GetTokenUid(model.ctx), OperatorAccountType: schema.AdminInfoAccountType20, OperatorBusinessCode: businessCode,
			BusinessCode: businessCode, TargetType: schema.PermissionAuditTargetCustomTemplate, TargetID: data.ID,
			BeforeMenuIDs: "", AfterMenuIDs: data.MenuIds, BeforeVersion: 0, AfterVersion: 1, TraceID: headInfo.GetTrace(model.ctx)}
		if err := tx.Create(audit).Error; err != nil {
			return fmt.Errorf("audit create custom template: %w", err)
		}
		return nil
	})
}

// UpdateSchema 在写事务中普通读取上限及目标，按旧版本条件完整覆盖；零行冲突返回Aborted，不重试。
// 不修改站点或模板类型，不裁剪用户及其他模板记录，空menu_ids明确清空。
func (model *RoleModel) UpdateSchema(data *schema.RolePermission) (int64, error) {
	ids, err := validateCustomWrite(data)
	if err != nil {
		return 0, err
	}
	if data.ID <= 0 || data.Version <= 0 || data.Version == math.MaxInt64 {
		return 0, status.Error(codes.InvalidArgument, "模板ID和版本必须为正数，版本不能达到MaxInt64")
	}
	businessCode, err := model.businessCode(data.BusinessCode)
	if err != nil {
		return 0, err
	}
	err = model.db.Transaction(func(tx *gorm.DB) error {
		if err := model.requireCustomManager(tx); err != nil {
			return err
		}
		allowed, err := model.siteCeiling(tx, businessCode)
		if err != nil {
			return err
		}
		current, err := model.findByID(tx, data.ID, businessCode)
		if err != nil {
			return err
		}
		if current.TemplateType != schema.RoleTemplateTypeCustom {
			return status.Error(codes.PermissionDenied, "站点根模板不能通过普通接口修改")
		}
		if current.Version != data.Version {
			return roleVersionConflict()
		}
		if err := model.validateCustomMenus(tx, allowed, ids); err != nil {
			return err
		}
		result := tx.Model(&schema.RolePermission{}).
			Where("id = ? AND business_code = ? AND template_type = ? AND version = ?",
				data.ID, businessCode, schema.RoleTemplateTypeCustom, data.Version).
			Updates(map[string]interface{}{
				"role_name": data.RoleName, "menu_ids": data.MenuIds, "version": gorm.Expr("version + 1"),
			})
		if result.Error != nil {
			return fmt.Errorf("update custom role template: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return roleVersionConflict()
		}
		audit := &schema.PermissionAudit{Operation: schema.PermissionAuditUpdateCustomTemplate,
			OperatorID: headInfo.GetTokenUid(model.ctx), OperatorAccountType: schema.AdminInfoAccountType20, OperatorBusinessCode: businessCode,
			BusinessCode: businessCode, TargetType: schema.PermissionAuditTargetCustomTemplate, TargetID: current.ID,
			BeforeMenuIDs: current.MenuIds, AfterMenuIDs: data.MenuIds, BeforeVersion: current.Version, AfterVersion: current.Version + 1, TraceID: headInfo.GetTrace(model.ctx)}
		if err := tx.Create(audit).Error; err != nil {
			return fmt.Errorf("audit update custom template: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return data.Version + 1, nil
}

// DeleteByID 仅软删除同站点自定义模板；先按版本递增，再由模型Delete写Unix秒及UpdatedAt，整个事务原子提交。
// 默认模板1/2创建后均不可删除；不检查用户绑定，不修改已复制用户权限，删除普通模板不依赖默认模板仍存在。
func (model *RoleModel) DeleteByID(id, version int64) error {
	if id <= 0 || version <= 0 || version == math.MaxInt64 {
		return status.Error(codes.InvalidArgument, "模板ID和版本必须为正数，版本不能达到MaxInt64")
	}
	businessCode, err := model.businessCode("")
	if err != nil {
		return err
	}
	return model.db.Transaction(func(tx *gorm.DB) error {
		if err := model.requireCustomManager(tx); err != nil {
			return err
		}
		current, err := model.findByID(tx, id, businessCode)
		if err != nil {
			return err
		}
		if current.TemplateType != schema.RoleTemplateTypeCustom {
			return status.Error(codes.PermissionDenied, "默认模板创建后不可删除，仅允许删除普通模板")
		}
		if current.Version != version {
			return roleVersionConflict()
		}
		updated := tx.Model(&schema.RolePermission{}).
			Where("id = ? AND business_code = ? AND version = ?", id, businessCode, version).
			Update("version", gorm.Expr("version + 1"))
		if updated.Error != nil {
			return fmt.Errorf("update deleted template version: %w", updated.Error)
		}
		if updated.RowsAffected != 1 {
			return roleVersionConflict()
		}
		deleted := tx.Where("id = ? AND business_code = ? AND version = ?", id, businessCode, version+1).
			Delete(&schema.RolePermission{})
		if deleted.Error != nil {
			return fmt.Errorf("delete custom role template: %w", deleted.Error)
		}
		if deleted.RowsAffected != 1 {
			return roleVersionConflict()
		}
		// 软删除仅改变可用状态，CSV保留用于审计及历史追踪，操作名明确表示模板已删除。
		audit := &schema.PermissionAudit{Operation: schema.PermissionAuditDeleteCustomTemplate,
			OperatorID: headInfo.GetTokenUid(model.ctx), OperatorAccountType: schema.AdminInfoAccountType20, OperatorBusinessCode: businessCode,
			BusinessCode: businessCode, TargetType: schema.PermissionAuditTargetCustomTemplate, TargetID: current.ID,
			BeforeMenuIDs: current.MenuIds, AfterMenuIDs: current.MenuIds, BeforeVersion: current.Version, AfterVersion: current.Version + 1, TraceID: headInfo.GetTrace(model.ctx)}
		if err := tx.Create(audit).Error; err != nil {
			return fmt.Errorf("audit delete custom template: %w", err)
		}
		return nil
	})
}

// GetList 普通查询同站点模板，不截取业务编码，不展开分组；类型0或3只返回自定义模板、页码0默认1、每页0默认20且最多200。
// STime/ETime为创建时间的Unix秒闭区间，0表示不限制；查询不额外开启事务，不承诺计数和列表是同一快照。
func (model *RoleModel) GetList(in *schema.RolePermission, pageQuery *bmodel.PageQuery, startTime, endTime int64) (int64, []*schema.RolePermission, error) {
	if in == nil || pageQuery == nil || in.ID < 0 ||
		in.TemplateType != 0 && in.TemplateType != schema.RoleTemplateTypeRoot && in.TemplateType != schema.RoleTemplateTypeCustom && in.TemplateType != schema.RoleTemplateTypePlatformDefault {
		return 0, nil, status.Error(codes.InvalidArgument, "模板筛选条件无效")
	}
	if startTime < 0 || endTime < 0 || startTime > 0 && endTime > 0 && startTime > endTime {
		return 0, nil, status.Error(codes.InvalidArgument, "创建时间范围无效")
	}
	if !utf8.ValidString(in.RoleName) || utf8.RuneCountInString(in.RoleName) > 255 {
		return 0, nil, status.Error(codes.InvalidArgument, "模板名称筛选最多255个UTF-8字符")
	}
	page, size := pageQuery.Page, pageQuery.PageSize
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	if page < 1 || size < 1 || size > 200 || page-1 > math.MaxInt64/size {
		return 0, nil, status.Error(codes.InvalidArgument, "页码必须为正数，pageSize范围为1到200")
	}
	businessCode, err := model.businessCode(in.BusinessCode)
	if err != nil {
		return 0, nil, err
	}
	if err := model.requireCustomManager(model.db); err != nil {
		return 0, nil, err
	}
	if in.TemplateType != 0 && in.TemplateType != schema.RoleTemplateTypeCustom {
		return 0, nil, status.Error(codes.PermissionDenied, "普通列表仅用于站点自定义模板，默认模板使用专用入口")
	}
	query := model.db.Model(&schema.RolePermission{}).Where("business_code = ? AND template_type = ?", businessCode, schema.RoleTemplateTypeCustom)
	if in.ID > 0 {
		query = query.Where("id = ?", in.ID)
	}
	if in.RoleName != "" {
		query = query.Where("role_name LIKE ?", "%"+in.RoleName+"%")
	}
	if in.TemplateType != 0 {
		query = query.Where("template_type = ?", in.TemplateType)
	}
	if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("count role templates: %w", err)
	}
	rows := make([]*schema.RolePermission, 0)
	if err := query.Order("id ASC").Offset(int((page - 1) * size)).Limit(int(size)).Find(&rows).Error; err != nil {
		return 0, nil, fmt.Errorf("list role templates: %w", err)
	}
	for _, row := range rows {
		if err := validateStoredRole(row, businessCode); err != nil {
			return 0, nil, err
		}
	}
	return total, rows, nil
}
