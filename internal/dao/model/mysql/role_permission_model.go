package mysql

import (
	bmodel "authRpc/internal/dao/model"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RoleModel 管理当前站点的权限模板，读写继承请求ctx，不提供用户角色绑定或平台编码绕过。
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

// lockRoot 在写事务中锁定当前站点唯一根模板；缺失或重复均拒绝，不从固定分组、自定义模板补齐上限。
func (model *RoleModel) lockRoot(tx *gorm.DB, businessCode string) (*schema.RolePermission, error) {
	roots := make([]*schema.RolePermission, 0, 2)
	if err := tx.Model(&schema.RolePermission{}).
		Where("business_code = ? AND template_type = ?", businessCode, schema.RoleTemplateTypeRoot).
		Order("id ASC").Limit(2).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&roots).Error; err != nil {
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
		root, err := model.lockRoot(tx, businessCode)
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
		root, err := model.lockRoot(tx, businessCode)
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
