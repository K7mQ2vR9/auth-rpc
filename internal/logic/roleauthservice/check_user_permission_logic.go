package roleauthservicelogic

import (
	"authRpc/authRpc"
	"authRpc/internal/dao/model/mysql"
	"authRpc/internal/svc"
	"context"
	"strings"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CheckUserPermissionLogic 按身份、站点根上限及完整菜单链检查有效权限，不改变RPC中间件metadata准入规则。
type CheckUserPermissionLogic struct {
	logx.Logger
	ctx     context.Context
	model   *mysql.AdminPermissionModel
	initErr error
}

// NewCheckUserPermissionLogic 创建请求级鉴权模型；数据库校验集中在对应模型文件中。
func NewCheckUserPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckUserPermissionLogic {
	model, err := mysql.NewAdminPermissionModel(ctx, svcCtx)
	return &CheckUserPermissionLogic{
		Logger: logx.WithContext(ctx), ctx: ctx, model: model, initErr: err,
	}
}

// CheckUserPermission 成功返回空响应，无权限及数据库异常均返回错误，不默认放行。
// userId由API从认证结果取得，不能直接采信前端任意指定的用户ID。
// 平台与站点身份按数据库精确类型分支，10默认全操作；其他身份按serviceName、path及默认上限精确匹配，不使用自定义模板继承。
func (l *CheckUserPermissionLogic) CheckUserPermission(in *authRpc.CheckUserPermissionReq) (*authRpc.CheckUserPermissionResp, error) {
	if in == nil || in.UserId <= 0 || strings.TrimSpace(in.Path) == "" {
		return nil, status.Error(codes.InvalidArgument, "userId必须为正数且path不能为空")
	}
	if err := mysql.ValidateServiceName(in.ServiceName); err != nil {
		return nil, err
	}
	if l.initErr != nil {
		return nil, l.initErr
	}
	if err := l.model.CheckPermission(in.UserId, in.ServiceName, in.Path, headInfo.GetBusinessCode(l.ctx)); err != nil {
		return nil, err
	}
	return &authRpc.CheckUserPermissionResp{}, nil
}
