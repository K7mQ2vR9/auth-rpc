package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"context"
	"github.com/jinzhu/copier"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type InitSuperRoleLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.RoleModel
}

// NewInitSuperRoleLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewInitSuperRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InitSuperRoleLogic {
	logic := &InitSuperRoleLogic{
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

func (l *InitSuperRoleLogic) InitSuperRole(in *authRpc.InitSuperRoleReq) (*authRpc.InitSuperRoleResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	resp := &authRpc.InitSuperRoleResp{Data: &authRpc.RoleItem{}}

	role, err := l.RoleModel.CreatedSuperRole(in.BusinessCode)
	if err != nil {
		return nil, err
	}

	copier.Copy(resp.Data, &role)
	return resp, nil
}
