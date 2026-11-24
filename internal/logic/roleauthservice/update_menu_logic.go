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

func (l *UpdateMenuLogic) UpdateMenu(in *authRpc.UpdateMenuReq) (*authRpc.UpdateMenuResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	if in.Data.MenuPid != 0 {
		pInfo := l.MenuModel.FindOne(&schema.Menu{
			ID: in.Data.MenuPid,
		})
		if pInfo.ID == 0 {
			return nil, errors.New("Pid menu not found")
		}
	}

	if in.Data.Path != "" {
		oInfo := l.MenuModel.FindOne(&schema.Menu{
			Path: in.Data.Path,
		})
		if oInfo.ID != 0 && oInfo.ID != in.Data.Id {
			return nil, errors.New("Path is exsit")
		}
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
