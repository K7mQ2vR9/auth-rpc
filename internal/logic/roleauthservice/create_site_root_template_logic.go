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

// CreateSiteRootTemplateLogic 处理平台显式创建站点根模板，不兼任更新、恢复或创建自定义模板。
type CreateSiteRootTemplateLogic struct {
	logx.Logger
	model   *mysql.RoleModel
	initErr error
}

// NewCreateSiteRootTemplateLogic 创建继承请求ctx的模板模型，不改变公共RPC metadata准入规则。
func NewCreateSiteRootTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSiteRootTemplateLogic {
	model, err := mysql.NewRoleModel(ctx, svcCtx)
	return &CreateSiteRootTemplateLogic{Logger: logx.WithContext(ctx), model: model, initErr: err}
}

// CreateSiteRootTemplate 要求显式menuIds；仅11操作，服务端固定名称、站点默认类型2及初始版本1，成功仅返回已提交的ID和版本。
func (l *CreateSiteRootTemplateLogic) CreateSiteRootTemplate(in *authRpc.CreateSiteRootTemplateReq) (*authRpc.CreateSiteRootTemplateResp, error) {
	if in == nil || in.MenuIds == nil {
		return nil, status.Error(codes.InvalidArgument, "必须显式传入menuIds，空根权限请传空字符串")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	created, err := l.model.CreateSiteRootTemplate(in.BusinessCode, in.GetMenuIds())
	if err != nil {
		return nil, err
	}
	return &authRpc.CreateSiteRootTemplateResp{Id: created.ID, Version: created.Version}, nil
}
