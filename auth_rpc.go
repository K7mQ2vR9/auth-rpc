package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"authRpc/authRpc"
	"authRpc/internal/config"
	"authRpc/internal/middleware"
	server "authRpc/internal/server/roleauthservice"
	"authRpc/internal/svc"
	"github.com/K7mQ2vR9/base-pkg/app"
	"github.com/K7mQ2vR9/base-pkg/appMiddleware"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/auth_rpc.yaml", "权限服务配置文件路径")

func main() {
	flag.Parse()
	if err := run(); err != nil {
		// run已完成资源清理，主入口仅记录脱敏错误并返回失败退出码。
		log.Printf("auth-rpc stopped: %v", err)
		os.Exit(1)
	}
}

// run 读取配置并装配服务，返回前先停止RPC，再释放数据资源，初始化失败同样清理。
func run() (err error) {
	var c config.Config
	if loadErr := conf.Load(*configFile, &c, conf.UseEnv()); loadErr != nil {
		// 配置解析错误可能含部署凭证，不回显原始配置值。
		return errors.New("load auth-rpc configuration failed")
	}
	app.InitAppServer()
	svcCtx, err := svc.NewServiceContext(context.Background(), c)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, svcCtx.Close()) }()
	s, err := zrpc.NewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		authRpc.RegisterRoleAuthServiceServer(grpcServer, server.NewRoleAuthServiceServer(svcCtx))
		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	if err != nil {
		return err
	}
	defer s.Stop()
	// 所有RPC先校验metadata/Trace，business-code豁免只读取本项目GetWhitePath，不按用户ID鉴权。
	s.AddOptions(appMiddleware.NewRpcAuthMiddleware().ServerOption(middleware.GetWhitePath))
	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
	return nil
}
