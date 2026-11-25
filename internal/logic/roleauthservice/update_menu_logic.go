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

// UpdateMenu 仅平台账号10修改共享菜单，模型校验真实资格和目标归属；必须提交原version，成功返回新版本。
func (l *UpdateMenuLogic) UpdateMenu(in *authRpc.UpdateMenuReq) (*authRpc.UpdateMenuResp, error) {
	if in == nil || in.Data == nil || in.Data.Version == nil {
		return nil, status.Error(codes.InvalidArgument, "菜单data不能为空且必须显式提交version")
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
	// 修改仍完整覆盖父级，省略或显式0均移动到根，不表示保留原父级。
	ind.MenuPid = in.Data.GetMenuPid()
	ind.Version = in.Data.GetVersion()
	ind.BusinessCode = headInfo.GetBusinessCode(l.ctx)
	err := l.MenuModel.UpdateAll(ind.ID, ind)
	if err != nil {
		return nil, err
	}

	return &authRpc.UpdateMenuResp{Version: ind.Version}, nil
}
