package svc

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"authRpc/internal/config"
	"github.com/K7mQ2vR9/base-pkg/configcipher"
	"github.com/K7mQ2vR9/base-pkg/dbM"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/service"
)

// ServiceContext 持有服务级连接池，模型用请求ctx派生会话；开始使用后不得按值复制。
type ServiceContext struct {
	Config config.Config
	// RedisClient 支持node/cluster；未配置RedisConf时为nil。
	RedisClient redis.UniversalClient
	DbSelect    *dbM.SelectDb
	closeOnce   sync.Once
	closeErr    error
}

// NewServiceContext 直接按公共包的配置格式初始化资源，生产模式仅在副本中解密受保护的配置值。
// 默认值及校验由公共包提供，不转换旧格式或覆盖日志/TLS策略；失败回收已创建资源。
// 数据库和Redis各自使用公共包的启动预算，启动成功后取消ctx不关闭共享资源。
func NewServiceContext(ctx context.Context, c config.Config) (*ServiceContext, error) {
	if ctx == nil {
		return nil, errors.New("auth-rpc startup context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	foundDatabase := false
	for _, entry := range c.SelectDb {
		if entry.DbKey == config.DatabaseKey {
			foundDatabase = true
		}
	}
	if !foundDatabase {
		return nil, errors.New("required mysql database key user-rpc is not configured")
	}
	var decryptor *configcipher.Cipher
	if c.Mode == service.ProMode {
		var err error
		decryptor, err = newConfigCipher(c.ConfigCipher)
		if err != nil {
			return nil, err
		}
	}
	// 配置形状与公共包一致；复制切片仅为解密时不修改原配置或将密文覆盖成明文。
	databases := append([]dbM.SelectDbConfig(nil), c.SelectDb...)
	if decryptor != nil {
		for index := range databases {
			plaintext, err := decryptor.Decrypt(databases[index].MySQL.DataSource)
			if err != nil {
				return nil, fmt.Errorf("decrypt SelectDb[%d].MySQL.DataSource: %w", index, err)
			}
			databases[index].MySQL.DataSource = plaintext
		}
	}
	var redisConfig *dbM.RedisConfig
	if c.RedisConf != nil {
		copyConfig := *c.RedisConf
		if decryptor != nil {
			plaintext, err := decryptor.Decrypt(copyConfig.Password)
			if err != nil {
				return nil, fmt.Errorf("decrypt RedisConf.Password: %w", err)
			}
			copyConfig.Password = plaintext
		}
		redisConfig = &copyConfig
	}
	manager, err := dbM.NewSelectDb(ctx, databases)
	if err != nil {
		return nil, fmt.Errorf("initialize mysql: %w", err)
	}
	result := &ServiceContext{Config: c, DbSelect: manager}
	if redisConfig != nil {
		result.RedisClient, err = dbM.NewRedisClient(ctx, *redisConfig)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("initialize redis: %w", err), result.Close())
		}
	}
	return result, nil
}

// Close 在RPC停止接收业务并完成收尾后释放连接池，可并发/重复调用，返回首次关闭的汇总结果。
func (s *ServiceContext) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		var redisErr error
		if s.RedisClient != nil {
			redisErr = s.RedisClient.Close()
		}
		s.closeErr = errors.Join(redisErr, s.DbSelect.Close())
	})
	return s.closeErr
}
