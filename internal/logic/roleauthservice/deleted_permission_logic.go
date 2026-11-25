package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"context"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeletedPermissionLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.PermissionMenuModel
}

// NewDeletedPermissionLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewDeletedPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeletedPermissionLogic {
	logic := &DeletedPermissionLogic{
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

	return logic
}

// DeletedPermission 仅账号10按已读version软删除平台展示分组，所有ID遵循同一规则，不删除菜单或改变授权。
func (l *DeletedPermissionLogic) DeletedPermission(in *authRpc.DeletedPermissionReq) (*authRpc.DeletedPermissionResp, error) {
	if in == nil || in.Id <= 0 || in.Version == nil {
		return nil, status.Error(codes.InvalidArgument, "分组ID须为正数且必须显式提交version")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}

	err := l.PermissionMenuModel.DeleteByID(in.Id, in.GetVersion())
	if err != nil {
		return nil, err
	}

	return &authRpc.DeletedPermissionResp{Version: in.GetVersion() + 1}, nil
}
