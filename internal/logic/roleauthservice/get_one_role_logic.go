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

// GetOneRoleLogic 承载当前站点权限模板操作，模型构造错误由业务入口返回。
type GetOneRoleLogic struct {
	logx.Logger
	model   *mysql.RoleModel
	initErr error
}

// NewGetOneRoleLogic 初始化继承请求ctx的模板模型，不改变RPC metadata准入规则。
func NewGetOneRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOneRoleLogic {
	roleModel, err := mysql.NewRoleModel(ctx, svcCtx)
	return &GetOneRoleLogic{Logger: logx.WithContext(ctx), model: roleModel, initErr: err}
}

// GetOneRole 普通查询当前站点模板快照，返回具体menuIds、用途类型和版本，不展开分组或用户绑定。
func (l *GetOneRoleLogic) GetOneRole(in *authRpc.GetOneRoleReq) (*authRpc.GetOneRoleResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "模板ID必须为正数")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	info, err := l.model.FindByID(in.Id, "")
	if err != nil {
		return nil, err
	}
	return &authRpc.GetOneRoleResp{Data: &authRpc.RoleListItem{
		Id: info.ID, RoleName: info.RoleName, MenuIds: info.MenuIds, CreatedAt: info.CreatedAt,
		BusinessCode: info.BusinessCode, TemplateType: int32(info.TemplateType), Version: info.Version,
	}}, nil
}
