package roleauthservicelogic

import (
	"authRpc/internal/dao/model"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"github.com/jinzhu/copier"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetPermissionListLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.PermissionMenuModel
}

// NewGetPermissionListLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewGetPermissionListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPermissionListLogic {
	logic := &GetPermissionListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
	permissionMenuModel, err := mysql.NewPermissionMenuModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.PermissionMenuModel = permissionMenuModel

	return logic
}

// GetPermissionList 完整回显共享分组的未删除成员及状态，禁用成员保留；授权页面另行过滤失效节点。
func (l *GetPermissionListLogic) GetPermissionList(in *authRpc.GetPermissionListReq) (*authRpc.GetPermissionListResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "请求不能为空")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}

	resp := &authRpc.GetPermissionListResp{
		List:     make([]*authRpc.PermissionListItem, 0),
		Page:     in.Page,
		PageSize: in.PageSize,
		Total:    0,
	}

	ind := &schema.PermissionMenu{}
	if in.Data != nil {
		copier.Copy(ind, in.Data)
	}
	pageQuery := model.NewPageQuery(in.Page, in.PageSize)
	total, list, err := l.PermissionMenuModel.GetList(ind, pageQuery)
	if err != nil {
		return nil, err
	}
	resp.Total, resp.Page, resp.PageSize = total, pageQuery.Page, pageQuery.PageSize

	membersByGroup := make(map[int64][]int64, len(list))
	union := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, group := range list {
		if group == nil || group.ID <= 0 || group.Version <= 0 {
			return nil, status.Error(codes.DataLoss, "分组记录或版本异常")
		}
		ids, err := mysql.ParseGroupMenuIDs(group.MenuIds)
		if err != nil {
			return nil, status.Error(codes.DataLoss, "菜单分组成员格式异常")
		}
		membersByGroup[group.ID] = ids
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				union = append(union, id)
			}
		}
	}
	menus, err := l.PermissionMenuModel.ListMemberMenus(union)
	if err != nil {
		return nil, err
	}
	menuByID := make(map[int64]*schema.Menu, len(menus))
	for _, menu := range menus {
		menuByID[menu.ID] = menu
	}
	for _, group := range list {
		item := &authRpc.PermissionListItem{Id: group.ID, PermissionName: group.PermissionName, CreatedAt: group.CreatedAt, Version: group.Version, Menus: make([]*authRpc.PermissionListMenuItem, 0)}
		for _, id := range membersByGroup[group.ID] {
			if menu := menuByID[id]; menu != nil {
				item.Menus = append(item.Menus, &authRpc.PermissionListMenuItem{Id: menu.ID, MenuName: menu.MenuName, Status: int32(menu.Status)})
			}
		}
		resp.List = append(resp.List, item)
	}

	return resp, nil
}
