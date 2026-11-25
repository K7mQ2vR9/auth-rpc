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

// GetRoleMenus 直接解析当前站点模板的逗号权限集合并返回来源版本；不从权限组展开、不保存用户权限。
func (l *GetRoleMenusLogic) GetRoleMenus(in *authRpc.GetRoleMenusReq) (*authRpc.GetRoleMenusResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "模板ID必须为正数")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	info, err := l.model.FindByID(in.Id, in.BusinessCode)
	if err != nil {
		return nil, err
	}
	ids, err := mysql.ParseMenuIDs(info.MenuIds)
	if err != nil {
		return nil, status.Error(codes.DataLoss, "模板菜单ID格式异常")
	}
	return &authRpc.GetRoleMenusResp{MenuIds: ids, Version: info.Version}, nil
}
