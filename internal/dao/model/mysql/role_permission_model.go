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
	"strings"
	"unicode/utf8"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RoleModel 管理权限模板，普通CRUD限定当前站点，平台跨站点根模板管理使用独立入口校验身份。
// 所有读写继承请求ctx，不提供用户角色绑定，也不因平台编码自动绕过普通CRUD边界。
type RoleModel struct {
	ctx context.Context
	db  *gorm.DB
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
		info.TemplateType != schema.RoleTemplateTypeRoot && info.TemplateType != schema.RoleTemplateTypeCustom {
		return status.Error(codes.DataLoss, "模板类型或版本异常")
	}
	normalized, _, err := NormalizeMenuIDs(info.MenuIds)
	if err != nil {
		return status.Error(codes.DataLoss, "模板菜单ID格式异常")
	}
	info.MenuIds = normalized
	return nil
}

// findByID 使用同站点及模型软删除作用域；写事务可要求行锁，查询错误不伪装成空模板。
func (model *RoleModel) findByID(db *gorm.DB, id int64, businessCode string, lock bool) (*schema.RolePermission, error) {
	query := db.Model(&schema.RolePermission{}).Where("id = ? AND business_code = ?", id, businessCode)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
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

// FindByID 普通查询当前站点未删除模板，可读取根模板，但不因此允许通过普通CRUD修改根模板。
func (model *RoleModel) FindByID(id int64, requestedBusinessCode string) (*schema.RolePermission, error) {
	if id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "模板ID必须为正数")
	}
	businessCode, err := model.businessCode(requestedBusinessCode)
	if err != nil {
		return nil, err
	}
	return model.findByID(model.db, id, businessCode, false)
}

// findRoot 查询目标站点唯一有效根模板；写事务要求行锁，普通读取不开事务、不自动初始化或恢复。
func (model *RoleModel) findRoot(db *gorm.DB, businessCode string, lock bool) (*schema.RolePermission, error) {
	roots := make([]*schema.RolePermission, 0, 2)
	query := db.Model(&schema.RolePermission{}).
		Where("business_code = ? AND template_type = ?", businessCode, schema.RoleTemplateTypeRoot).
		Order("id ASC").Limit(2)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Find(&roots).Error; err != nil {
		return nil, fmt.Errorf("query site root template: %w", err)
	}
	if len(roots) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "站点根模板尚未配置")
	}
	if len(roots) != 1 {
		return nil, status.Error(codes.DataLoss, "站点根模板不唯一")
	}
	if err := validateStoredRole(roots[0], businessCode); err != nil {
		return nil, err
	}
	return roots[0], nil
}

// validateCustomMenus 先检查根模板子集，再按共享目录主键检查菜单存在、启用及页面/按钮类型。
// 授予范围由当前站点根模板的具体ID限定，不要求为每个站点复制相同菜单，不读取菜单分组。
func (model *RoleModel) validateCustomMenus(tx *gorm.DB, root *schema.RolePermission, ids []int64) error {
	rootIDs, err := ParseMenuIDs(root.MenuIds)
	if err != nil {
		return status.Error(codes.DataLoss, "站点根模板菜单ID格式异常")
	}
	allowed := make(map[int64]bool, len(rootIDs))
	for _, id := range rootIDs {
		allowed[id] = true
	}
	for _, id := range ids {
		if !allowed[id] {
			return status.Error(codes.PermissionDenied, "模板权限不能超过当前站点根权限")
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var menus []schema.Menu
	if err := tx.Model(&schema.Menu{}).Where("id IN ?", ids).
		Order("id ASC").Clauses(clause.Locking{Strength: "SHARE"}).Find(&menus).Error; err != nil {
		return fmt.Errorf("query template menus: %w", err)
	}
	if len(menus) != len(ids) {
		return status.Error(codes.InvalidArgument, "模板包含不存在或已删除的菜单")
	}
	for _, menu := range menus {
		if menu.Status != schema.MenuStatus1 ||
			menu.MenuType != schema.MenuMenuType1 && menu.MenuType != schema.MenuMenuType2 {
			return status.Error(codes.InvalidArgument, "模板只能授予已启用的页面或按钮")
		}
	}
	return nil
}

// validateCustomWrite 检查完整自定义模板快照；空menu_ids表示清空，不把0类型默认为根模板。
func validateCustomWrite(data *schema.RolePermission) ([]int64, error) {
	if data == nil {
		return nil, status.Error(codes.InvalidArgument, "模板data不能为空")
	}
	if data.TemplateType == schema.RoleTemplateTypeRoot {
		return nil, status.Error(codes.PermissionDenied, "普通模板接口只允许维护自定义模板")
	}
	if data.TemplateType != schema.RoleTemplateTypeCustom {
		return nil, status.Error(codes.InvalidArgument, "templateType必须为2，自定义模板不能缺省类型")
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

// permissionOperator 仅取必要身份字段，写事务锁账号，普通查询不加锁；不把metadata当作JWT验证结果。
func (model *RoleModel) permissionOperator(db *gorm.DB, businessCode string, lock bool) (*schema.AdminInfo, error) {
	operatorID := headInfo.GetTokenUid(model.ctx)
	if operatorID <= 0 || headInfo.GetTrace(model.ctx) == "" {
		return nil, status.Error(codes.Unauthenticated, "缺少有效操作者或Trace上下文")
	}
	var operator schema.AdminInfo
	query := db.Model(&schema.AdminInfo{}).Select("id", "status", "account_type", "business_code").Where("id = ?", operatorID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
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

// CreateSiteRootTemplate 由平台账号显式创建目标站点根模板；名称固定、类型1、初始版本1。
// 操作者来自可信调用链的ctx并在事务中重新查询身份，目标编码不覆盖ctx；不检查外部站点实体是否存在。
// 已删除根模板也占用唯一名额；创建与成功审计同事务，失败不自动恢复、不覆盖、不返回未提交ID。
func (model *RoleModel) CreateSiteRootTemplate(targetBusinessCode, menuIDs string) (*schema.RolePermission, error) {
	current, err := model.rootTarget(targetBusinessCode)
	if err != nil {
		return nil, err
	}
	normalized, ids, err := NormalizeMenuIDs(menuIDs)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "menuIds必须为正数ID的逗号字符串，规范化后不超过65535字节")
	}
	operatorID, traceID := headInfo.GetTokenUid(model.ctx), headInfo.GetTrace(model.ctx)
	if operatorID <= 0 || traceID == "" {
		return nil, status.Error(codes.Unauthenticated, "缺少有效操作者或Trace上下文")
	}
	created := &schema.RolePermission{
		RoleName: "站点根模板", MenuIds: normalized, TemplateType: schema.RoleTemplateTypeRoot,
		Version: 1, BusinessCode: targetBusinessCode,
	}
	err = model.db.Transaction(func(tx *gorm.DB) error {
		if err := model.ctx.Err(); err != nil {
			return err
		}
		operator, err := model.permissionOperator(tx, current, true)
		if err != nil {
			return fmt.Errorf("query root template operator: %w", err)
		}
		if operator.AccountType == schema.AdminInfoAccountType11 {
			if err := model.validateOperatorMenus(tx, operatorID, current, ids); err != nil {
				return err
			}
		}
		// 不对缺失根记录加间隙锁，避免并发首次创建互相升级锁；最终由uniq_site_root兜底。
		var existing int64
		if err := tx.Unscoped().Model(&schema.RolePermission{}).
			Where("business_code = ? AND template_type = ?", targetBusinessCode, schema.RoleTemplateTypeRoot).
			Count(&existing).Error; err != nil {
			return fmt.Errorf("query existing site root template: %w", err)
		}
		if existing != 0 {
			return status.Error(codes.AlreadyExists, "站点根模板已存在，不允许重复创建或恢复")
		}
		if err := model.validateRootMenuTree(tx, ids); err != nil {
			return err
		}
		if err := tx.Create(created).Error; err != nil {
			var duplicate *mysqlDriver.MySQLError
			if errors.Is(err, gorm.ErrDuplicatedKey) || errors.As(err, &duplicate) && duplicate.Number == 1062 {
				return status.Error(codes.AlreadyExists, "站点根模板已存在，不允许重复创建")
			}
			return fmt.Errorf("create site root template: %w", err)
		}
		audit := &schema.PermissionAudit{
			Operation:  schema.PermissionAuditCreateSiteRootTemplate,
			OperatorID: operator.ID, OperatorAccountType: operator.AccountType, OperatorBusinessCode: operator.BusinessCode,
			BusinessCode: targetBusinessCode, TargetType: schema.PermissionAuditTargetSiteRootTemplate, TargetID: created.ID,
			BeforeMenuIDs: "", AfterMenuIDs: normalized, BeforeVersion: 0, AfterVersion: 1, TraceID: traceID,
		}
		if err := tx.Create(audit).Error; err != nil {
			return fmt.Errorf("create site root template audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// GetSiteRootTemplate 由平台专用入口普通读取保存快照；不自动创建，不过滤或改写历史权限集合。
// 身份与模板分别查询，不宣称跨查询快照一致；API仍须校验读取根权限的操作许可。
func (model *RoleModel) GetSiteRootTemplate(targetBusinessCode string) (*schema.RolePermission, error) {
	current, err := model.rootTarget(targetBusinessCode)
	if err != nil {
		return nil, err
	}
	if _, err := model.permissionOperator(model.db, current, false); err != nil {
		return nil, err
	}
	return model.findRoot(model.db, targetBusinessCode, false)
}

// UpdateSiteRootTemplate 按已读版本完整覆盖根集合，空字符串清空，不改名称、类型或归属，不级联裁剪。
// 普通平台管理员须能覆盖当前旧集合和提交新集合，不能只检查新增项；成功变更与审计同事务。
func (model *RoleModel) UpdateSiteRootTemplate(targetBusinessCode, menuIDs string, version int64) (int64, error) {
	if version <= 0 || version == math.MaxInt64 {
		return 0, status.Error(codes.InvalidArgument, "根模板version必须在1到MaxInt64-1范围内")
	}
	current, err := model.rootTarget(targetBusinessCode)
	if err != nil {
		return 0, err
	}
	normalized, ids, err := NormalizeMenuIDs(menuIDs)
	if err != nil {
		return 0, status.Error(codes.InvalidArgument, "menuIds必须为正数ID的逗号字符串，规范化后不超过65535字节")
	}
	err = model.db.Transaction(func(tx *gorm.DB) error {
		operator, err := model.permissionOperator(tx, current, true)
		if err != nil {
			return err
		}
		root, err := model.findRoot(tx, targetBusinessCode, true)
		if err != nil {
			return err
		}
		if root.Version != version {
			return roleVersionConflict()
		}
		if operator.AccountType == schema.AdminInfoAccountType11 {
			oldIDs, err := ParseMenuIDs(root.MenuIds)
			if err != nil {
				return status.Error(codes.DataLoss, "根模板菜单ID格式异常")
			}
			if err := model.validateOperatorMenus(tx, operator.ID, current, oldIDs, ids); err != nil {
				return err
			}
		}
		if err := model.validateRootMenuTree(tx, ids); err != nil {
			return err
		}
		result := tx.Model(&schema.RolePermission{}).
			Where("id = ? AND business_code = ? AND template_type = ? AND version = ?",
				root.ID, targetBusinessCode, schema.RoleTemplateTypeRoot, version).
			Updates(map[string]interface{}{"menu_ids": normalized, "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return fmt.Errorf("update site root template: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return roleVersionConflict()
		}
		audit := &schema.PermissionAudit{
			Operation:  schema.PermissionAuditUpdateSiteRootTemplate,
			OperatorID: operator.ID, OperatorAccountType: operator.AccountType, OperatorBusinessCode: operator.BusinessCode,
			BusinessCode: targetBusinessCode, TargetType: schema.PermissionAuditTargetSiteRootTemplate, TargetID: root.ID,
			BeforeMenuIDs: root.MenuIds, AfterMenuIDs: normalized, BeforeVersion: version, AfterVersion: version + 1,
			TraceID: headInfo.GetTrace(model.ctx),
		}
		if err := tx.Create(audit).Error; err != nil {
			return fmt.Errorf("audit site root template update: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return version + 1, nil
}

// validateOperatorMenus 锁定操作者当前独立授权，各完整集合均不得超出本人；平台超级管理员由调用方跳过此项。
// 空授权不代表全部权限；缺失、删除或损坏的授权均拒绝，不从模板或用户传入的账号类型推导。
func (model *RoleModel) validateOperatorMenus(tx *gorm.DB, operatorID int64, businessCode string, selections ...[]int64) error {
	var grant schema.AdminPermission
	err := tx.Model(&schema.AdminPermission{}).Where("user_id = ?", operatorID).
		Clauses(clause.Locking{Strength: "SHARE"}).Take(&grant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return status.Error(codes.PermissionDenied, "操作者未配置有效授权")
	}
	if err != nil {
		return fmt.Errorf("query root template operator grant: %w", err)
	}
	if grant.BusinessCode != businessCode || grant.ID <= 0 || grant.Version <= 0 {
		return status.Error(codes.DataLoss, "操作者授权归属或版本异常")
	}
	_, grantedIDs, err := NormalizeMenuIDs(grant.MenuIds)
	if err != nil {
		return status.Error(codes.DataLoss, "操作者授权菜单格式异常")
	}
	allowed := make(map[int64]bool, len(grantedIDs))
	for _, id := range grantedIDs {
		allowed[id] = true
	}
	for _, ids := range selections {
		for _, id := range ids {
			if !allowed[id] {
				return status.Error(codes.PermissionDenied, "完整授权集合不能超过操作者本人当前权限")
			}
		}
	}
	return nil
}

// validateRootMenuTree 锁定所选共享平台菜单及完整祖先链，要求未删除、已启用的页面或按钮且服务名有效。
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
			Order("id ASC").Clauses(clause.Locking{Strength: "SHARE"}).Find(&rows).Error; err != nil {
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

// InsertSchema 仅创建当前站点自定义模板，锁定根模板检查上限，首次版本固定1；不初始化根模板。
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
		root, err := model.findRoot(tx, businessCode, true)
		if err != nil {
			return err
		}
		if err := model.validateCustomMenus(tx, root, ids); err != nil {
			return err
		}
		data.BusinessCode, data.Version = businessCode, 1
		data.CreatedAt, data.UpdatedAt = 0, 0
		if err := tx.Create(data).Error; err != nil {
			return fmt.Errorf("create custom role template: %w", err)
		}
		return nil
	})
}

// UpdateSchema 在短写事务中先锁根模板再锁目标，校验版本、类型和上限，完整覆盖名称与menu_ids。
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
		root, err := model.findRoot(tx, businessCode, true)
		if err != nil {
			return err
		}
		current, err := model.findByID(tx, data.ID, businessCode, true)
		if err != nil {
			return err
		}
		if current.TemplateType != schema.RoleTemplateTypeCustom {
			return status.Error(codes.PermissionDenied, "站点根模板不能通过普通接口修改")
		}
		if current.Version != data.Version {
			return roleVersionConflict()
		}
		if err := model.validateCustomMenus(tx, root, ids); err != nil {
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
		return nil
	})
	if err != nil {
		return 0, err
	}
	return data.Version + 1, nil
}

// DeleteByID 仅软删除同站点自定义模板；先按版本递增，再由模型Delete写Unix秒及UpdatedAt，整个事务原子提交。
// 根模板禁止删除，不检查用户绑定，也不修改已复制用户的独立权限；清理自定义模板无需依赖根模板仍存在。
func (model *RoleModel) DeleteByID(id, version int64) error {
	if id <= 0 || version <= 0 || version == math.MaxInt64 {
		return status.Error(codes.InvalidArgument, "模板ID和版本必须为正数，版本不能达到MaxInt64")
	}
	businessCode, err := model.businessCode("")
	if err != nil {
		return err
	}
	return model.db.Transaction(func(tx *gorm.DB) error {
		current, err := model.findByID(tx, id, businessCode, true)
		if err != nil {
			return err
		}
		if current.TemplateType != schema.RoleTemplateTypeCustom {
			return status.Error(codes.PermissionDenied, "站点根模板不允许普通删除")
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
		return nil
	})
}

// GetList 普通查询同站点模板，不截取业务编码，不展开分组；类型0为全部、页码0默认1、每页0默认20且最多200。
// STime/ETime为创建时间的Unix秒闭区间，0表示不限制；查询不额外开启事务，不承诺计数和列表是同一快照。
func (model *RoleModel) GetList(in *schema.RolePermission, pageQuery *bmodel.PageQuery, startTime, endTime int64) (int64, []*schema.RolePermission, error) {
	if in == nil || pageQuery == nil || in.ID < 0 ||
		in.TemplateType != 0 && in.TemplateType != schema.RoleTemplateTypeRoot && in.TemplateType != schema.RoleTemplateTypeCustom {
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
	query := model.db.Model(&schema.RolePermission{}).Where("business_code = ?", businessCode)
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
