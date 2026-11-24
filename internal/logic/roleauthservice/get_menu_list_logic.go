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

func (l *GetMenuListLogic) GetMenuList(in *authRpc.GetMenuListReq) (*authRpc.GetMenuListResp, error) {
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
	if in.Data != nil {
		copier.Copy(ind, in.Data)
	}
	ind.BusinessCode = headInfo.GetBusinessCode(l.ctx)
	total, list, err := l.MenuModel.GetList(ind, in.MenuIds, model.NewPageQuery(in.Page, in.PageSize))
	if err != nil {
		return nil, err
	}
	resp.Total = total

	for _, v := range list {
		item := &authRpc.MenuItem{}
		copier.Copy(item, v)
		resp.List = append(resp.List, item)
	}

	return resp, nil
}
