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

// UpdateSiteRootTemplateLogic 处理平台按版本覆盖根权限，不通过普通模板CRUD绕过身份或版本。
type UpdateSiteRootTemplateLogic struct {
	logx.Logger
	model   *mysql.RoleModel
	initErr error
}

// NewUpdateSiteRootTemplateLogic 获取继承原ctx的模型，不切换为目标站点上下文。
func NewUpdateSiteRootTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSiteRootTemplateLogic {
	model, err := mysql.NewRoleModel(ctx, svcCtx)
	return &UpdateSiteRootTemplateLogic{Logger: logx.WithContext(ctx), model: model, initErr: err}
}

// UpdateSiteRootTemplate 要求显式最终menuIds及已读version，缺省字段不作为清空或隐式覆盖。
func (l *UpdateSiteRootTemplateLogic) UpdateSiteRootTemplate(in *authRpc.UpdateSiteRootTemplateReq) (*authRpc.UpdateSiteRootTemplateResp, error) {
	if in == nil || in.MenuIds == nil || in.Version == nil {
		return nil, status.Error(codes.InvalidArgument, "必须显式传入menuIds及version，清空权限请传空字符串")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	version, err := l.model.UpdateSiteRootTemplate(in.BusinessCode, in.GetMenuIds(), in.GetVersion())
	if err != nil {
		return nil, err
	}
	return &authRpc.UpdateSiteRootTemplateResp{Version: version}, nil
}
