package roleauthservicelogic

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UpdateRoleLogic 承载当前站点权限模板操作，模型构造错误由业务入口返回。
type UpdateRoleLogic struct {
	logx.Logger
	model   *mysql.RoleModel
	initErr error
}

// NewUpdateRoleLogic 初始化继承请求ctx的模板模型，不改变RPC metadata准入规则。
func NewUpdateRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRoleLogic {
	roleModel, err := mysql.NewRoleModel(ctx, svcCtx)
	return &UpdateRoleLogic{Logger: logx.WithContext(ctx), model: roleModel, initErr: err}
}

// UpdateRole 完整覆盖当前站点自定义模板，menuIds和version必须显式提交，不接受根模板或归属转换。
func (l *UpdateRoleLogic) UpdateRole(in *authRpc.UpdateRoleReq) (*authRpc.UpdateRoleResp, error) {
	if in == nil || in.Data == nil || in.Data.MenuIds == nil || in.Data.Version == nil {
		return nil, status.Error(codes.InvalidArgument, "模板data、menuIds和version必须显式传入")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	version, err := l.model.UpdateSchema(&schema.RolePermission{
		ID: in.Data.Id, RoleName: in.Data.RoleName, MenuIds: in.Data.GetMenuIds(),
		BusinessCode: in.Data.BusinessCode, TemplateType: int(in.Data.TemplateType), Version: in.Data.GetVersion(),
	})
	if err != nil {
		return nil, err
	}
	return &authRpc.UpdateRoleResp{Version: version}, nil
}
