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

// CopyRolePermissionToUserLogic 处理直接按模板覆盖用户授权，不用于本地草稿预览，不保留角色绑定。
type CopyRolePermissionToUserLogic struct {
	logx.Logger
	model   *mysql.AdminPermissionModel
	initErr error
}

// NewCopyRolePermissionToUserLogic 继承请求ctx及取消、截止时间，初始化错误由入口返回。
func NewCopyRolePermissionToUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CopyRolePermissionToUserLogic {
	model, err := mysql.NewAdminPermissionModel(ctx, svcCtx)
	return &CopyRolePermissionToUserLogic{Logger: logx.WithContext(ctx), model: model, initErr: err}
}

// CopyRolePermissionToUser 必须显式提交来源模板版本与用户版本，成功返回用户授权新版本。
// 按复制事务快照过滤来源中失效和上限外项，全部失效明确覆盖为空；不改写来源模板，实际写入与审计同事务。
func (l *CopyRolePermissionToUserLogic) CopyRolePermissionToUser(in *authRpc.CopyRolePermissionToUserReq) (*authRpc.CopyRolePermissionToUserResp, error) {
	if in == nil || in.RoleVersion == nil || in.Version == nil {
		return nil, status.Error(codes.InvalidArgument, "必须显式传入roleVersion及用户version")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	version, err := l.model.CopyRolePermissionToUser(in.UserId, in.RoleId, in.GetRoleVersion(), in.GetVersion(), in.BusinessCode)
	if err != nil {
		return nil, err
	}
	return &authRpc.CopyRolePermissionToUserResp{Version: version}, nil
}
