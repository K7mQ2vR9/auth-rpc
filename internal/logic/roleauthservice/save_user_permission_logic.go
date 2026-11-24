package roleauthservicelogic

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/svc"
	"context"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SaveUserPermissionLogic 将最终勾选结果覆盖到用户独立记录，不读取或绑定角色模板。
type SaveUserPermissionLogic struct {
	logx.Logger
	ctx     context.Context
	model   *mysql.AdminPermissionModel
	initErr error
}

// NewSaveUserPermissionLogic 初始化请求级模型，构造错误由保存入口返回。
func NewSaveUserPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveUserPermissionLogic {
	model, err := mysql.NewAdminPermissionModel(ctx, svcCtx)
	return &SaveUserPermissionLogic{
		Logger: logx.WithContext(ctx), ctx: ctx, model: model, initErr: err,
	}
}

// SaveUserPermission 要求显式提供menuIds及version，避免字段缺失被误认为清空或首次创建。
// 保存成功返回新版本；业务范围取请求上下文，菜单校验和并发保护由模型处理。
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
	version, err := l.model.Save(in.UserId, in.GetMenuIds(), in.GetVersion(), headInfo.GetBusinessCode(l.ctx))
	if err != nil {
		return nil, err
	}
	return &authRpc.SaveUserPermissionResp{Version: version}, nil
}
