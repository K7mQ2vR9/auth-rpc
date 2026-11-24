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
	"gorm.io/gorm"
	"time"
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
	dbRes := builder.First(&res)
	if err := dbRes.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
		}
	}
	return res
}

func (model *MenuModel) InsertSchema(data *schema.Menu) error {
	data.CreatedAt = time.Now().Unix()
	dbRes := model.getDb().Model(&schema.Menu{}).Create(data)
	if err := dbRes.Error; err != nil {
		return err
	}

	return nil
}

func (model *MenuModel) UpdateByMap(id int64, data *schema.Menu) error {
	upData := map[string]interface{}{}
	if data.DeletedAt != 0 {
		upData["deleted_at"] = data.DeletedAt
	}
	return model.getDb().Model(&schema.Menu{}).Where("id = ?", id).Updates(upData).Error
}

func (model *MenuModel) UpdateAll(id int64, data *schema.Menu) error {
	return model.getDb().Model(&schema.Menu{}).Where("id = ?", id).Updates(data).Error
}

func (model *MenuModel) GetList(in *schema.Menu, menuIds []int64, pageQuery *bmodel.PageQuery) (int64, []*schema.Menu, error) {
	builder := model.getDb().Model(&schema.Menu{})
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
