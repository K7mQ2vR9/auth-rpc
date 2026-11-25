package mysql

import (
	"authRpc/internal/dao/dto"
	bmodel "authRpc/internal/dao/model"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"errors"
	"fmt"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/copier"
	"github.com/zeromicro/go-zero/core/logc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

type MenuModel struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	db     *gorm.DB
}

// NewMenuModel 获取绑定请求ctx的数据库会话，取库或组合模型初始化失败时返回错误。
func NewMenuModel(ctx context.Context, svcCtx *svc.ServiceContext) (*MenuModel, error) {
	db, err := svcCtx.DbSelect.GetDb(ctx, bmodel.DB)
	if err != nil {
		return nil, err
	}
	return &MenuModel{
		ctx: ctx,
		db:  db,
	}, nil
}

func (model *MenuModel) getDb() *gorm.DB {
	return model.db
}

func (model *MenuModel) FindOne(in *schema.Menu) schema.Menu {
	var res schema.Menu
	builder := model.getDb().Model(&schema.Menu{}).Where("business_code = ?", headInfo.PLATFORM_BUSINESS_CODE)
	if in.ID != 0 {
		builder.Where("id = ?", in.ID)
	}
	if in.MenuPid != 0 {
		builder.Where("menu_pid = ?", in.MenuPid)
	}
	if in.Status != 0 {
		builder.Where("status = ?", in.Status)
	}
	if in.Path != "" {
		builder.Where("path = ?", in.Path)
	}
	if in.MenuType != 0 {
		builder.Where("menu_type = ?", in.MenuType)
	}
	if in.ServiceName != "" {
		builder.Where("service_name = ?", in.ServiceName)
	}
	dbRes := builder.First(&res)
	if err := dbRes.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
		}
	}
	return res
}

// ValidateServiceName 校验必填服务名，最多64个UTF-8字符，不接受首尾空白或隐式默认值。
// 保持原始大小写，不自动修剪或改写，菜单与鉴权调用必须传同一个精确值。
func ValidateServiceName(value string) error {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 64 {
		return status.Error(codes.InvalidArgument, "serviceName必填，最多64字符，不能包含首尾空白")
	}
	return nil
}

// ValidateMenuParents 为授权编辑页和登录菜单树检查父级链，拒绝自身引用、负数父ID及集合内祖先循环。
// 调用方先校验节点非空及ID唯一；集合外父级不展开、不额外查询或授予，保留当前节点作为展示根。
func ValidateMenuParents(ctx context.Context, menus map[int64]*schema.Menu) error {
	finished := make(map[int64]bool, len(menus))
	for id := range menus {
		if err := ctx.Err(); err != nil {
			return err
		}
		if finished[id] {
			continue
		}
		path := make([]int64, 0)
		visiting := make(map[int64]bool)
		for current := id; current != 0 && menus[current] != nil && !finished[current]; current = menus[current].MenuPid {
			if err := ctx.Err(); err != nil {
				return err
			}
			if visiting[current] || menus[current].MenuPid < 0 {
				return status.Error(codes.DataLoss, "菜单父级结构异常")
			}
			visiting[current] = true
			path = append(path, current)
		}
		for _, current := range path {
			finished[current] = true
		}
	}
	return ctx.Err()
}

// validateWrite 在乐观写事务内普通读取并校验快照；服务、路径和类型允许修改，保留原ID及既有授权关系。
// 父级检查完整祖先链，不锁祖先，不保证不同菜单同时改父级的串行性；路径唯一索引兜底新增竞争。
func (model *MenuModel) validateWrite(db *gorm.DB, data *schema.Menu, id int64) error {
	if data == nil || id < 0 {
		return status.Error(codes.InvalidArgument, "菜单数据不能为空且ID不能为负数")
	}
	if err := ValidateServiceName(data.ServiceName); err != nil {
		return err
	}
	if data.MenuName == "" || data.MenuName != strings.TrimSpace(data.MenuName) || !utf8.ValidString(data.MenuName) || utf8.RuneCountInString(data.MenuName) > 100 {
		return status.Error(codes.InvalidArgument, "menuName必填，最多100个UTF-8字符，不能有首尾空白")
	}
	if data.MenuType != schema.MenuMenuType1 && data.MenuType != schema.MenuMenuType2 || data.Status != schema.MenuStatus1 && data.Status != schema.MenuStatus2 {
		return status.Error(codes.InvalidArgument, "menuType仅支持1页面或2按钮，status必须为1启用或2禁用")
	}
	if !utf8.ValidString(data.Path) || utf8.RuneCountInString(data.Path) > 255 || data.Path != strings.TrimSpace(data.Path) {
		return status.Error(codes.InvalidArgument, "path最多255个UTF-8字符且不能有首尾空白；空字符串可用于目录")
	}
	if data.BusinessCode != "" && data.BusinessCode != headInfo.PLATFORM_BUSINESS_CODE {
		return status.Error(codes.PermissionDenied, "菜单只能归属平台共享目录")
	}
	data.BusinessCode = headInfo.PLATFORM_BUSINESS_CODE
	if id > 0 {
		var target schema.Menu
		err := db.Model(&schema.Menu{}).Where("id = ? AND business_code = ?", id, data.BusinessCode).
			Take(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Error(codes.NotFound, "菜单不存在或不属于平台共享目录")
		}
		if err != nil {
			return fmt.Errorf("query menu update target: %w", err)
		}
		if target.Version <= 0 {
			return status.Error(codes.DataLoss, "菜单版本异常")
		}
		if target.Version != data.Version {
			return catalogVersionConflict()
		}
	}
	if data.MenuPid < 0 {
		return status.Error(codes.InvalidArgument, "菜单父级ID无效")
	}
	seen := make(map[int64]bool)
	for parentID := data.MenuPid; parentID != 0; {
		if err := model.ctx.Err(); err != nil {
			return err
		}
		if parentID == id || seen[parentID] {
			return status.Error(codes.InvalidArgument, "菜单不能移动到自身或后代下方，父级链不能循环")
		}
		seen[parentID] = true
		var parent schema.Menu
		err := db.Model(&schema.Menu{}).Select("id", "menu_pid", "menu_type").
			Where("id = ? AND business_code = ?", parentID, data.BusinessCode).
			Take(&parent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Error(codes.InvalidArgument, "父级链包含不存在、已删除或非共享目录菜单")
		}
		if err != nil {
			return fmt.Errorf("query menu ancestor: %w", err)
		}
		if parent.MenuPid < 0 || parent.MenuType != schema.MenuMenuType1 && parent.MenuType != schema.MenuMenuType2 {
			return status.Error(codes.DataLoss, "菜单祖先结构或类型异常")
		}
		parentID = parent.MenuPid
	}
	if data.Path == "" {
		return nil
	}
	var candidates []struct {
		Path        string
		ServiceName string
	}
	if err := db.Model(&schema.Menu{}).Select("path", "service_name").
		Where("business_code = ? AND service_name = ? AND path = ? AND id <> ?", data.BusinessCode, data.ServiceName, data.Path, id).
		Find(&candidates).Error; err != nil {
		return fmt.Errorf("query duplicate service menu path: %w", err)
	}
	for _, candidate := range candidates {
		if candidate.ServiceName == data.ServiceName && candidate.Path == data.Path {
			return status.Error(codes.AlreadyExists, "当前服务内已存在该菜单路径")
		}
	}
	return nil
}

// menuWriteError 将数据库唯一冲突转换为稳定的业务错误，不回显驱动原始数据，不重试目录写入。
func menuWriteError(err error) error {
	var duplicate *mysqlDriver.MySQLError
	if errors.Is(err, gorm.ErrDuplicatedKey) || errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return status.Error(codes.AlreadyExists, "当前服务内已存在该菜单路径")
	}
	return err
}

// catalogVersionConflict 统一目录记录的旧版本/竞争写入错误；不自动重读新版本覆盖。
func catalogVersionConflict() error {
	return status.Error(codes.Aborted, "菜单或分组已更新，请刷新后重试")
}

// InsertSchema 仅正常平台账号10可新增共享菜单，归属由服务端固定为平台，服务名仍须显式传入。
func (model *MenuModel) InsertSchema(data *schema.Menu) error {
	if data != nil && (data.ID != 0 || data.Version != 0) {
		return status.Error(codes.InvalidArgument, "新增菜单不能指定ID或已有版本")
	}
	return withPlatformCatalogWrite(model.ctx, model.db, func(tx *gorm.DB) error {
		if err := model.validateWrite(tx, data, 0); err != nil {
			return err
		}
		data.Version = 1
		data.CreatedAt = time.Now().Unix()
		return menuWriteError(tx.Create(data).Error)
	})
}

// DeleteByID 仅账号10按已读版本软删除菜单；条件递增版本与模型Delete在同一事务，任一步失败回滚。
// 分组不阻止删除，历史引用保留；不使用显式行锁，不将版本冲突伪装为成功或重复删除。
func (model *MenuModel) DeleteByID(id, version int64) error {
	if id <= 0 || version <= 0 || version == math.MaxInt64 {
		return status.Error(codes.InvalidArgument, "菜单ID/version须为正数且version小于MaxInt64")
	}
	return withPlatformCatalogWrite(model.ctx, model.db, func(tx *gorm.DB) error {
		var target schema.Menu
		err := tx.Model(&schema.Menu{}).Select("id", "version").Where("id = ? AND business_code = ?", id, headInfo.PLATFORM_BUSINESS_CODE).Take(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Error(codes.NotFound, "菜单不存在或不属于平台共享目录")
		}
		if err != nil {
			return err
		}
		if target.Version <= 0 {
			return status.Error(codes.DataLoss, "菜单版本异常")
		}
		if target.Version != version {
			return catalogVersionConflict()
		}
		updated := tx.Model(&schema.Menu{}).Where("id = ? AND business_code = ? AND version = ?", id, headInfo.PLATFORM_BUSINESS_CODE, version).Update("version", gorm.Expr("version + 1"))
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return catalogVersionConflict()
		}
		deleted := tx.Where("id = ? AND business_code = ? AND version = ?", id, headInfo.PLATFORM_BUSINESS_CODE, version+1).Delete(&schema.Menu{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return catalogVersionConflict()
		}
		return nil
	})
}

// UpdateAll 仅账号10完整覆盖菜单业务字段；允许修改服务、路径及类型，但菜单ID、归属和既有授权保持。
func (model *MenuModel) UpdateAll(id int64, data *schema.Menu) error {
	if id <= 0 || data == nil || data.Version <= 0 || data.Version == math.MaxInt64 {
		return status.Error(codes.InvalidArgument, "菜单ID/version须为正数且version小于MaxInt64")
	}
	err := withPlatformCatalogWrite(model.ctx, model.db, func(tx *gorm.DB) error {
		if err := model.validateWrite(tx, data, id); err != nil {
			return err
		}
		// 包含合法零值，版本原子递增；零行表示旧版本、并发删除等竞争，不做自动重试。
		result := tx.Model(&schema.Menu{}).Where("id = ? AND business_code = ? AND version = ?", id, headInfo.PLATFORM_BUSINESS_CODE, data.Version).
			Updates(map[string]interface{}{"menu_pid": data.MenuPid, "menu_name": data.MenuName, "menu_type": data.MenuType, "path": data.Path,
				"service_name": data.ServiceName, "status": data.Status, "sort": data.Sort, "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return menuWriteError(result.Error)
		}
		if result.RowsAffected != 1 {
			return catalogVersionConflict()
		}
		return nil
	})
	if err == nil {
		data.Version++
	}
	return err
}

// GetList 查询共享目录；page=0默认1、pageSize=0默认20，最多200，防止负数及偏移溢出。
// menuPid独立保留查询存在性：nil不限制，0只查根，正数查直接子级；不从in.MenuPid的零值推断是否筛选。
func (model *MenuModel) GetList(in *schema.Menu, menuPid *int64, menuIds []int64, pageQuery *bmodel.PageQuery) (int64, []*schema.Menu, error) {
	if in == nil || pageQuery == nil {
		return 0, nil, status.Error(codes.InvalidArgument, "菜单查询参数不能为空")
	}
	if menuPid != nil && *menuPid < 0 {
		return 0, nil, status.Error(codes.InvalidArgument, "菜单父级筛选menuPid不能为负数")
	}
	if pageQuery.Page == 0 {
		pageQuery.Page = 1
	}
	if pageQuery.PageSize == 0 {
		pageQuery.PageSize = 20
	}
	if pageQuery.Page < 1 || pageQuery.PageSize < 1 || pageQuery.PageSize > 200 || pageQuery.Page-1 > math.MaxInt64/pageQuery.PageSize {
		return 0, nil, status.Error(codes.InvalidArgument, "页码必须为正数，pageSize范围为1到200")
	}
	builder := model.getDb().Model(&schema.Menu{}).Where("business_code = ?", headInfo.PLATFORM_BUSINESS_CODE)
	if in.ServiceName != "" {
		builder.Where("service_name = ?", in.ServiceName)
	}
	if in.BusinessCode != "" && in.BusinessCode != headInfo.PLATFORM_BUSINESS_CODE {
		return 0, nil, status.Error(codes.InvalidArgument, "菜单目录仅支持平台归属")
	}
	if menuPid != nil {
		builder = builder.Where("menu_pid = ?", *menuPid)
	}
	if in.MenuType != 0 {
		builder.Where("menu_type = ?", in.MenuType)
	}
	if in.Status != 0 {
		builder.Where("status = ?", in.Status)
	}
	if in.Path != "" {
		builder.Where("path like ?", fmt.Sprintf("%s", "%"+in.Path+"%"))
	}
	if in.MenuName != "" {
		builder.Where("menu_name like ?", fmt.Sprintf("%s", "%"+in.MenuName+"%"))
	}
	if len(menuIds) > 0 {
		builder.Where("id in (?)", menuIds)
	}

	total := int64(0)
	err := builder.Count(&total).Error
	if err != nil {
		return 0, nil, err
	}

	rows := make([]*schema.Menu, 0)
	err = builder.Order("id desc").Offset(pageQuery.Offset()).Limit(pageQuery.GetPageSize()).Find(&rows).Error
	return total, rows, err
}

func (model *MenuModel) buildMenuTree(menus []*schema.Menu) []*dto.MenuTree {
	MenuTree := make(map[int64]*dto.MenuTree)
	MenuTreeTemp := make([]*dto.MenuTree, 0)
	var tree []*dto.MenuTree
	if len(menus) == 0 {
		return tree
	}

	for _, v := range menus {
		menuTree := &dto.MenuTree{}
		copier.Copy(menuTree, v)
		MenuTree[v.ID] = menuTree
		MenuTreeTemp = append(MenuTreeTemp, menuTree)
	}

	for _, menu := range MenuTreeTemp {
		if menu.MenuPid == 0 {
			tree = append(tree, menu)
		} else {
			if parent, exists := MenuTree[menu.MenuPid]; exists {
				parent.Children = append(parent.Children, menu)
			}
		}
	}
	return tree
}

func (model *MenuModel) GetAllMenu(in *schema.Menu) ([]*schema.Menu, error) {
	rows := make([]*schema.Menu, 0)
	builder := model.getDb().Model(&schema.Menu{}).Where("business_code = ?", headInfo.PLATFORM_BUSINESS_CODE)
	if in.MenuType != 0 {
		builder.Where("menu_type = ?", in.MenuType)
	}
	err := builder.Where("status = ?", schema.MenuStatus1).Find(&rows).Error
	return rows, err
}

func (model *MenuModel) GetMenuByIds(menuIds []int64) (res []*schema.Menu, err error) {
	err = model.getDb().Model(&schema.Menu{}).Where("id in ?", menuIds).Find(&res).Error
	if err != nil {
		logc.Error(model.ctx, "GetMenuTreeByIdsCache", err)
		return nil, errors.New("query menu error")
	}
	return
}
