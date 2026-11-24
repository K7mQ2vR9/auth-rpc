package roleauthservicelogic

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"errors"

	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMenuByAdminIdLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.AdminRoleModel
	*mysql.AdminInfoModel
}

// NewGetMenuByAdminIdLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewGetMenuByAdminIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMenuByAdminIdLogic {
	logic := &GetMenuByAdminIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
	adminRoleModel, err := mysql.NewAdminRoleModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.AdminRoleModel = adminRoleModel

	adminInfoModel, err := mysql.NewAdminInfoModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.AdminInfoModel = adminInfoModel
	return logic
}

func (l *GetMenuByAdminIdLogic) GetMenuByAdminId(in *authRpc.GetMenuByAdminIdReq) (*authRpc.GetMenuByAdminIdResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	resp := &authRpc.GetMenuByAdminIdResp{List: make([]*authRpc.GetMenuByAdminIdItem, 0)}
	adminInfo := l.AdminInfoModel.FindOne(&schema.AdminInfo{
		ID: in.AdminId,
	})
	if adminInfo.ID == 0 {
		return nil, errors.New("Admin not found")
	}
	if adminInfo.RoleId == 0 {
		return nil, errors.New("Admin role not set")
	}
	list, err := l.AdminRoleModel.GetAdminRoleMenu(adminInfo.RoleId)
	if err != nil {
		return nil, err
	}
	resp.List = list

	return resp, nil
}
