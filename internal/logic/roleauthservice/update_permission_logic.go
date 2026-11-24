package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"errors"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/jinzhu/copier"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePermissionLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.PermissionMenuModel
}

// NewUpdatePermissionLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewUpdatePermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePermissionLogic {
	logic := &UpdatePermissionLogic{
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

func (l *UpdatePermissionLogic) UpdatePermission(in *authRpc.UpdatePermissionReq) (*authRpc.UpdatePermissionResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	if in.Data.Id == schema.SitePermissionId && headInfo.GetBusinessCode(l.ctx) != headInfo.PLATFORM_BUSINESS_CODE {
		return nil, errors.New("初始数据不能被修改")
	}
	ind := &schema.PermissionMenu{}
	copier.Copy(ind, in.Data)
	err := l.PermissionMenuModel.UpdateByMap(ind.ID, ind)
	if err != nil {
		return nil, err
	}
	return &authRpc.UpdatePermissionResp{}, nil
}
