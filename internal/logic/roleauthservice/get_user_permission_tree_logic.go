package roleauthservicelogic

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"encoding/json"
	"sort"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
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

// GetUserPermissionTree 按当前business-code返回全部分组及菜单，不以用户已有权限过滤目录。
// 无权限记录返回version=0且全部未勾选；软删除记录只保留版本，停用菜单展示为disabled。
func (l *GetUserPermissionTreeLogic) GetUserPermissionTree(in *authRpc.GetUserPermissionTreeReq) (*authRpc.GetUserPermissionTreeResp, error) {
	if in == nil || in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "userId必须为正数")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	data, err := l.model.GetTreeData(in.UserId, headInfo.GetBusinessCode(l.ctx))
	if err != nil {
		return nil, err
	}
	groups, err := buildUserPermissionGroups(l.ctx, data)
	if err != nil {
		return nil, err
	}
	resp := &authRpc.GetUserPermissionTreeResp{UserId: in.UserId, Groups: groups}
	if data.Permission != nil {
		resp.Version = data.Permission.Version
	}
	return resp, nil
}

// parseUserMenuIDs 仅解析用户记录中的英文逗号分隔正数ID，空字符串表示没有权限。
// 重复ID合并；不接受空项、符号或int64越界，不把异常记录静默当作空权限。
func parseUserMenuIDs(value string) (map[int64]struct{}, error) {
	parsed, err := mysql.ParseMenuIDs(value)
	if err != nil {
		return nil, status.Error(codes.DataLoss, "用户菜单ID格式异常")
	}
	ids := make(map[int64]struct{}, len(parsed))
	for _, id := range parsed {
		ids[id] = struct{}{}
	}
	return ids, nil
}

// parseGroupMenuIDs 沿用分组表的JSON数组；数据库NULL映射的空字符串及JSON null均表示空组。
// 分组仅组织展示，解析出的成员不能自动变成用户授权。
func parseGroupMenuIDs(value string) ([]int64, error) {
	if value == "" {
		return nil, nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return nil, status.Error(codes.DataLoss, "菜单分组成员格式异常")
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, status.Error(codes.DataLoss, "菜单分组成员格式异常")
		}
	}
	return ids, nil
}

// validateMenuParents 迭代检查父级链，阻止自身引用和祖先循环形成不可序列化的树。
// 不在当前业务目录中的父级不展开，保留可见节点作为根，不读取其他业务或已删除菜单。
func validateMenuParents(ctx context.Context, menus map[int64]*schema.Menu) error {
	finished := make(map[int64]bool, len(menus))
	for id := range menus {
		if err := ctx.Err(); err != nil {
			return err
		}
		if finished[id] {
			continue
		}
		path := make([]int64, 0)
		visiting := make(map[int64]bool)
		for current := id; current != 0 && menus[current] != nil && !finished[current]; current = menus[current].MenuPid {
			if visiting[current] || menus[current].MenuPid < 0 {
				return status.Error(codes.DataLoss, "菜单父级结构异常")
			}
			visiting[current] = true
			path = append(path, current)
		}
		for _, current := range path {
			finished[current] = true
		}
	}
	return nil
}

// buildUserPermissionGroups 为全部可见菜单标注用户已保存的选择，分组半选状态由前端计算。
// 未分组菜单放入ID为0的虚拟分组；同一菜单跨组独立建树，勾选状态始终取同一ID集合。
func buildUserPermissionGroups(ctx context.Context, data *mysql.UserPermissionTreeData) ([]*authRpc.UserPermissionGroup, error) {
	checked := make(map[int64]struct{})
	if data.Permission != nil && data.Permission.DeletedAt == 0 {
		var err error
		checked, err = parseUserMenuIDs(data.Permission.MenuIds)
		if err != nil {
			return nil, err
		}
	}
	menus := make(map[int64]*schema.Menu, len(data.Menus))
	ordered := append([]*schema.Menu(nil), data.Menus...)
	for _, menu := range ordered {
		if menu == nil || menu.ID <= 0 || menus[menu.ID] != nil {
			return nil, status.Error(codes.DataLoss, "菜单目录数据异常")
		}
		menus[menu.ID] = menu
	}
	if err := validateMenuParents(ctx, menus); err != nil {
		return nil, err
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Sort == ordered[j].Sort {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Sort < ordered[j].Sort
	})
	groups := make([]*authRpc.UserPermissionGroup, 0, len(data.Groups)+1)
	assigned := make(map[int64]bool, len(menus))
	for _, group := range data.Groups {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if group == nil || group.ID <= 0 {
			return nil, status.Error(codes.DataLoss, "菜单分组数据异常")
		}
		ids, err := parseGroupMenuIDs(group.MenuIds)
		if err != nil {
			return nil, err
		}
		members := make(map[int64]bool, len(ids))
		for _, id := range ids {
			// 过期、已删除或其他业务的成员引用不带入当前目录，也不据此创建虚假菜单。
			if menus[id] != nil {
				members[id] = true
				assigned[id] = true
			}
		}
		groups = append(groups, &authRpc.UserPermissionGroup{
			Id: group.ID, PermissionName: group.PermissionName,
			Menus: buildPermissionGroupTree(ordered, menus, members, checked),
		})
	}
	ungrouped := make(map[int64]bool)
	for _, menu := range ordered {
		if !assigned[menu.ID] {
			ungrouped[menu.ID] = true
		}
	}
	if len(ungrouped) > 0 {
		groups = append(groups, &authRpc.UserPermissionGroup{
			Id: 0, PermissionName: "未分组",
			Menus: buildPermissionGroupTree(ordered, menus, ungrouped, checked),
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

// buildPermissionGroupTree 在已校验无循环的目录中组装单个分组，按sort和ID保持稳定顺序。
// 仅为补齐结构的祖先标记displayOnly和disabled，不自动改变其授权或授予其他按钮。
func buildPermissionGroupTree(ordered []*schema.Menu, menus map[int64]*schema.Menu, members map[int64]bool, checked map[int64]struct{}) []*authRpc.UserPermissionMenu {
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
			Checked:     selected, Disabled: menu.Status != schema.MenuStatus1 || displayOnly,
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
