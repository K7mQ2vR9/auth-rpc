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

// DeleteRoleLogic 承载当前站点权限模板操作，模型构造错误由业务入口返回。
type DeleteRoleLogic struct {
	logx.Logger
	model   *mysql.RoleModel
	initErr error
}

// NewDeleteRoleLogic 初始化继承请求ctx的模板模型，不改变RPC metadata准入规则。
func NewDeleteRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteRoleLogic {
	roleModel, err := mysql.NewRoleModel(ctx, svcCtx)
	return &DeleteRoleLogic{Logger: logx.WithContext(ctx), model: roleModel, initErr: err}
}

// DeleteRole 按已读版本软删除当前站点自定义模板；不允许普通删除根模板，不检查用户绑定或修改用户权限。
func (l *DeleteRoleLogic) DeleteRole(in *authRpc.DeleteRoleReq) (*authRpc.DeleteRoleResp, error) {
	if in == nil || in.Id <= 0 || in.Version == nil {
		return nil, status.Error(codes.InvalidArgument, "模板ID必须为正数且version必须显式传入")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	if err := l.model.DeleteByID(in.Id, in.GetVersion()); err != nil {
		return nil, err
	}
	return &authRpc.DeleteRoleResp{}, nil
}
