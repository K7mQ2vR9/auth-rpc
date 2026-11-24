package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cast"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteMenuLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.MenuModel
	*mysql.PermissionMenuModel
}

// NewDeleteMenuLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewDeleteMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMenuLogic {
	logic := &DeleteMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
	menuModel, err := mysql.NewMenuModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.MenuModel = menuModel

	permissionMenuModel, err := mysql.NewPermissionMenuModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.PermissionMenuModel = permissionMenuModel
	return logic
}

func (l *DeleteMenuLogic) DeleteMenu(in *authRpc.DeleteMenuReq) (*authRpc.DeleteMenuResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	permissionInfo := l.PermissionMenuModel.FindOneByMenuId(cast.ToString(in.Id))
	if permissionInfo.ID != 0 {
		return nil, errors.New(fmt.Sprintf("Permission [ %s ] info have set", permissionInfo.PermissionName))
	}

	err := l.MenuModel.DeleteByID(in.Id)
	if err != nil {
		return nil, err
	}
	return &authRpc.DeleteMenuResp{}, nil
}
