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

// GetSiteRootTemplateLogic 为平台管理页读取目标站点根快照，不在查询时创建根记录。
type GetSiteRootTemplateLogic struct {
	logx.Logger
	model   *mysql.RoleModel
	initErr error
}

// NewGetSiteRootTemplateLogic 继承请求上下文，初始化失败交由RPC入口返回。
func NewGetSiteRootTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSiteRootTemplateLogic {
	model, err := mysql.NewRoleModel(ctx, svcCtx)
	return &GetSiteRootTemplateLogic{Logger: logx.WithContext(ctx), model: model, initErr: err}
}

// GetSiteRootTemplate 返回实际保存的根模板及版本，不按操作者目录裁剪历史集合，不产生任何写入。
func (l *GetSiteRootTemplateLogic) GetSiteRootTemplate(in *authRpc.GetSiteRootTemplateReq) (*authRpc.GetSiteRootTemplateResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求不能为空")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	info, err := l.model.GetSiteRootTemplate(in.BusinessCode)
	if err != nil {
		return nil, err
	}
	return &authRpc.GetSiteRootTemplateResp{Data: &authRpc.RoleListItem{
		Id: info.ID, RoleName: info.RoleName, BusinessCode: info.BusinessCode, CreatedAt: info.CreatedAt,
		MenuIds: info.MenuIds, TemplateType: int32(info.TemplateType), Version: info.Version,
	}}, nil
}
