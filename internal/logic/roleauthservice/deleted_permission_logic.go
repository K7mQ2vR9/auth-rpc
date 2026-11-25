package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"errors"

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

// DeletedPermission 只删除展示分组，不再检查角色是否引用分组，也不改变模板或用户的具体权限集合。
func (l *DeletedPermissionLogic) DeletedPermission(in *authRpc.DeletedPermissionReq) (*authRpc.DeletedPermissionResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "分组ID必须为正数")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}

	if in.Id == schema.SitePermissionId {
		return nil, errors.New("init data not be delete")
	}

	err := l.PermissionMenuModel.DeleteByID(in.Id)
	if err != nil {
		return nil, err
	}

	return &authRpc.DeletedPermissionResp{}, nil
}
