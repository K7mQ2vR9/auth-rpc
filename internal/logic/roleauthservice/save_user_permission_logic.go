package roleauthservicelogic

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/svc"
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SaveUserPermissionLogic 将最终勾选覆盖到独立授权；10可给平台11赋权，站点目标读取默认上限，不绑定自定义模板。
type SaveUserPermissionLogic struct {
	logx.Logger
	model   *mysql.AdminPermissionModel
	initErr error
}

// NewSaveUserPermissionLogic 初始化请求级模型，构造错误由保存入口返回。
func NewSaveUserPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveUserPermissionLogic {
	model, err := mysql.NewAdminPermissionModel(ctx, svcCtx)
	return &SaveUserPermissionLogic{
		Logger: logx.WithContext(ctx), model: model, initErr: err,
	}
}

// SaveUserPermission 要求显式提供menuIds及version，避免字段缺失被误认为清空或首次创建。
// 平台须指定目标归属；10可配置自身菜单展示或给11赋权，站点限本站其他用户，同事务校验及审计。
func (l *SaveUserPermissionLogic) SaveUserPermission(in *authRpc.SaveUserPermissionReq) (*authRpc.SaveUserPermissionResp, error) {
	if in == nil || in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "userId必须为正数")
	}
	if in.MenuIds == nil {
		return nil, status.Error(codes.InvalidArgument, "必须显式传入menuIds，清空权限请传空字符串")
	}
	if in.Version == nil {
		return nil, status.Error(codes.InvalidArgument, "必须显式传入version")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	version, err := l.model.Save(in.UserId, in.GetMenuIds(), in.GetVersion(), in.BusinessCode)
	if err != nil {
		return nil, err
	}
	return &authRpc.SaveUserPermissionResp{Version: version}, nil
}
