package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/dao/schema"
	"context"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/jinzhu/copier"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateMenuLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.MenuModel
}

// NewUpdateMenuLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewUpdateMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMenuLogic {
	logic := &UpdateMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
	menuModel, err := mysql.NewMenuModel(ctx, svcCtx)
	if err != nil {
		logic.initErr = err
		return logic
	}
	logic.MenuModel = menuModel
	return logic
}

// UpdateMenu 修改菜单每次都须传入API服务名，避免服务字段缺失或被默认归入其他服务。
func (l *UpdateMenuLogic) UpdateMenu(in *authRpc.UpdateMenuReq) (*authRpc.UpdateMenuResp, error) {
	if in == nil || in.Data == nil {
		return nil, status.Error(codes.InvalidArgument, "菜单data不能为空")
	}
	if err := mysql.ValidateServiceName(in.Data.ServiceName); err != nil {
		return nil, err
	}
	if in.Data.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "修改菜单ID必须为正数")
	}

	if l.initErr != nil {
		return nil, l.initErr
	}

	ind := &schema.Menu{}
	copier.Copy(ind, in.Data)
	ind.BusinessCode = headInfo.GetBusinessCode(l.ctx)
	err := l.MenuModel.UpdateAll(ind.ID, ind)
	if err != nil {
		return nil, err
	}

	return &authRpc.UpdateMenuResp{}, nil
}
