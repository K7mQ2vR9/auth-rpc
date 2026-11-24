package roleauthservicelogic

import (
	"authRpc/internal/dao/model"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"encoding/json"
	"github.com/jinzhu/copier"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPermissionListLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.PermissionMenuModel
	*mysql.MenuModel
}

// NewGetPermissionListLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewGetPermissionListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPermissionListLogic {
	logic := &GetPermissionListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
	permissionMenuModel, err := mysql.NewPermissionMenuModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.PermissionMenuModel = permissionMenuModel

	menuModel, err := mysql.NewMenuModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.MenuModel = menuModel
	return logic
}

func (l *GetPermissionListLogic) GetPermissionList(in *authRpc.GetPermissionListReq) (*authRpc.GetPermissionListResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	resp := &authRpc.GetPermissionListResp{
		List:     make([]*authRpc.PermissionListItem, 0),
		Page:     in.Page,
		PageSize: in.PageSize,
		Total:    0,
	}

	menuList, err := l.MenuModel.GetAllMenu(&schema.Menu{
		//MenuType: schema.MenuMenuType1,
	})
	if err != nil {
		return nil, err
	}
	menuListMap := make(map[int64]*schema.Menu)
	for _, v := range menuList {
		menuListMap[v.ID] = v
	}

	ind := &schema.PermissionMenu{}
	if in.Data != nil {
		copier.Copy(ind, in.Data)
	}
	total, list, err := l.PermissionMenuModel.GetList(ind, model.NewPageQuery(in.Page, in.PageSize))
	if err != nil {
		return nil, err
	}
	resp.Total = total

	ids := make([]int64, 0)
	for _, v := range list {
		permissionItem := &authRpc.PermissionListItem{}
		permissionItem.Menus = make([]*authRpc.PermissionListMenuItem, 0)
		copier.Copy(permissionItem, v)
		json.Unmarshal([]byte(v.MenuIds), &ids)
		for _, v1 := range ids {
			menuItem := &authRpc.PermissionListMenuItem{}
			if da, ok := menuListMap[v1]; ok {
				menuItem.MenuName = da.MenuName
				menuItem.Id = da.ID
				permissionItem.Menus = append(permissionItem.Menus, menuItem)
			}
		}
		resp.List = append(resp.List, permissionItem)
	}

	return resp, nil
}
