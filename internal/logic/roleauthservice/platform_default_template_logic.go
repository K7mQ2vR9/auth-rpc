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

// PlatformDefaultTemplateLogic 集中由10维护平台默认模板1的创建、查询和版本覆盖，实际校验及事务集中在角色模型。
type PlatformDefaultTemplateLogic struct {
	logx.Logger
	model   *mysql.RoleModel
	initErr error
}

// NewPlatformDefaultTemplateLogic 继承原ctx，仅初始化请求级模型，不自动创建模板。
func NewPlatformDefaultTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformDefaultTemplateLogic {
	model, err := mysql.NewRoleModel(ctx, svcCtx)
	return &PlatformDefaultTemplateLogic{Logger: logx.WithContext(ctx), model: model, initErr: err}
}

// GetPlatformDefaultTemplate 仅10查询类型1的真实权限集合及版本，不自动创建或恢复。
func (l *PlatformDefaultTemplateLogic) GetPlatformDefaultTemplate(in *authRpc.GetPlatformDefaultTemplateReq) (*authRpc.GetPlatformDefaultTemplateResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求不能为空")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	info, err := l.model.GetPlatformDefaultTemplate()
	if err != nil {
		return nil, err
	}
	return &authRpc.GetPlatformDefaultTemplateResp{Data: &authRpc.RoleListItem{
		Id: info.ID, RoleName: info.RoleName, MenuIds: info.MenuIds, BusinessCode: info.BusinessCode,
		TemplateType: int32(info.TemplateType), Version: info.Version, CreatedAt: info.CreatedAt,
	}}, nil
}

// CreatePlatformDefaultTemplate 显式提交初始权限集合，仅10可操作，创建和成功审计同事务。
func (l *PlatformDefaultTemplateLogic) CreatePlatformDefaultTemplate(in *authRpc.CreatePlatformDefaultTemplateReq) (*authRpc.CreatePlatformDefaultTemplateResp, error) {
	if in == nil || in.MenuIds == nil {
		return nil, status.Error(codes.InvalidArgument, "必须显式传入menuIds")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	info, err := l.model.CreatePlatformDefaultTemplate(in.GetMenuIds())
	if err != nil {
		return nil, err
	}
	return &authRpc.CreatePlatformDefaultTemplateResp{Id: info.ID, Version: info.Version}, nil
}

// UpdatePlatformDefaultTemplate 显式集合及已读版本必填，覆盖后返回新版本，不改写任何下级CSV。
func (l *PlatformDefaultTemplateLogic) UpdatePlatformDefaultTemplate(in *authRpc.UpdatePlatformDefaultTemplateReq) (*authRpc.UpdatePlatformDefaultTemplateResp, error) {
	if in == nil || in.MenuIds == nil || in.Version == nil {
		return nil, status.Error(codes.InvalidArgument, "必须显式传入menuIds和version")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	version, err := l.model.UpdatePlatformDefaultTemplate(in.GetMenuIds(), in.GetVersion())
	if err != nil {
		return nil, err
	}
	return &authRpc.UpdatePlatformDefaultTemplateResp{Version: version}, nil
}
