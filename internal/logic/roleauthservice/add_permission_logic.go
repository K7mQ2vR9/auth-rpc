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

type AddPermissionLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.PermissionMenuModel
}

// NewAddPermissionLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewAddPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddPermissionLogic {
	logic := &AddPermissionLogic{
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

// 权限板块
func (l *AddPermissionLogic) AddPermission(in *authRpc.AddPermissionReq) (*authRpc.AddPermissionResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	ind := &schema.PermissionMenu{}
	copier.Copy(ind, in.Data)
	err := l.PermissionMenuModel.InsertSchema(ind)
	if err != nil {
		return nil, err
	}

	return &authRpc.AddPermissionResp{Id: ind.ID}, nil
}
