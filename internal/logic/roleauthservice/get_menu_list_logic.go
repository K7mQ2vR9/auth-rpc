package roleauthservicelogic

import (
	"authRpc/internal/dao/model"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/jinzhu/copier"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetMenuListLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.MenuModel
}

// NewGetMenuListLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewGetMenuListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMenuListLogic {
	logic := &GetMenuListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
	menuModel, err := mysql.NewMenuModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.MenuModel = menuModel
	return logic
}

// GetMenuList 查询共享目录，维护者可查看禁用菜单；查询归属固定为平台，不复制站点目录。
// menuPid保留字段存在性：未传查所有父级，显式0只查根，正数只查直接子级；负数由模型拒绝。
func (l *GetMenuListLogic) GetMenuList(in *authRpc.GetMenuListReq) (*authRpc.GetMenuListResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求不能为空")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}

	resp := &authRpc.GetMenuListResp{
		List:     make([]*authRpc.MenuItem, 0),
		Page:     in.Page,
		PageSize: in.PageSize,
		Total:    0,
	}

	ind := &schema.Menu{}
	var menuPid *int64
	if in.Data != nil {
		copier.Copy(ind, in.Data)
		menuPid = in.Data.MenuPid
	}
	ind.BusinessCode = headInfo.PLATFORM_BUSINESS_CODE
	pageQuery := model.NewPageQuery(in.Page, in.PageSize)
	total, list, err := l.MenuModel.GetList(ind, menuPid, in.MenuIds, pageQuery)
	if err != nil {
		return nil, err
	}
	resp.Total, resp.Page, resp.PageSize = total, pageQuery.Page, pageQuery.PageSize

	for _, v := range list {
		item := &authRpc.MenuItem{}
		copier.Copy(item, v)
		// 列表显式回填父级存在性，根菜单也返回menuPid=0，避免与未传查询条件混淆。
		menuPid := v.MenuPid
		item.MenuPid = &menuPid
		if v.Version <= 0 {
			return nil, status.Error(codes.DataLoss, "菜单版本异常")
		}
		version := v.Version
		item.Version = &version
		resp.List = append(resp.List, item)
	}

	return resp, nil
}
