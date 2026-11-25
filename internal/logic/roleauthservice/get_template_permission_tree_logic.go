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

// GetTemplatePermissionTreeLogic 提供独立模板配置页，不依赖目标用户；数据读取和资格校验集中在角色模型。
type GetTemplatePermissionTreeLogic struct {
	logx.Logger
	ctx     context.Context
	model   *mysql.RoleModel
	initErr error
}

// NewGetTemplatePermissionTreeLogic 初始化请求级模型，保留原ctx，构造失败由RPC方法返回。
func NewGetTemplatePermissionTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTemplatePermissionTreeLogic {
	model, err := mysql.NewRoleModel(ctx, svcCtx)
	return &GetTemplatePermissionTreeLogic{Logger: logx.WithContext(ctx), ctx: ctx, model: model, initErr: err}
}

// GetTemplatePermissionTree 按模板用途返回上限内有效权限及勾选，超出上限的权限项直接过滤。
// 首次预览ID/版本为0，不创建模板或授权；必要祖先仅作树结构，不恢复其历史勾选。
func (l *GetTemplatePermissionTreeLogic) GetTemplatePermissionTree(in *authRpc.GetTemplatePermissionTreeReq) (*authRpc.GetTemplatePermissionTreeResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求不能为空")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	data, err := l.model.GetTemplateTreeData(int(in.TemplateType), in.BusinessCode, in.TemplateId)
	if err != nil {
		return nil, err
	}
	checked := make(map[int64]struct{})
	if data.Template != nil {
		checked, err = parseSelectedMenuIDs(data.Template.MenuIds)
		if err != nil {
			return nil, err
		}
	}
	groups, err := buildPermissionGroups(l.ctx, data.Groups, data.Menus, checked, data.Selectable)
	if err != nil {
		return nil, err
	}
	resp := &authRpc.GetTemplatePermissionTreeResp{TemplateType: int32(data.TemplateType), BusinessCode: data.BusinessCode, Groups: groups}
	if data.Template != nil {
		resp.TemplateId = data.Template.ID
		resp.Version = data.Template.Version
		resp.TemplateName = data.Template.RoleName
	}
	return resp, nil
}
