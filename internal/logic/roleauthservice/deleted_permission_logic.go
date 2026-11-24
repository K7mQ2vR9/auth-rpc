package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"errors"
	"fmt"
	"gorm.io/plugin/soft_delete"
	"time"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeletedPermissionLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.PermissionMenuModel
	*mysql.RoleModel
}

// NewDeletedPermissionLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewDeletedPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeletedPermissionLogic {
	logic := &DeletedPermissionLogic{
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

	roleModel, err := mysql.NewRoleModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.RoleModel = roleModel
	return logic
}

func (l *DeletedPermissionLogic) DeletedPermission(in *authRpc.DeletedPermissionReq) (*authRpc.DeletedPermissionResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	if in.Id == schema.SitePermissionId {
		return nil, errors.New("init data not be delete")
	}

	roleInfo := l.RoleModel.CountPermissionId(in.Id)
	if roleInfo.ID != 0 {
		return nil, errors.New(fmt.Sprintf("Role [ %s ] have set, can not be delete", roleInfo.RoleName))
	}

	err := l.PermissionMenuModel.UpdateByMap(in.Id, &schema.PermissionMenu{DeletedAt: soft_delete.DeletedAt(time.Now().Unix())})
	if err != nil {
		return nil, err
	}

	return &authRpc.DeletedPermissionResp{}, nil
}
