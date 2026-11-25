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

// GetMenuByAdminId 读取当前business-code下用户独立获授的有效菜单，不再要求或读取用户绑定角色。
// 无有效授权返回空列表；用户异常、存储损坏及数据库错误返回错误，不开启事务或通过角色回退。
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

// buildGrantedMenuTree 按模型给出的sort、ID顺序组装直接授权菜单，拒绝无效ID及祖先循环。
// 父节点不在已授予集合时将当前节点放在返回根列表，不加载或补授父节点，MenuPid保留原值。
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
