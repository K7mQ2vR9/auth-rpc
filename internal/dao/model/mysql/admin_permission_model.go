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

	mysqlDriver "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// AdminPermissionModel 持有绑定请求上下文的会话，不读角色权限，不关闭共享连接池。
type AdminPermissionModel struct {
	db *gorm.DB
}

// UserPermissionTreeData 汇总依次普通查询取得的用户权限和完整目录，不承诺跨查询快照一致。
type UserPermissionTreeData struct {
	// Permission 为nil表示从未保存；包含软删除记录时仍保留版本，由业务层屏蔽旧勾选。
	Permission *schema.AdminPermission
	// Groups 包含当前业务所有未删除分组，按ID升序，不按用户权限过滤。
	Groups []*schema.PermissionMenu
	// Menus 包含当前业务所有未删除菜单（包括停用项），按sort和ID升序。
	Menus []*schema.Menu
}

// NewAdminPermissionModel 获取用户独立权限的数据会话，取库失败原样返回。
func NewAdminPermissionModel(ctx context.Context, svcCtx *svc.ServiceContext) (*AdminPermissionModel, error) {
	if svcCtx == nil {
		return nil, errors.New("auth-rpc service context is required")
	}
	db, err := svcCtx.DbSelect.GetDb(ctx, bmodel.DB)
	if err != nil {
		return nil, err
	}
	return &AdminPermissionModel{db: db}, nil
}

// GetTreeData 依次执行普通查询，校验目标管理员并读取菜单目录及权限版本，不开启事务。
// 各查询继承请求上下文，不保证跨查询使用同一快照；版本号供后续保存时比较。
// 业务范围严格匹配businessCode；不按管理员状态隐藏编辑页，不授予跨业务或平台绕过。
func (model *AdminPermissionModel) GetTreeData(userID int64, businessCode string) (*UserPermissionTreeData, error) {
	if err := model.checkTargetAdmin(userID, businessCode); err != nil {
		return nil, err
	}
	permission, err := model.findUserPermission(userID, businessCode)
	if err != nil {
		return nil, err
	}
	data := &UserPermissionTreeData{
		Permission: permission,
		Groups:     make([]*schema.PermissionMenu, 0),
		Menus:      make([]*schema.Menu, 0),
	}
	// 普通目录查询统一由模型自动排除软删除记录，无需手写deleted_at条件。
	if err := model.db.Model(&schema.PermissionMenu{}).Where("business_code = ?", businessCode).
		Order("id ASC").Find(&data.Groups).Error; err != nil {
		return nil, fmt.Errorf("query permission groups: %w", err)
	}
	if err := model.db.Model(&schema.Menu{}).Where("business_code = ?", businessCode).
		Order("sort ASC").Order("id ASC").Find(&data.Menus).Error; err != nil {
		return nil, fmt.Errorf("query permission menus: %w", err)
	}
	return data, nil
}

// checkTargetAdmin 为查询和保存统一校验目标管理员及业务归属，模型自动过滤已删除管理员。
func (model *AdminPermissionModel) checkTargetAdmin(userID int64, businessCode string) error {
	if userID <= 0 {
		return status.Error(codes.InvalidArgument, "userId必须为正数")
	}
	if strings.TrimSpace(businessCode) == "" {
		return status.Error(codes.Unauthenticated, "business-code不能为空")
	}
	if model == nil || model.db == nil {
		return errors.New("admin permission model is not initialized")
	}
	// 显式指定管理员模型以启用软删除作用域，仅取ID，避免载入密码等资料。
	var admin struct{ ID int64 }
	err := model.db.Model(&schema.AdminInfo{}).Select("id").
		Where("id = ? AND business_code = ?", userID, businessCode).
		Take(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return status.Error(codes.NotFound, "管理员不存在或不属于当前业务")
	}
	if err != nil {
		return fmt.Errorf("query permission target: %w", err)
	}
	return nil
}

// findUserPermission 包含软删除记录以保留恢复版本；真正没有记录时返回nil，不掩盖数据库错误。
func (model *AdminPermissionModel) findUserPermission(userID int64, businessCode string) (*schema.AdminPermission, error) {
	// 保留软删除记录版本，避免后续恢复时把已有唯一user_id误判成首次创建。
	var permission schema.AdminPermission
	err := model.db.Unscoped().Model(&schema.AdminPermission{}).Where("user_id = ?", userID).Take(&permission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query admin permission: %w", err)
	}
	if permission.BusinessCode != businessCode {
		return nil, status.Error(codes.FailedPrecondition, "用户权限的业务归属不一致")
	}
	if permission.Version <= 0 {
		return nil, status.Error(codes.DataLoss, "用户权限版本异常")
	}
	return &permission, nil
}

// CheckPermission 按管理员ID、业务归属、API服务名及完整路径检查直接授权，不读角色或分组，不开事务。
// 普通模型查询排除软删除数据；无有效用户、权限或菜单均拒绝，数据库和存储格式异常单独返回。
func (model *AdminPermissionModel) CheckPermission(userID int64, serviceName, path, businessCode string) error {
	if userID <= 0 || strings.TrimSpace(path) == "" {
		return status.Error(codes.InvalidArgument, "userId必须为正数且path不能为空")
	}
	if err := ValidateServiceName(serviceName); err != nil {
		return err
	}
	if strings.TrimSpace(businessCode) == "" {
		return status.Error(codes.Unauthenticated, "business-code不能为空")
	}
	if model == nil || model.db == nil {
		return errors.New("admin permission model is not initialized")
	}
	denied := status.Error(codes.PermissionDenied, "没有访问权限")
	// 只读取ID与状态，不载入管理员密码；停用用户不能因已保存菜单权限而继续放行。
	var admin struct {
		ID     int64
		Status int
	}
	err := model.db.Model(&schema.AdminInfo{}).Select("id", "status").
		Where("id = ? AND business_code = ?", userID, businessCode).Take(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return denied
	}
	if err != nil {
		return fmt.Errorf("query permission check user: %w", err)
	}
	if admin.Status != schema.AdminInfoStatus1 {
		return denied
	}

	// 实际鉴权只读未删除权限；与编辑页保留软删除版本的查询分开，不使用Unscoped。
	var permission schema.AdminPermission
	err = model.db.Model(&schema.AdminPermission{}).
		Where("user_id = ? AND business_code = ?", userID, businessCode).Take(&permission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return denied
	}
	if err != nil {
		return fmt.Errorf("query permission check grants: %w", err)
	}
	ids, err := ParseMenuIDs(permission.MenuIds)
	if err != nil {
		return status.Error(codes.DataLoss, "用户菜单ID格式异常")
	}
	if len(ids) == 0 {
		return denied
	}

	// 仅查用户明确拥有的有效页面或按钮，不由页面、父节点或模板推导其他权限。
	var menus []struct {
		ID          int64
		Path        string
		ServiceName string
	}
	err = model.db.Model(&schema.Menu{}).Select("id", "path", "service_name").
		Where("id IN ? AND business_code = ? AND status = ? AND menu_type IN ? AND service_name = ? AND path = ?",
			ids, businessCode, schema.MenuStatus1, []int{schema.MenuMenuType1, schema.MenuMenuType2}, serviceName, path).
		Find(&menus).Error
	if err != nil {
		return fmt.Errorf("query permission check menus: %w", err)
	}
	for _, menu := range menus {
		// 数据库排序规则可能忽略大小写或尾空格；最终以Go字符串精确匹配，拒绝前缀和通配推断。
		if menu.ServiceName == serviceName && menu.Path == path {
			return nil
		}
	}
	return denied
}

// permissionVersionConflict 统一处理旧版本、首次并发创建以及条件更新未匹配，禁止静默覆盖。
func permissionVersionConflict() error {
	return status.Error(codes.Aborted, "用户权限已更新，请刷新后重试")
}

// Save 以menuIDs全量覆盖用户权限，空字符串表示清空；version=0仅用于首次创建。
// 先普通查询校验，再单条INSERT或版本条件UPDATE；显式关闭GORM默认写事务，不追加权限、不绑定角色。
func (model *AdminPermissionModel) Save(userID int64, menuIDs string, version int64, businessCode string) (int64, error) {
	if version < 0 || version == math.MaxInt64 {
		return 0, status.Error(codes.InvalidArgument, "version必须在0到MaxInt64-1范围内")
	}
	normalized, ids, err := NormalizeMenuIDs(menuIDs)
	if errors.Is(err, ErrMenuIDsTooLong) {
		return 0, status.Error(codes.InvalidArgument, "菜单ID列表超过65535字节存储上限")
	}
	if err != nil {
		return 0, status.Error(codes.InvalidArgument, "menuIds必须是英文逗号分隔的正数ID，不能包含空项")
	}
	if err := model.checkTargetAdmin(userID, businessCode); err != nil {
		return 0, err
	}
	current, err := model.findUserPermission(userID, businessCode)
	if err != nil {
		return 0, err
	}
	if current == nil && version != 0 || current != nil && current.Version != version {
		return 0, permissionVersionConflict()
	}
	if err := model.validateMenuSelection(ids, businessCode, current); err != nil {
		return 0, err
	}

	// 独立会话仅关闭本次写入的默认事务，不修改共享连接池配置；版本条件及唯一键负责并发保护。
	writer := model.db.Session(&gorm.Session{SkipDefaultTransaction: true})
	if current == nil {
		permission := &schema.AdminPermission{
			UserId: userID, MenuIds: normalized, BusinessCode: businessCode, Version: 1,
		}
		if err := writer.Create(permission).Error; err != nil {
			var mysqlErr *mysqlDriver.MySQLError
			if errors.Is(err, gorm.ErrDuplicatedKey) || errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				return 0, permissionVersionConflict()
			}
			return 0, fmt.Errorf("create admin permission: %w", err)
		}
		return 1, nil
	}

	// Unscoped仅为显式恢复原记录；NULL恢复软删除状态，created_at和用户归属保持，UpdatedAt由模型维护。
	result := writer.Unscoped().Model(&schema.AdminPermission{}).
		Where("user_id = ? AND business_code = ? AND version = ?", userID, businessCode, version).
		Updates(map[string]interface{}{
			"menu_ids":   normalized,
			"version":    gorm.Expr("version + 1"),
			"deleted_at": nil,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("update admin permission: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return 0, permissionVersionConflict()
	}
	return version + 1, nil
}

// validateMenuSelection 只允许本业务未删除菜单；停用项仅可保留已有有效授权，不得新增授予。
// 空集合表示清空，软删除权限的旧菜单不作为保留停用项的依据。
func (model *AdminPermissionModel) validateMenuSelection(ids []int64, businessCode string, current *schema.AdminPermission) error {
	if len(ids) == 0 {
		return nil
	}
	menus := make([]schema.Menu, 0, len(ids))
	if err := model.db.Model(&schema.Menu{}).Where("id IN ? AND business_code = ?", ids, businessCode).
		Find(&menus).Error; err != nil {
		return fmt.Errorf("query selected permission menus: %w", err)
	}
	if len(menus) != len(ids) {
		return status.Error(codes.InvalidArgument, "菜单不存在、已删除或不属于当前业务")
	}
	previous := make(map[int64]bool)
	loadedPrevious := false
	for _, menu := range menus {
		if menu.Status == schema.MenuStatus1 {
			continue
		}
		if !loadedPrevious {
			if current != nil && current.DeletedAt == 0 {
				oldIDs, err := ParseMenuIDs(current.MenuIds)
				if err != nil {
					return status.Error(codes.DataLoss, "已保存的用户菜单ID格式异常")
				}
				for _, id := range oldIDs {
					previous[id] = true
				}
			}
			loadedPrevious = true
		}
		if !previous[menu.ID] {
			return status.Error(codes.InvalidArgument, "不能新增授予停用菜单")
		}
	}
	return nil
}

// MaxMenuIDsBytes 为menu_ids的TEXT存储字节上限；规范化结果仅含ASCII数字和英文逗号。
const MaxMenuIDsBytes = 65535

var (
	// ErrInvalidMenuIDs 表示逗号分隔列表含空项、非十进制正数或int64越界，不包含原始输入。
	ErrInvalidMenuIDs = errors.New("invalid menu ID list")
	// ErrMenuIDsTooLong 表示规范化后的列表超过TEXT字节上限。
	ErrMenuIDsTooLong = errors.New("menu ID list exceeds TEXT capacity")
)

// ParseMenuIDs 解析英文逗号分隔的int64正数并去重，空字符串返回空集合。
// 允许单项前后空白和前导零，不接受空项、正负号或其他分隔符。
func ParseMenuIDs(value string) ([]int64, error) {
	ids := make([]int64, 0)
	if value == "" {
		return ids, nil
	}
	seen := make(map[int64]bool)
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, ErrInvalidMenuIDs
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return nil, ErrInvalidMenuIDs
			}
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, ErrInvalidMenuIDs
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}

// NormalizeMenuIDs 返回升序、去重、无空白的存储字符串和对应ID列表，空字符串表示清空。
// 超过TEXT字节上限时返回错误，不截断或丢弃菜单。
func NormalizeMenuIDs(value string) (string, []int64, error) {
	ids, err := ParseMenuIDs(value)
	if err != nil {
		return "", nil, err
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	parts := make([]string, len(ids))
	size := 0
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
		size += len(parts[i])
		if i > 0 {
			size++
		}
		if size > MaxMenuIDsBytes {
			return "", nil, ErrMenuIDsTooLong
		}
	}
	return strings.Join(parts, ","), ids, nil
}
