package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"context"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeleteMenuLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.MenuModel
}

// NewDeleteMenuLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewDeleteMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMenuLogic {
	logic := &DeleteMenuLogic{
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

// DeleteMenu 仅账号10按已读version软删除平台共享菜单，冲突拒绝；分组引用不阻止删除，不级联清理模板或用户授权。
func (l *DeleteMenuLogic) DeleteMenu(in *authRpc.DeleteMenuReq) (*authRpc.DeleteMenuResp, error) {
	if in == nil || in.Id <= 0 || in.Version == nil {
		return nil, status.Error(codes.InvalidArgument, "菜单ID须为正数且必须显式提交version")
	}
	if l.initErr != nil {
		return nil, l.initErr
	}

	err := l.MenuModel.DeleteByID(in.Id, in.GetVersion())
	if err != nil {
		return nil, err
	}
	return &authRpc.DeleteMenuResp{Version: in.GetVersion() + 1}, nil
}
