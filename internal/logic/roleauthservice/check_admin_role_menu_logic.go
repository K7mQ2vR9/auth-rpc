package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"context"
	"errors"

	"authRpc/authRpc"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type CheckAdminRoleMenuLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.PermissionMenuModel
	*mysql.RoleModel
	*mysql.MenuModel
}

// NewCheckAdminRoleMenuLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewCheckAdminRoleMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckAdminRoleMenuLogic {
	logic := &CheckAdminRoleMenuLogic{
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

	roleModel, err := mysql.NewRoleModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.RoleModel = roleModel

	menuModel, err := mysql.NewMenuModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.MenuModel = menuModel
	return logic
}

func (l *CheckAdminRoleMenuLogic) CheckAdminRoleMenu(in *authRpc.CheckAdminRoleMenuReq) (*authRpc.CheckAdminRoleMenuResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	resp := &authRpc.CheckAdminRoleMenuResp{}
	respErr := errors.New("没有访问权限")
	if in.RoleId == 0 || in.Path == "" {
		return nil, respErr
	}
	permissionIds, err := l.RoleModel.GetRolePermissionById(in.RoleId)
	if err != nil {
		return nil, err
	}
	if len(permissionIds) == 0 {
		return nil, respErr
	}
	menuIds, err := l.PermissionMenuModel.GetMenuIds(permissionIds)
	if err != nil {
		return nil, err
	}
	if len(menuIds) == 0 {
		return nil, respErr
	}

	menuInfo := l.MenuModel.FindOne(&schema.Menu{Path: in.Path, MenuType: schema.MenuMenuType2})
	if menuInfo.ID == 0 {
		return nil, respErr
	}

	bl := false
	for _, v := range menuIds {
		if menuInfo.ID == v {
			bl = true
			continue
		}
	}
	if !bl {
		return nil, respErr
	}

	return resp, nil
}
