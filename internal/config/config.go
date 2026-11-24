package config

import (
	"github.com/K7mQ2vR9/base-pkg/configcipher"
	"github.com/K7mQ2vR9/base-pkg/dbM"
	"github.com/zeromicro/go-zero/zrpc"
)

// DatabaseKey 沿用现有权限数据的连接标识，不因迁移而更换数据库或表。
const DatabaseKey = "user-rpc"

// Config 直接使用公共包的MySQL/Redis配置结构，资源由启动代码显式创建，不兼容旧平铺数据库格式。
type Config struct {
	zrpc.RpcServerConf
	// SelectDb 必须包含DatabaseKey连接，参数放入MySQL子结构；生产模式MySQL.DataSource仍使用v1:密文。
	// 未配置的连接池、超时及SQL日志参数使用公共包默认值，不按运行模式覆盖。
	SelectDb []dbM.SelectDbConfig `json:",optional"`
	// RedisConf 为可选配置，nil不创建客户端；生产模式Password仍使用v1:密文。
	// TLS与证书校验开关分别按公共包规则显式配置，不从InsecureSkipVerify推断TLS开关。
	RedisConf *dbM.RedisConfig `json:",optional"`
	// ConfigCipher 配置部署密文的解密密钥，与请求和响应的AES开关无关。
	// 生产模式KeyBase64为空时，由启动层依次读取MYSQL_CONFIG_KEY_BASE64和本项目链接注入值，无默认密钥。
	ConfigCipher configcipher.Config `json:",optional"`
}
