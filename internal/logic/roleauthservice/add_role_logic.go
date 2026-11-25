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

// AddRoleLogic 承载当前站点权限模板操作，模型构造错误由业务入口返回。
type AddRoleLogic struct {
	logx.Logger
	model   *mysql.RoleModel
	initErr error
}

// NewAddRoleLogic 初始化继承请求ctx的模板模型，不改变RPC metadata准入规则。
func NewAddRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddRoleLogic {
	roleModel, err := mysql.NewRoleModel(ctx, svcCtx)
	return &AddRoleLogic{Logger: logx.WithContext(ctx), model: roleModel, initErr: err}
}

// AddRole 只新增当前站点自定义模板，menuIds须显式提供，空字符串表示空模板，首次版本为1。
func (l *AddRoleLogic) AddRole(in *authRpc.AddRoleReq) (*authRpc.AddRoleResp, error) {
	if in == nil || in.Data == nil || in.Data.MenuIds == nil {
		return nil, status.Error(codes.InvalidArgument, "模板data和menuIds必须显式传入")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	data := &schema.RolePermission{
		ID: in.Data.Id, RoleName: in.Data.RoleName, MenuIds: in.Data.GetMenuIds(),
		BusinessCode: in.Data.BusinessCode, TemplateType: int(in.Data.TemplateType), Version: in.Data.GetVersion(),
	}
	if err := l.model.InsertSchema(data); err != nil {
		return nil, err
	}
	return &authRpc.AddRoleResp{Id: data.ID, Version: data.Version}, nil
}
