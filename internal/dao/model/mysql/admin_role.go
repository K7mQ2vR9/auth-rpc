package mysql

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"github.com/jinzhu/copier"
	"github.com/zeromicro/go-zero/core/logc"
	"gorm.io/gorm"
)

type AdminRoleModel struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	db     *gorm.DB
	*MenuModel
	*PermissionMenuModel
	*RoleModel
}

// NewAdminRoleModel 获取绑定请求ctx的数据库会话，取库或组合模型初始化失败时返回错误。
func NewAdminRoleModel(ctx context.Context, svcCtx *svc.ServiceContext) (*AdminRoleModel, error) {
	db, err := svcCtx.DbSelect.GetDb(ctx, model.DB)
	if err != nil {
		return nil, err
	}
	menuModel, err := NewMenuModel(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	permissionMenuModel, err := NewPermissionMenuModel(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	roleModel, err := NewRoleModel(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	return &AdminRoleModel{
		ctx:                 ctx,
		svcCtx:              svcCtx,
		db:                  db,
		MenuModel:           menuModel,
		PermissionMenuModel: permissionMenuModel,
		RoleModel:           roleModel,
	}, nil
}

func (model *AdminRoleModel) GetAdminRoleMenu(roleId int64) (res []*authRpc.GetMenuByAdminIdItem, err error) {
	permissionIds, err := model.RoleModel.GetRolePermissionById(roleId)
	if err != nil {
		return nil, err
	}
	if len(permissionIds) == 0 {
		logc.Info(model.ctx, "GetAdminRoleMenu", "role not found permission id")
		return
	}
	menuIds, err := model.PermissionMenuModel.GetMenuIds(permissionIds)
	if err != nil {
		return nil, err
	}
	if len(menuIds) == 0 {
		logc.Info(model.ctx, "GetAdminRoleMenu", "role not found menu id")
		return
	}

	menuInfo, err := model.MenuModel.GetMenuByIds(menuIds)
	if err != nil {
		return nil, err
	}
	if len(menuInfo) == 0 {
		logc.Info(model.ctx, "GetAdminRoleMenu", "role not found menu info list")
		return
	}

	menuTree := model.buildMenuTree(menuInfo)
	return menuTree, nil
}

func (model *AdminRoleModel) buildMenuTree(menus []*schema.Menu) []*authRpc.GetMenuByAdminIdItem {
	MenuTree := make(map[int64]*authRpc.GetMenuByAdminIdItem)
	MenuTreeTemp := make([]*authRpc.GetMenuByAdminIdItem, 0)
	var tree []*authRpc.GetMenuByAdminIdItem
	if len(menus) == 0 {
		return tree
	}

	for _, v := range menus {
		menuTree := &authRpc.GetMenuByAdminIdItem{}
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
