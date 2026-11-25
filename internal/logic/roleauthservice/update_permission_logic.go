package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"github.com/jinzhu/copier"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdatePermissionLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.PermissionMenuModel
}

// NewUpdatePermissionLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewUpdatePermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePermissionLogic {
	logic := &UpdatePermissionLogic{
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

// UpdatePermission 仅更新展示分组，平台账号10资格、目标归属及旧版本条件写入由模型校验，成功返回新版本。
func (l *UpdatePermissionLogic) UpdatePermission(in *authRpc.UpdatePermissionReq) (*authRpc.UpdatePermissionResp, error) {
	if in == nil || in.Data == nil || in.Data.Id <= 0 || in.Data.Version == nil {
		return nil, status.Error(codes.InvalidArgument, "分组data不能为空且ID须为正数，必须显式提交version")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}

	ind := &schema.PermissionMenu{}
	copier.Copy(ind, in.Data)
	ind.Version = in.Data.GetVersion()
	err := l.PermissionMenuModel.UpdateByMap(ind.ID, ind)
	if err != nil {
		return nil, err
	}
	return &authRpc.UpdatePermissionResp{Version: ind.Version}, nil
}
