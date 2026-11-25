package roleauthservicelogic

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"sort"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetUserPermissionTreeLogic 为授权编辑页合并目录和用户勾选状态，不保存或复制模板。
type GetUserPermissionTreeLogic struct {
	logx.Logger
	ctx     context.Context
	model   *mysql.AdminPermissionModel
	initErr error
}

// NewGetUserPermissionTreeLogic 初始化请求级查询模型，构造失败由RPC方法返回。
func NewGetUserPermissionTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserPermissionTreeLogic {
	model, err := mysql.NewAdminPermissionModel(ctx, svcCtx)
	return &GetUserPermissionTreeLogic{
		Logger: logx.WithContext(ctx), ctx: ctx, model: model, initErr: err,
	}
}

// GetUserPermissionTree 返回共享分组及上限内有效菜单，删除、禁用、祖先失效或超出上限的权限项不展示。
// 10配置自身展示可选全部有效目录；给11赋权受模板1限制，其余按站点默认上限，查询不写库。
func (l *GetUserPermissionTreeLogic) GetUserPermissionTree(in *authRpc.GetUserPermissionTreeReq) (*authRpc.GetUserPermissionTreeResp, error) {
	if in == nil || in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "userId必须为正数")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	data, err := l.model.GetTreeData(in.UserId, in.BusinessCode)
	if err != nil {
		return nil, err
	}
	checked := make(map[int64]struct{})
	if data.Permission != nil && data.Permission.DeletedAt == 0 {
		checked, err = parseSelectedMenuIDs(data.Permission.MenuIds)
		if err != nil {
			return nil, err
		}
	}
	groups, err := buildPermissionGroups(l.ctx, data.Groups, data.Menus, checked, data.Selectable)
	if err != nil {
		return nil, err
	}
	resp := &authRpc.GetUserPermissionTreeResp{UserId: in.UserId, Groups: groups}
	if data.Permission != nil {
		resp.Version = data.Permission.Version
	}
	return resp, nil
}

// parseSelectedMenuIDs 共用用户/模板显式勾选CSV解析，空字符串表示没有勾选，不推导父子授权。
// 重复ID合并；不接受空项、符号或int64越界，不把异常记录静默当作空权限。
func parseSelectedMenuIDs(value string) (map[int64]struct{}, error) {
	parsed, err := mysql.ParseMenuIDs(value)
	if err != nil {
		return nil, status.Error(codes.DataLoss, "已选菜单ID格式异常")
	}
	ids := make(map[int64]struct{}, len(parsed))
	for _, id := range parsed {
		ids[id] = struct{}{}
	}
	return ids, nil
}

// buildPermissionGroups 为用户或模板配置页共用分组树组装，checked来自调用方已校验的独立集合。
// 超出当前上限的权限项直接过滤，不保留禁用勾选；必要祖先仅补齐结构，不扩大权限或在查询时写库。
// 未分组使用虚拟ID0，同一菜单跨组勾选一致，不初始化实体，也不自动扩展授权。
func buildPermissionGroups(ctx context.Context, sourceGroups []*schema.PermissionMenu, sourceMenus []*schema.Menu, checked map[int64]struct{}, selectable map[int64]bool) ([]*authRpc.UserPermissionGroup, error) {
	menus := make(map[int64]*schema.Menu, len(sourceMenus))
	ordered := append([]*schema.Menu(nil), sourceMenus...)
	for _, menu := range ordered {
		if menu == nil || menu.ID <= 0 || menus[menu.ID] != nil {
			return nil, status.Error(codes.DataLoss, "菜单目录数据异常")
		}
		menus[menu.ID] = menu
	}
	if err := mysql.ValidateMenuParents(ctx, menus); err != nil {
		return nil, err
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Sort == ordered[j].Sort {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Sort < ordered[j].Sort
	})
	groups := make([]*authRpc.UserPermissionGroup, 0, len(sourceGroups)+1)
	assigned := make(map[int64]bool, len(menus))
	for _, group := range sourceGroups {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if group == nil || group.ID <= 0 {
			return nil, status.Error(codes.DataLoss, "菜单分组数据异常")
		}
		ids, err := mysql.ParseGroupMenuIDs(group.MenuIds)
		if err != nil {
			return nil, status.Error(codes.DataLoss, "菜单分组成员格式异常")
		}
		members := make(map[int64]bool, len(ids))
		for _, id := range ids {
			// 仅把当前上限内有效菜单作为权限项，旧勾选不使越界、失效或其他业务的引用继续展示。
			if menus[id] != nil && selectable[id] {
				members[id] = true
				assigned[id] = true
			}
		}
		groups = append(groups, &authRpc.UserPermissionGroup{
			Id: group.ID, PermissionName: group.PermissionName,
			Menus: buildPermissionGroupTree(ordered, menus, members, checked, selectable),
		})
	}
	ungrouped := make(map[int64]bool)
	for _, menu := range ordered {
		if selectable[menu.ID] && !assigned[menu.ID] {
			ungrouped[menu.ID] = true
		}
	}
	if len(ungrouped) > 0 {
		groups = append(groups, &authRpc.UserPermissionGroup{
			Id: 0, PermissionName: "未分组",
			Menus: buildPermissionGroupTree(ordered, menus, ungrouped, checked, selectable),
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

// buildPermissionGroupTree 在已校验无循环的目录中组装单个分组，按sort和ID保持稳定顺序。
// 权限成员已过滤到上限内，disabled仅用于补齐结构的祖先；上限外祖先不回显勾选或授予权限。
func buildPermissionGroupTree(ordered []*schema.Menu, menus map[int64]*schema.Menu, members map[int64]bool, checked map[int64]struct{}, selectable map[int64]bool) []*authRpc.UserPermissionMenu {
	included := make(map[int64]bool, len(members))
	for id := range members {
		for current := id; current != 0 && menus[current] != nil && !included[current]; current = menus[current].MenuPid {
			included[current] = true
		}
	}
	nodes := make(map[int64]*authRpc.UserPermissionMenu, len(included))
	for _, menu := range ordered {
		if !included[menu.ID] {
			continue
		}
		_, selected := checked[menu.ID]
		displayOnly := !members[menu.ID]
		nodes[menu.ID] = &authRpc.UserPermissionMenu{
			Id: menu.ID, MenuPid: menu.MenuPid, MenuName: menu.MenuName,
			MenuType: int32(menu.MenuType), Path: menu.Path, Status: int32(menu.Status), Sort: int32(menu.Sort),
			ServiceName: menu.ServiceName,
			Checked:     selected && selectable[menu.ID], Disabled: displayOnly,
			DisplayOnly: displayOnly, Children: make([]*authRpc.UserPermissionMenu, 0),
		}
	}
	roots := make([]*authRpc.UserPermissionMenu, 0)
	for _, menu := range ordered {
		node := nodes[menu.ID]
		if node == nil {
			continue
		}
		if parent := nodes[menu.MenuPid]; parent != nil {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}
	return roots
}
