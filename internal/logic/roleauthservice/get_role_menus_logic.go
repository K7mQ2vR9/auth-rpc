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

// GetRoleMenusLogic 承载当前站点权限模板操作，模型构造错误由业务入口返回。
type GetRoleMenusLogic struct {
	logx.Logger
	model   *mysql.RoleModel
	initErr error
}

// NewGetRoleMenusLogic 初始化继承请求ctx的模板模型，不改变RPC metadata准入规则。
func NewGetRoleMenusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoleMenusLogic {
	roleModel, err := mysql.NewRoleModel(ctx, svcCtx)
	return &GetRoleMenusLogic{Logger: logx.WithContext(ctx), model: roleModel, initErr: err}
}

// GetRoleMenus 返回本站普通模板中当前有效且在两级上限内的权限及来源版本；全部失效返回空ID列表。
// 与直接复制共用过滤规则，不从权限组展开，不改写模板或用户；默认模板缺失及异常存储仍返回错误。
func (l *GetRoleMenusLogic) GetRoleMenus(in *authRpc.GetRoleMenusReq) (*authRpc.GetRoleMenusResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "模板ID必须为正数")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	ids, version, err := l.model.GetEffectiveMenuIDs(in.Id, in.BusinessCode)
	if err != nil {
		return nil, err
	}
	return &authRpc.GetRoleMenusResp{MenuIds: ids, Version: version}, nil
}
