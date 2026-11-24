package roleauthservicelogic

import (
	"authRpc/internal/dao/model"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"encoding/json"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/jinzhu/copier"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRoleListLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.RoleModel
	*mysql.PermissionMenuModel
}

// NewGetRoleListLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewGetRoleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoleListLogic {
	logic := &GetRoleListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
	roleModel, err := mysql.NewRoleModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.RoleModel = roleModel

	permissionMenuModel, err := mysql.NewPermissionMenuModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.PermissionMenuModel = permissionMenuModel
	return logic
}

func (l *GetRoleListLogic) GetRoleList(in *authRpc.GetRoleListReq) (*authRpc.GetRoleListResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	resp := &authRpc.GetRoleListResp{
		List:     make([]*authRpc.RoleListItem, 0),
		Page:     in.Page,
		PageSize: in.PageSize,
		Total:    0,
	}

	if headInfo.GetBusinessCode(l.ctx) != headInfo.PLATFORM_BUSINESS_CODE {
		tm := headInfo.GetBusinessCode(l.ctx)
		in.Data.BusinessCode = tm[:5] + "000"
	}

	permissionListMap := make(map[int64]*schema.PermissionMenu)
	permissionList, err := l.PermissionMenuModel.GetAll(&schema.PermissionMenu{BusinessCode: in.Data.BusinessCode})
	if err != nil {
		return nil, err
	}
	for _, v := range permissionList {
		permissionListMap[v.ID] = v
	}

	ind := &schema.RolePermission{}
	if in.Data != nil {
		copier.Copy(ind, in.Data)
	}
	ind.BusinessCode = in.Data.BusinessCode
	total, list, err := l.RoleModel.GetList(ind, model.NewPageQuery(in.Page, in.PageSize))
	if err != nil {
		return nil, err
	}
	resp.Total = total

	ids := make([]int64, 0)
	for _, v := range list {
		roleitem := &authRpc.RoleListItem{}
		copier.Copy(roleitem, v)
		json.Unmarshal([]byte(v.PermissionIds), &ids)
		roleitem.PermissionIds = make([]*authRpc.RoleListPermissionItem, 0)
		for _, v1 := range ids {
			permissionItem := &authRpc.RoleListPermissionItem{}
			if da, ok := permissionListMap[v1]; ok {
				permissionItem.PermissionName = da.PermissionName
				permissionItem.Id = da.ID
				roleitem.PermissionIds = append(roleitem.PermissionIds, permissionItem)
			}
		}
		roleitem.IsSuperRole = int32(v.IsSuperRole)
		resp.List = append(resp.List, roleitem)
	}

	return resp, nil
}
