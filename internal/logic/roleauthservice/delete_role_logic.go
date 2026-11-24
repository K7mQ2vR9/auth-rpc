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

type DeleteRoleLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.RoleModel
	*mysql.AdminInfoModel
}

// NewDeleteRoleLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewDeleteRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteRoleLogic {
	logic := &DeleteRoleLogic{
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

	adminInfoModel, err := mysql.NewAdminInfoModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.AdminInfoModel = adminInfoModel
	return logic
}

func (l *DeleteRoleLogic) DeleteRole(in *authRpc.DeleteRoleReq) (*authRpc.DeleteRoleResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	one := l.RoleModel.FindOne(&schema.RolePermission{ID: in.Id})
	if one.IsSuperRole == schema.RolePermissionIsSuperRole1 {
		return nil, errors.New("super role not be delete")
	}

	adminInfo := l.AdminInfoModel.FindOne(&schema.AdminInfo{
		RoleId: in.Id,
	})
	if adminInfo.ID != 0 {
		return nil, errors.New(fmt.Sprintf("Admin [ %s ] have set, can not be delete", adminInfo.Name))
	}

	err := l.RoleModel.UpdateByMap(in.Id, &schema.RolePermission{
		DeletedAt: soft_delete.DeletedAt(time.Now().Unix()),
	})
	if err != nil {
		return nil, err
	}

	return &authRpc.DeleteRoleResp{}, nil
}
