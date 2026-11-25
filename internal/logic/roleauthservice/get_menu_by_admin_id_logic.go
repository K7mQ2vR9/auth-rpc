package roleauthservicelogic

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"

	"authRpc/internal/svc"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetMenuByAdminIdLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	logx.Logger
	model *mysql.AdminPermissionModel
}

// NewGetMenuByAdminIdLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewGetMenuByAdminIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMenuByAdminIdLogic {
	logic := &GetMenuByAdminIdLogic{
		ctx:    ctx,
		Logger: logx.WithContext(ctx),
	}
	permissionModel, err := mysql.NewAdminPermissionModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.model = permissionModel
	return logic
}

// GetMenuByAdminId 所有账号统一读取已配置菜单（包括10），共用身份、站点上限及祖先校验，不自动返回全量。
// 无有效授权返回空列表；身份或根不可用、存储损坏及数据库错误返回错误，普通查询不加读事务。
func (l *GetMenuByAdminIdLogic) GetMenuByAdminId(in *authRpc.GetMenuByAdminIdReq) (*authRpc.GetMenuByAdminIdResp, error) {
	if in == nil || in.AdminId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "adminId必须为正数")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}

	menus, err := l.model.GetGrantedMenus(in.AdminId, headInfo.GetBusinessCode(l.ctx))
	if err != nil {
		return nil, err
	}
	list, err := buildGrantedMenuTree(l.ctx, menus)
	if err != nil {
		return nil, err
	}
	return &authRpc.GetMenuByAdminIdResp{List: list}, nil
}

// buildGrantedMenuTree 按模型给出的sort、ID顺序组装已通过完整有效性检查的菜单，拒绝无效ID及集合内循环。
// 已校验有效但未获授的父节点不加入列表，子节点作为返回根并保留MenuPid；缺失或停用祖先已由模型过滤。
func buildGrantedMenuTree(ctx context.Context, menus []*schema.Menu) ([]*authRpc.GetMenuByAdminIdItem, error) {
	menuByID := make(map[int64]*schema.Menu, len(menus))
	for _, menu := range menus {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if menu == nil || menu.ID <= 0 || menuByID[menu.ID] != nil {
			return nil, status.Error(codes.DataLoss, "菜单目录数据异常")
		}
		menuByID[menu.ID] = menu
	}
	if err := mysql.ValidateMenuParents(ctx, menuByID); err != nil {
		return nil, err
	}
	nodes := make(map[int64]*authRpc.GetMenuByAdminIdItem, len(menus))
	for _, menu := range menus {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		nodes[menu.ID] = &authRpc.GetMenuByAdminIdItem{
			Id: menu.ID, MenuName: menu.MenuName, Path: menu.Path,
			MenuPid: menu.MenuPid, MenuType: int32(menu.MenuType), ServiceName: menu.ServiceName,
			Children: make([]*authRpc.GetMenuByAdminIdItem, 0),
		}
	}
	roots := make([]*authRpc.GetMenuByAdminIdItem, 0)
	for _, menu := range menus {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		node := nodes[menu.ID]
		if parent := nodes[menu.MenuPid]; parent != nil {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return roots, nil
}
