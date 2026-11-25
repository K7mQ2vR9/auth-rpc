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

type AddMenuLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.MenuModel
}

// NewAddMenuLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewAddMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddMenuLogic {
	logic := &AddMenuLogic{
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

// AddMenu 仅平台账号10新增共享菜单，模型校验真实资格；API服务名必填，归属固定为平台。
func (l *AddMenuLogic) AddMenu(in *authRpc.AddMenuReq) (*authRpc.AddMenuResp, error) {
	if in == nil || in.Data == nil {
		return nil, status.Error(codes.InvalidArgument, "菜单data不能为空")
	}
	if err := mysql.ValidateServiceName(in.Data.ServiceName); err != nil {
		return nil, err
	}

	if l.initErr != nil {
		return nil, l.initErr
	}

	ind := &schema.Menu{}
	copier.Copy(ind, in.Data)
	// 写入仍为完整数据，省略父级按0（根菜单）保存，不受查询字段可选性的影响。
	ind.MenuPid = in.Data.GetMenuPid()
	ind.Version = in.Data.GetVersion()
	ind.BusinessCode = headInfo.GetBusinessCode(l.ctx)
	err := l.MenuModel.InsertSchema(ind)
	if err != nil {
		return nil, err
	}
	return &authRpc.AddMenuResp{Id: ind.ID, Version: ind.Version}, nil
}
