package roleauthservicelogic

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/svc"
	"context"
	"encoding/json"
	"errors"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRoleMenusLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.RoleModel
	*mysql.PermissionMenuModel
}

// NewGetRoleMenusLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewGetRoleMenusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoleMenusLogic {
	logic := &GetRoleMenusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
	roleModel, err := mysql.NewRoleModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.RoleModel = roleModel

	permissionMenuModel, err := mysql.NewPermissionMenuModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.PermissionMenuModel = permissionMenuModel
	return logic
}

func (l *GetRoleMenusLogic) GetRoleMenus(in *authRpc.GetRoleMenusReq) (*authRpc.GetRoleMenusResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	resp := &authRpc.GetRoleMenusResp{MenuIds: make([]int64, 0)}
	if in.Id == 0 {
		return resp, nil
	}
	roleInfo := l.RoleModel.FindOneById(in.Id)
	if roleInfo.ID == 0 {
		return nil, errors.New("RolePermission not found")
	}

	permissionIds := make([]int64, 0)
	err := json.Unmarshal([]byte(roleInfo.PermissionIds), &permissionIds)
	if err != nil {
		return nil, errors.New("Unmarshal PermissionIds err," + err.Error())
	}

	if len(permissionIds) == 0 {
		return resp, nil
	}

	menuIds, err := l.PermissionMenuModel.GetMenuIds(permissionIds)
	resp.MenuIds = append(resp.MenuIds, menuIds...)

	return resp, nil
}
