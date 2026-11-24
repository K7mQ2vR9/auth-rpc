package roleauthservicelogic

import (
	"authRpc/internal/dao/model/mysql"
	"context"
	"github.com/jinzhu/copier"

	"authRpc/authRpc"
	"authRpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetOneRoleLogic struct {
	// initErr 防止模型取库失败后继续执行权限业务或解引用空模型。
	initErr error
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	logx.Logger
	*mysql.RoleModel
}

// NewGetOneRoleLogic 初始化请求模型，取库错误保存在当前Logic并由业务入口返回。
func NewGetOneRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOneRoleLogic {
	logic := &GetOneRoleLogic{
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
	return logic
}

func (l *GetOneRoleLogic) GetOneRole(in *authRpc.GetOneRoleReq) (*authRpc.GetOneRoleResp, error) {
	if l.initErr != nil {
		return nil, l.initErr
	}

	resp := &authRpc.GetOneRoleResp{Data: &authRpc.RoleListItem{}}
	info := l.RoleModel.FindOneById(in.Id)
	copier.Copy(resp.Data, info)
	return resp, nil
}
