package mysql

import (
	"authRpc/internal/dao/dto"
	bmodel "authRpc/internal/dao/model"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"errors"
	"fmt"
	"github.com/jinzhu/copier"
	"github.com/zeromicro/go-zero/core/logc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
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
	builder := model.getDb().Model(&schema.Menu{})
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

// validateWrite 校验所属服务、目标记录和父级，并按同一业务、服务及完整路径检查重复。
// 保留普通查询方式；重复检查不替代数据库并发唯一约束，空路径目录不参与重复判断。
func (model *MenuModel) validateWrite(data *schema.Menu, id int64) error {
	if data == nil || id < 0 {
		return status.Error(codes.InvalidArgument, "菜单数据不能为空且ID不能为负数")
	}
	if err := ValidateServiceName(data.ServiceName); err != nil {
		return err
	}
	if strings.TrimSpace(data.BusinessCode) == "" {
		return status.Error(codes.Unauthenticated, "business-code不能为空")
	}
	if id > 0 {
		var target struct{ ID int64 }
		err := model.getDb().Model(&schema.Menu{}).Select("id").
			Where("id = ? AND business_code = ?", id, data.BusinessCode).Take(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Error(codes.NotFound, "菜单不存在或不属于当前业务")
		}
		if err != nil {
			return fmt.Errorf("query menu update target: %w", err)
		}
	}
	if data.MenuPid < 0 || id > 0 && data.MenuPid == id {
		return status.Error(codes.InvalidArgument, "菜单父级ID无效")
	}
	if data.MenuPid > 0 {
		var parent struct{ ID int64 }
		err := model.getDb().Model(&schema.Menu{}).Select("id").
			Where("id = ? AND business_code = ?", data.MenuPid, data.BusinessCode).Take(&parent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Error(codes.InvalidArgument, "父菜单不存在或不属于当前业务")
		}
		if err != nil {
			return fmt.Errorf("query menu parent: %w", err)
		}
	}
	if data.Path == "" {
		return nil
	}
	var candidates []struct {
		Path        string
		ServiceName string
	}
	err := model.getDb().Model(&schema.Menu{}).Select("path", "service_name").
		Where("business_code = ? AND service_name = ? AND path = ? AND id <> ?", data.BusinessCode, data.ServiceName, data.Path, id).
		Find(&candidates).Error
	if err != nil {
		return fmt.Errorf("query duplicate service menu path: %w", err)
	}
	for _, candidate := range candidates {
		if candidate.ServiceName == data.ServiceName && candidate.Path == data.Path {
			return status.Error(codes.AlreadyExists, "当前服务内已存在该菜单路径")
		}
	}
	return nil
}

// InsertSchema 新增菜单必须显式传入服务名，按业务与服务检查路径后写入，不默认归入其他服务。
func (model *MenuModel) InsertSchema(data *schema.Menu) error {
	if data != nil && data.ID != 0 {
		return status.Error(codes.InvalidArgument, "新增菜单不能指定ID")
	}
	if err := model.validateWrite(data, 0); err != nil {
		return err
	}
	data.CreatedAt = time.Now().Unix()
	dbRes := model.getDb().Model(&schema.Menu{}).Create(data)
	if err := dbRes.Error; err != nil {
		return err
	}

	return nil
}

// DeleteByID 通过模型软删除指定菜单，自动写入Unix秒并同步UpdatedAt，不物理删除记录。
func (model *MenuModel) DeleteByID(id int64) error {
	if id <= 0 {
		return status.Error(codes.InvalidArgument, "菜单ID必须为正数")
	}
	return model.getDb().Where("id = ?", id).Delete(&schema.Menu{}).Error
}

// UpdateAll 要求每次修改传入服务名，按同一业务及服务检查路径，软删除仍由DeleteByID处理。
func (model *MenuModel) UpdateAll(id int64, data *schema.Menu) error {
	if id <= 0 {
		return status.Error(codes.InvalidArgument, "修改菜单ID必须为正数")
	}
	if err := model.validateWrite(data, id); err != nil {
		return err
	}
	return model.getDb().Model(&schema.Menu{}).Where("id = ? AND business_code = ?", id, data.BusinessCode).
		Omit("id", "created_at", "deleted_at", "business_code").Updates(data).Error
}

func (model *MenuModel) GetList(in *schema.Menu, menuIds []int64, pageQuery *bmodel.PageQuery) (int64, []*schema.Menu, error) {
	builder := model.getDb().Model(&schema.Menu{})
	if in.ServiceName != "" {
		builder.Where("service_name = ?", in.ServiceName)
	}
	if in.BusinessCode != "" {
		builder.Where("business_code = ?", in.BusinessCode)
	}
	if in.MenuPid != 0 {
		builder.Where("menu_pid = ?", in.MenuPid)
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
	builder := model.getDb().Model(&schema.Menu{})
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
