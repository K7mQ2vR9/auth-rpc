package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"github.com/jinzhu/copier"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddRoleLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.RoleModel
}

// NewAddRoleLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewAddRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddRoleLogic {
	logic := &AddRoleLogic{
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
	return logic
}

// 角色管理
func (l *AddRoleLogic) AddRole(in *authRpc.AddRoleReq) (*authRpc.AddRoleResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	ind := &schema.RolePermission{}
	copier.Copy(ind, in.Data)
	ind.BusinessCode = in.Data.BusinessCode
	ind.IsSuperRole = int(in.Data.IsSuperRole)
	err := l.RoleModel.InsertSchema(ind)
	if err != nil {
		return nil, err
	}

	return &authRpc.AddRoleResp{Id: ind.ID}, nil
}
