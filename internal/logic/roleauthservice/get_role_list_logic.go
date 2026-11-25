package roleauthservicelogic

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetRoleListLogic 承载当前站点权限模板操作，模型构造错误由业务入口返回。
type GetRoleListLogic struct {
	logx.Logger
	model   *mysql.RoleModel
	initErr error
}

// NewGetRoleListLogic 初始化继承请求ctx的模板模型，不改变RPC metadata准入规则。
func NewGetRoleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoleListLogic {
	roleModel, err := mysql.NewRoleModel(ctx, svcCtx)
	return &GetRoleListLogic{Logger: logx.WithContext(ctx), model: roleModel, initErr: err}
}

// GetRoleList 列出当前站点模板，支持ID、名称、用途类型和Unix秒创建时间范围筛选，不改写或截取业务编码。
func (l *GetRoleListLogic) GetRoleList(in *authRpc.GetRoleListReq) (*authRpc.GetRoleListResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "模板列表请求不能为空")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	filter := &schema.RolePermission{}
	if in.Data != nil {
		filter.ID, filter.RoleName = in.Data.Id, in.Data.RoleName
		filter.TemplateType, filter.BusinessCode = int(in.Data.TemplateType), in.Data.BusinessCode
	}
	page, size := in.Page, in.PageSize
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	total, rows, err := l.model.GetList(filter, model.NewPageQuery(page, size), in.STime, in.ETime)
	if err != nil {
		return nil, err
	}
	list := make([]*authRpc.RoleListItem, 0, len(rows))
	for _, info := range rows {
		list = append(list, &authRpc.RoleListItem{
			Id: info.ID, RoleName: info.RoleName, MenuIds: info.MenuIds, CreatedAt: info.CreatedAt,
			BusinessCode: info.BusinessCode, TemplateType: int32(info.TemplateType), Version: info.Version,
		})
	}
	return &authRpc.GetRoleListResp{List: list, Page: page, PageSize: size, Total: total}, nil
}
