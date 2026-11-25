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

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// AdminPermissionModel 持有请求上下文及数据库会话；站点运行时读取根上限，不继承自定义模板。
// 自定义模板仅显式复制时作为快照来源；所有读取不显式锁行，用户目标版本保护覆盖和审计，不锁来源到提交。
type AdminPermissionModel struct {
	ctx context.Context
	db  *gorm.DB
}

// UserPermissionTreeData 汇总依次普通查询取得的用户权限和完整目录，不承诺跨查询快照一致。
type UserPermissionTreeData struct {
	// Permission 为nil表示从未保存；包含软删除记录时仍保留版本，由业务层屏蔽旧勾选。
	Permission *schema.AdminPermission
	// Groups 包含平台共享目录所有未删除分组，按ID升序，不按目标用户已有授权过滤。
	Groups []*schema.PermissionMenu
	// Menus 只包含共享目录中自身及完整祖先链有效的菜单，删除、禁用及受其影响的下级不展示。
	Menus []*schema.Menu
	// Selectable 给11赋权取平台默认上限，站点目标取两级默认交集；未命中不作为权限项返回，nil不代表全部。
	Selectable map[int64]bool
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
	return &AdminPermissionModel{ctx: ctx, db: db}, nil
}

// CopyRolePermissionToUser 校验读取到的来源版本，并按用户旧版本条件完整覆盖，不追加、不绑定、不重试。
// 只复制当前有效且在两级上限内的权限，全部失效明确清空；不改写来源模板或增加其版本。
// 上限和来源在事务快照中校验，来源并发变更可能与复制重叠，不保证跨记录串行；审计与目标写入同事务。
func (model *AdminPermissionModel) CopyRolePermissionToUser(userID, roleID, roleVersion, version int64, requestedBusinessCode string) (int64, error) {
	if roleID <= 0 || roleVersion <= 0 {
		return 0, status.Error(codes.InvalidArgument, "来源模板ID及roleVersion须为正数")
	}
	return model.overwriteUserPermission(userPermissionWrite{
		userID: userID, version: version, businessCode: requestedBusinessCode, roleID: roleID, roleVersion: roleVersion,
	})
}

// userPermissionWrite 仅供模型内部统一手工保存和模板复制，不由RPC直接构造，不形成角色绑定。
type userPermissionWrite struct {
	userID       int64  // 目标用户ID；除平台10配置自己的菜单展示外，禁止修改自己。
	version      int64  // 用户已读授权版本，0仅表示从未保存，已有或软删除记录使用原版本。
	businessCode string // 平台必填目标站点，站点可省略使用ctx归属，不修改ctx。
	menuIDs      string // 手工保存的规范化完整CSV；复制时使用来源模板快照，忽略此值。
	roleID       int64  // 0仅供手工保存；复制入口须传正数自定义模板ID。
	roleVersion  int64  // 来源预览版本，复制须为正数；手工保存为0，不写入复制来源审计。
}

// permissionTarget 解析操作者与目标站点边界，手工保存、模板复制和编辑页使用同一规则，不覆盖ctx。
// 平台须显式指定目标归属；允许10给11赋权或配置自己菜单，站点只能管理本站其他用户。
func (model *AdminPermissionModel) permissionTarget(userID int64, requested string) (string, string, int64, error) {
	if model == nil || model.db == nil || model.ctx == nil {
		return "", "", 0, errors.New("admin permission model is not initialized")
	}
	if userID <= 0 {
		return "", "", 0, status.Error(codes.InvalidArgument, "userId须为正数")
	}
	roles := &RoleModel{ctx: model.ctx, db: model.db}
	current, err := roles.businessCode("")
	if err != nil {
		return "", "", 0, err
	}
	target := requested
	if current != headInfo.PLATFORM_BUSINESS_CODE {
		if target == "" {
			target = current
		} else if target != current {
			return "", "", 0, status.Error(codes.PermissionDenied, "站点操作者只能管理本站用户权限")
		}
	}
	if target != headInfo.PLATFORM_BUSINESS_CODE {
		if err := validateTargetSiteCode(target); err != nil {
			return "", "", 0, err
		}
	}
	operatorID := headInfo.GetTokenUid(model.ctx)
	if operatorID <= 0 || headInfo.GetTrace(model.ctx) == "" {
		return "", "", 0, status.Error(codes.Unauthenticated, "缺少有效操作者或Trace上下文")
	}
	if operatorID == userID && (current != headInfo.PLATFORM_BUSINESS_CODE || target != current) {
		return "", "", 0, status.Error(codes.PermissionDenied, "仅平台超级管理员10可配置自己的菜单展示")
	}
	return current, target, operatorID, nil
}

// permissionAccounts 按ID顺序普通读取必要身份及归属，不锁账号；校验基于本次读取时的账号状态。
// 平台目标允许10给11赋权或仅配置自身菜单；不允许维护其他10。站点目标为20/30/40，允许目标停用。
func (model *AdminPermissionModel) permissionAccounts(db *gorm.DB, operatorID, userID int64, current, target string) (*schema.AdminInfo, error) {
	accountIDs := []int64{operatorID, userID}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })
	var accounts []schema.AdminInfo
	query := db.Model(&schema.AdminInfo{}).Select("id", "status", "account_type", "business_code").
		Where("id IN ?", accountIDs).Order("id ASC")
	if err := query.Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("query permission management accounts: %w", err)
	}
	var operator, targetUser *schema.AdminInfo
	for i := range accounts {
		if accounts[i].ID == operatorID {
			operator = &accounts[i]
		}
		if accounts[i].ID == userID {
			targetUser = &accounts[i]
		}
	}
	if err := validatePermissionOperator(operator, current); err != nil {
		return nil, err
	}
	if operator.AccountType == schema.AdminInfoAccountType20 {
		if err := requireUniqueSiteSuper(db, current, operator.ID); err != nil {
			return nil, err
		}
	}
	if target == headInfo.PLATFORM_BUSINESS_CODE {
		if current != headInfo.PLATFORM_BUSINESS_CODE || operator.AccountType != schema.AdminInfoAccountType10 ||
			targetUser == nil || targetUser.BusinessCode != target ||
			(targetUser.AccountType != schema.AdminInfoAccountType11 &&
				!(targetUser.ID == operator.ID && targetUser.AccountType == schema.AdminInfoAccountType10)) {
			return nil, status.Error(codes.PermissionDenied, "仅平台10可给平台11赋权或配置自己的菜单展示")
		}
		return operator, nil
	}
	if targetUser == nil || targetUser.BusinessCode != target ||
		targetUser.AccountType != schema.AdminInfoAccountType20 && targetUser.AccountType != schema.AdminInfoAccountType30 &&
			targetUser.AccountType != schema.AdminInfoAccountType40 {
		return nil, status.Error(codes.PermissionDenied, "目标用户不存在、跨站点或不属于可授权的非平台账号身份")
	}
	if targetUser.AccountType == schema.AdminInfoAccountType20 {
		if err := requireUniqueSiteSuper(db, target, targetUser.ID); err != nil {
			return nil, err
		}
	}
	return operator, nil
}

// overwriteUserPermission 同事务读取身份/上限/来源/目录快照，再按用户旧版本覆盖并审计；无显式行锁。
// 仅由校验过参数的手工保存/模板复制入口调用；角色来源只影响当次快照与审计，不参与后续鉴权。
func (model *AdminPermissionModel) overwriteUserPermission(write userPermissionWrite) (int64, error) {
	userID, version := write.userID, write.version
	if userID <= 0 || version < 0 || version == math.MaxInt64 {
		return 0, status.Error(codes.InvalidArgument, "userId须为正数，用户version范围为0到MaxInt64-1")
	}
	current, target, operatorID, err := model.permissionTarget(userID, write.businessCode)
	if err != nil {
		return 0, err
	}
	roles := &RoleModel{ctx: model.ctx, db: model.db}
	traceID := headInfo.GetTrace(model.ctx)
	err = model.db.Transaction(func(tx *gorm.DB) error {
		operator, err := model.permissionAccounts(tx, operatorID, userID, current, target)
		if err != nil {
			return err
		}
		// 自身菜单仅为10的展示偏好，不受模板1限制，也不修改其默认操作权限。
		selfMenus := current == headInfo.PLATFORM_BUSINESS_CODE && target == current &&
			operator.ID == userID && operator.AccountType == schema.AdminInfoAccountType10
		var allowed map[int64]bool
		if !selfMenus {
			if target == headInfo.PLATFORM_BUSINESS_CODE {
				allowed, err = roles.platformCeiling(tx)
			} else {
				allowed, err = roles.siteCeiling(tx, target)
			}
		}
		if err != nil {
			return err
		}
		menuIDs, operation := write.menuIDs, schema.PermissionAuditSaveUserPermission
		if write.roleID > 0 {
			// 按模板复制同样属于使用本站自定义模板，仅20可操作；30/40及平台账号不能绕过模板管理资格。
			if operator.AccountType != schema.AdminInfoAccountType20 {
				return status.Error(codes.PermissionDenied, "仅本站超级管理员20可按普通模板覆盖用户权限")
			}
			source, err := roles.findByID(tx, write.roleID, target)
			if err != nil {
				return err
			}
			if source.TemplateType != schema.RoleTemplateTypeCustom {
				return status.Error(codes.PermissionDenied, "只能复制目标站点自定义模板，不能复制根模板")
			}
			if source.Version != write.roleVersion {
				return roleVersionConflict()
			}
			menuIDs, operation = source.MenuIds, schema.PermissionAuditCopyRoleToUser
		}
		normalized, ids, err := NormalizeMenuIDs(menuIDs)
		if err != nil {
			return status.Error(codes.DataLoss, "已校验的授权菜单ID格式异常")
		}
		if write.roleID > 0 {
			// 模板复制只取当前有效目录与两级上限交集；全部失效明确清空，不改写来源模板或其版本。
			normalized, ids, err = roles.filterTemplateMenuIDs(tx, ids, allowed)
		} else {
			// 先移除手工提交中已失效的菜单，再对剩余当前有效集合检查授权上限，不裁剪有效越权项。
			normalized, ids, err = model.filterSavedMenuIDs(tx, ids)
		}
		if err != nil {
			return err
		}
		if !selfMenus {
			if err := validateMenuCeiling(ids, allowed); err != nil {
				return err
			}
		}
		// 普通读取原快照；后续ID/归属/version条件UPDATE防覆盖，唯一键保护首次创建，恢复保留原ID。
		var previous schema.AdminPermission
		err = tx.Unscoped().Model(&schema.AdminPermission{}).Where("user_id = ?", userID).Take(&previous).Error
		missing := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !missing {
			return fmt.Errorf("query permission write target: %w", err)
		}
		if !missing {
			if previous.BusinessCode != target || previous.ID <= 0 || previous.Version <= 0 {
				return status.Error(codes.DataLoss, "目标用户授权归属或版本异常")
			}
			before, _, err := NormalizeMenuIDs(previous.MenuIds)
			if err != nil {
				return status.Error(codes.DataLoss, "目标用户原授权菜单格式异常")
			}
			previous.MenuIds = before
		}
		if missing && version != 0 || !missing && previous.Version != version {
			return permissionVersionConflict()
		}
		if missing {
			grant := &schema.AdminPermission{UserId: userID, BusinessCode: target, MenuIds: normalized, Version: 1}
			if err := tx.Create(grant).Error; err != nil {
				var duplicate *mysqlDriver.MySQLError
				if errors.Is(err, gorm.ErrDuplicatedKey) || errors.As(err, &duplicate) && duplicate.Number == 1062 {
					return permissionVersionConflict()
				}
				return fmt.Errorf("create user permission: %w", err)
			}
		} else {
			result := tx.Unscoped().Model(&schema.AdminPermission{}).
				Where("id = ? AND user_id = ? AND business_code = ? AND version = ?", previous.ID, userID, target, version).
				Updates(map[string]interface{}{"menu_ids": normalized, "version": gorm.Expr("version + 1"), "deleted_at": nil})
			if result.Error != nil {
				return fmt.Errorf("overwrite user permission: %w", result.Error)
			}
			if result.RowsAffected != 1 {
				return permissionVersionConflict()
			}
		}
		audit := &schema.PermissionAudit{
			Operation:  operation,
			OperatorID: operator.ID, OperatorAccountType: operator.AccountType, OperatorBusinessCode: operator.BusinessCode,
			BusinessCode: target, TargetType: schema.PermissionAuditTargetUserPermission, TargetID: userID,
			BeforeMenuIDs: previous.MenuIds, AfterMenuIDs: normalized, BeforeVersion: version, AfterVersion: version + 1,
			SourceTemplateID: write.roleID, SourceTemplateVersion: write.roleVersion, TraceID: traceID,
		}
		if err := tx.Create(audit).Error; err != nil {
			return fmt.Errorf("audit user permission overwrite: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return version + 1, nil
}

// GetTreeData 普通读取编辑页；10配置自身菜单可选全部有效目录，给11赋权受模板1限制，站点受两级上限。
// 隐藏删除、禁用或祖先不可用菜单，不与操作者本人授权取交集，查询不写库。
func (model *AdminPermissionModel) GetTreeData(userID int64, requestedBusinessCode string) (*UserPermissionTreeData, error) {
	current, target, operatorID, err := model.permissionTarget(userID, requestedBusinessCode)
	if err != nil {
		return nil, err
	}
	operator, err := model.permissionAccounts(model.db, operatorID, userID, current, target)
	if err != nil {
		return nil, err
	}
	selfMenus := current == headInfo.PLATFORM_BUSINESS_CODE && target == current &&
		operator.ID == userID && operator.AccountType == schema.AdminInfoAccountType10
	roles := &RoleModel{ctx: model.ctx, db: model.db}
	allowed := make(map[int64]bool)
	if !selfMenus {
		if target == headInfo.PLATFORM_BUSINESS_CODE {
			allowed, err = roles.platformCeiling(model.db)
		} else {
			allowed, err = roles.siteCeiling(model.db, target)
		}
	}
	if err != nil {
		return nil, err
	}
	permission, err := model.findUserPermission(userID, target)
	if err != nil {
		return nil, err
	}
	data := &UserPermissionTreeData{Permission: permission, Groups: make([]*schema.PermissionMenu, 0), Menus: make([]*schema.Menu, 0), Selectable: allowed}
	if err := model.db.Model(&schema.PermissionMenu{}).Where("business_code = ?", headInfo.PLATFORM_BUSINESS_CODE).
		Order("id ASC").Find(&data.Groups).Error; err != nil {
		return nil, fmt.Errorf("query shared permission groups: %w", err)
	}
	data.Menus, err = model.ListAvailableMenus()
	if err != nil {
		return nil, err
	}
	if selfMenus {
		for _, menu := range data.Menus {
			data.Selectable[menu.ID] = true
		}
	}

	return data, nil
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
	if permission.ID <= 0 || permission.Version <= 0 {
		return nil, status.Error(codes.DataLoss, "用户权限版本异常")
	}
	normalized, _, err := NormalizeMenuIDs(permission.MenuIds)
	if err != nil {
		return nil, status.Error(codes.DataLoss, "用户菜单ID格式异常")
	}
	permission.MenuIds = normalized
	return &permission, nil
}

// permissionSubject 按真实账号和ctx归属读取运行时主体，20须是本站唯一超级管理员，不读取密码或模板绑定。
func (model *AdminPermissionModel) permissionSubject(userID int64, businessCode string) (*schema.AdminInfo, string, error) {
	if userID <= 0 {
		return nil, "", status.Error(codes.InvalidArgument, "userId必须为正数")
	}
	if model == nil || model.db == nil || model.ctx == nil {
		return nil, "", errors.New("admin permission model is not initialized")
	}
	roles := &RoleModel{ctx: model.ctx, db: model.db}
	current, err := roles.businessCode(businessCode)
	if err != nil {
		return nil, "", err
	}
	denied := status.Error(codes.PermissionDenied, "没有访问权限")
	// 只读取身份必要字段，按数据库明确类型分支，不信任请求体账号类型或固定超级用户ID。
	var admin schema.AdminInfo
	err = model.db.Model(&schema.AdminInfo{}).Select("id", "status", "account_type", "business_code").
		Where("id = ? AND business_code = ?", userID, current).Take(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", denied
	}
	if err != nil {
		return nil, "", fmt.Errorf("query effective permission user: %w", err)
	}
	if err := validatePermissionOperator(&admin, current); err != nil {
		return nil, "", denied
	}
	if current != headInfo.PLATFORM_BUSINESS_CODE && admin.AccountType == schema.AdminInfoAccountType20 {
		if err := requireUniqueSiteSuper(model.db, current, userID); err != nil {
			return nil, "", err
		}
	}
	return &admin, current, nil
}

// effectiveUserMenus 仅接收已校验主体；菜单树统一按配置，接口鉴权仅20默认使用本站上限，10由CheckPermission身份分支处理。
// 默认模板、目录及祖先校验共用，普通读取无事务/行锁/缓存，不修改任何CSV。
func (model *AdminPermissionModel) effectiveUserMenus(admin *schema.AdminInfo, current, serviceName, path string, configuredOnly bool) ([]*schema.Menu, error) {
	userID := admin.ID
	roles := &RoleModel{ctx: model.ctx, db: model.db}
	denied := status.Error(codes.PermissionDenied, "没有访问权限")
	platformSuper := current == headInfo.PLATFORM_BUSINESS_CODE && admin.AccountType == schema.AdminInfoAccountType10
	siteSuper := current != headInfo.PLATFORM_BUSINESS_CODE && admin.AccountType == schema.AdminInfoAccountType20

	useSiteDefault := siteSuper && !configuredOnly
	var err error
	var rootIDs map[int64]bool
	if !platformSuper {
		if current != headInfo.PLATFORM_BUSINESS_CODE {
			rootIDs, err = roles.siteCeiling(model.db, current)
		} else {
			rootIDs, err = roles.platformCeiling(model.db)
		}
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition || status.Code(err) == codes.NotFound {
				return nil, denied
			}
			return nil, err
		}
	}
	ids := make([]int64, 0)
	if useSiteDefault {
		for id := range rootIDs {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		if len(ids) == 0 {
			return make([]*schema.Menu, 0), nil
		}
	}
	if !useSiteDefault {
		// 实际访问不读取软删除授权、不恢复旧记录；缺失授权返回空集合，损坏存储独立返回错误。
		var grant schema.AdminPermission
		err := model.db.Model(&schema.AdminPermission{}).Where("user_id = ?", userID).Take(&grant).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return make([]*schema.Menu, 0), nil
		}
		if err != nil {
			return nil, fmt.Errorf("query effective user grants: %w", err)
		}
		if grant.BusinessCode != current || grant.ID <= 0 || grant.Version <= 0 {
			return nil, status.Error(codes.DataLoss, "用户授权归属或版本异常")
		}
		_, grantedIDs, err := NormalizeMenuIDs(grant.MenuIds)
		if err != nil {
			return nil, status.Error(codes.DataLoss, "用户菜单ID格式异常")
		}
		for _, id := range grantedIDs {
			if rootIDs == nil || rootIDs[id] {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return make([]*schema.Menu, 0), nil
		}
	}
	// 所有站点共用平台目录，站点归属仅约束账号及授权，不要求复制菜单记录到站点。
	query := model.db.Model(&schema.Menu{}).
		Where("business_code = ? AND status = ? AND menu_type IN ?", headInfo.PLATFORM_BUSINESS_CODE,
			schema.MenuStatus1, []int{schema.MenuMenuType1, schema.MenuMenuType2})
	query = query.Where("id IN ?", ids)
	if serviceName != "" {
		query = query.Where("service_name = ? AND path = ?", serviceName, path)
	}
	candidates := make([]*schema.Menu, 0)
	if err := query.Select("id", "menu_pid", "menu_name", "menu_type", "path", "service_name", "status", "sort", "business_code").
		Order("sort ASC").Order("id ASC").Find(&candidates).Error; err != nil {
		return nil, fmt.Errorf("query effective permission menus: %w", err)
	}
	if serviceName != "" {
		matched := make([]*schema.Menu, 0, len(candidates))
		for _, menu := range candidates {
			if menu != nil && menu.ServiceName == serviceName && menu.Path == path {
				matched = append(matched, menu)
			}
		}
		candidates = matched
	}
	return model.filterEffectiveMenuAncestors(model.db, candidates)
}

// filterEffectiveMenuAncestors 批量普通查询完整父级链，祖先只校验、不自动加入返回菜单或授权。
// 停用、删除、缺失、非共享目录或非页面/按钮的节点使依赖它的候选不可用；循环、负父ID等结构异常返回DataLoss。
// 仅过滤返回结果，不改写任何用户、根或模板CSV；父节点有效但未获授时不阻止已获授子节点使用。
func (model *AdminPermissionModel) filterEffectiveMenuAncestors(db *gorm.DB, candidates []*schema.Menu) ([]*schema.Menu, error) {
	loaded := make(map[int64]*schema.Menu, len(candidates))
	queried := make(map[int64]bool, len(candidates))
	for _, menu := range candidates {
		if menu == nil || menu.ID <= 0 || loaded[menu.ID] != nil {
			return nil, status.Error(codes.DataLoss, "菜单目录数据异常")
		}
		loaded[menu.ID], queried[menu.ID] = menu, true
	}
	frontier := candidates
	for len(frontier) != 0 {
		if err := model.ctx.Err(); err != nil {
			return nil, err
		}
		pending := make([]int64, 0)
		for _, menu := range frontier {
			if menu.MenuPid < 0 {
				return nil, status.Error(codes.DataLoss, "菜单父级结构异常")
			}
			if parent := menu.MenuPid; parent > 0 && !queried[parent] {
				queried[parent] = true
				pending = append(pending, parent)
			}
		}
		if len(pending) == 0 {
			break
		}
		sort.Slice(pending, func(i, j int) bool { return pending[i] < pending[j] })
		frontier = make([]*schema.Menu, 0)
		query := db.Model(&schema.Menu{}).Select("id", "menu_pid", "menu_type", "service_name", "status", "business_code").
			Where("id IN ? AND business_code = ?", pending, headInfo.PLATFORM_BUSINESS_CODE).Order("id ASC")
		if err := query.Find(&frontier).Error; err != nil {
			return nil, fmt.Errorf("query effective menu ancestors: %w", err)
		}
		for _, menu := range frontier {
			if menu == nil || menu.ID <= 0 || loaded[menu.ID] != nil {
				return nil, status.Error(codes.DataLoss, "菜单祖先数据异常")
			}
			loaded[menu.ID] = menu
		}
	}
	if err := ValidateMenuParents(model.ctx, loaded); err != nil {
		return nil, err
	}
	// 迭代并缓存已判定链，避免递归深度和为每个候选重复遍历同一条祖先链。
	resolved, available := make(map[int64]bool, len(loaded)), make(map[int64]bool, len(loaded))
	result := make([]*schema.Menu, 0, len(candidates))
	for _, candidate := range candidates {
		chain := make([]int64, 0)
		valid := true
		for id := candidate.ID; id != 0; {
			if err := model.ctx.Err(); err != nil {
				return nil, err
			}
			if resolved[id] {
				valid = available[id]
				break
			}
			menu := loaded[id]
			if menu == nil {
				valid = false
				break
			}
			chain = append(chain, id)
			if menu.BusinessCode != headInfo.PLATFORM_BUSINESS_CODE || menu.DeletedAt != 0 || menu.Status != schema.MenuStatus1 ||
				menu.MenuType != schema.MenuMenuType1 && menu.MenuType != schema.MenuMenuType2 {
				valid = false
				break
			}
			if err := ValidateServiceName(menu.ServiceName); err != nil {
				return nil, status.Error(codes.DataLoss, "菜单或祖先服务名异常")
			}
			id = menu.MenuPid
		}
		for _, id := range chain {
			resolved[id], available[id] = true, valid
		}
		if valid {
			result = append(result, candidate)
		}
	}
	if err := model.ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// filterSavedMenuIDs 在保存事务中普通读取菜单及祖先，按本次快照过滤失效项，不保证目录在提交前不变。
// 仍有效的跨目录菜单及非法类型拒绝，结构/存储损坏返回错误；剩余有效集合随后整体检查模板上限。
func (model *AdminPermissionModel) filterSavedMenuIDs(tx *gorm.DB, ids []int64) (string, []int64, error) {
	if len(ids) == 0 {
		return "", make([]int64, 0), nil
	}
	var menus []*schema.Menu
	if err := tx.Unscoped().Model(&schema.Menu{}).Where("id IN ?", ids).Order("id ASC").
		Find(&menus).Error; err != nil {
		return "", nil, fmt.Errorf("query saved menu availability: %w", err)
	}
	candidates := make([]*schema.Menu, 0, len(menus))
	for _, menu := range menus {
		if menu == nil || menu.ID <= 0 {
			return "", nil, status.Error(codes.DataLoss, "菜单目录异常")
		}
		if menu.DeletedAt != 0 || menu.Status != schema.MenuStatus1 {
			continue
		}
		if menu.BusinessCode != headInfo.PLATFORM_BUSINESS_CODE {
			return "", nil, status.Error(codes.PermissionDenied, "只能授予共享平台目录的菜单")
		}
		if menu.MenuType != schema.MenuMenuType1 && menu.MenuType != schema.MenuMenuType2 {
			return "", nil, status.Error(codes.InvalidArgument, "只能授予页面或按钮")
		}
		candidates = append(candidates, menu)
	}
	effective, err := model.filterEffectiveMenuAncestors(tx, candidates)
	if err != nil {
		return "", nil, err
	}
	kept := make([]int64, 0, len(effective))
	parts := make([]string, 0, len(effective))
	for _, menu := range effective {
		kept = append(kept, menu.ID)
		parts = append(parts, strconv.FormatInt(menu.ID, 10))
	}
	return strings.Join(parts, ","), kept, nil
}

// ListAvailableMenus 为分组列表与授权编辑页读取有效共享目录，统一过滤删除、禁用及祖先失效菜单。
// 普通读取，无事务或行锁；不自动授予祖先，不改写分组/模板/用户保存的历史ID。
func (model *AdminPermissionModel) ListAvailableMenus() ([]*schema.Menu, error) {
	var menus []*schema.Menu
	if err := model.db.Model(&schema.Menu{}).
		Where("business_code = ? AND status = ? AND menu_type IN ?", headInfo.PLATFORM_BUSINESS_CODE,
			schema.MenuStatus1, []int{schema.MenuMenuType1, schema.MenuMenuType2}).
		Order("sort ASC").Order("id ASC").Find(&menus).Error; err != nil {
		return nil, fmt.Errorf("query shared available menus: %w", err)
	}
	return model.filterEffectiveMenuAncestors(model.db, menus)
}

// GetGrantedMenus 所有账号（包括10）统一按已保存menuIds返回菜单，再检查站点上限及完整祖先链。
// 无配置、已删除授权或空配置返回空列表，不因超级管理员身份自动返回整棵目录，结果按sort、ID升序。
func (model *AdminPermissionModel) GetGrantedMenus(userID int64, businessCode string) ([]*schema.Menu, error) {
	admin, current, err := model.permissionSubject(userID, businessCode)
	if err != nil {
		return nil, err
	}
	return model.effectiveUserMenus(admin, current, "", "", true)
}

// CheckPermission 按真实身份判定操作权限：10默认全操作，20默认本站有效上限，其他账号逐项授权并受默认上限限制。
// 除10身份分支外按服务/路径及完整菜单链精确校验；菜单树始终按配置，普通读取不修改或恢复授权。
func (model *AdminPermissionModel) CheckPermission(userID int64, serviceName, path, businessCode string) error {
	if userID <= 0 || strings.TrimSpace(path) == "" {
		return status.Error(codes.InvalidArgument, "userId必须为正数且path不能为空")
	}
	if err := ValidateServiceName(serviceName); err != nil {
		return err
	}
	admin, current, err := model.permissionSubject(userID, businessCode)
	if err != nil {
		return err
	}
	if current == headInfo.PLATFORM_BUSINESS_CODE && admin.AccountType == schema.AdminInfoAccountType10 {
		// 10自身操作全权限不依赖模板或菜单配置；菜单树仍单独按配置读取，不能由此自动展开。
		return model.ctx.Err()
	}
	menus, err := model.effectiveUserMenus(admin, current, serviceName, path, false)
	if err != nil {
		return err
	}
	if len(menus) == 0 {
		return status.Error(codes.PermissionDenied, "没有访问权限")
	}
	return nil
}

// permissionVersionConflict 统一处理旧版本、首次并发创建以及条件更新未匹配，禁止静默覆盖。
func permissionVersionConflict() error {
	return status.Error(codes.Aborted, "用户权限已更新，请刷新后重试")
}

// Save 覆盖独立配置；10可配置自身菜单展示或给11赋权，其余按站点上限；失效项取消，其余身份禁止自改。
// 平台显式指定目标站点，站点缺省使用ctx归属；版本、身份和权限上限与成功审计均在同一事务完成。
func (model *AdminPermissionModel) Save(userID int64, menuIDs string, version int64, requestedBusinessCode string) (int64, error) {
	normalized, _, err := NormalizeMenuIDs(menuIDs)
	if errors.Is(err, ErrMenuIDsTooLong) {
		return 0, status.Error(codes.InvalidArgument, "菜单ID列表超过65535字节存储上限")
	}
	if err != nil {
		return 0, status.Error(codes.InvalidArgument, "menuIds必须是英文逗号分隔的正数ID，不能包含空项")
	}
	return model.overwriteUserPermission(userPermissionWrite{
		userID: userID, version: version, businessCode: requestedBusinessCode, menuIDs: normalized,
	})
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
