package svc

import (
	"os"

	"github.com/K7mQ2vR9/base-pkg/configcipher"
)

// encryptionKeyBase64 可由 -ldflags -X authRpc/internal/svc.encryptionKeyBase64 注入。
// 默认为空，不内置密钥；原部署须更新注入路径，密钥本身保持不变。
var encryptionKeyBase64 string

// newConfigCipher 仅在启动层解析密钥来源，加解密算法统一由公共包提供。
// 配置为空时依次读取MYSQL_CONFIG_KEY_BASE64和链接注入值，仍为空则由公共包返回错误，不修改原配置。
func newConfigCipher(options configcipher.Config) (*configcipher.Cipher, error) {
	if options.KeyBase64 == "" {
		options.KeyBase64 = os.Getenv("MYSQL_CONFIG_KEY_BASE64")
	}
	if options.KeyBase64 == "" {
		options.KeyBase64 = encryptionKeyBase64
	}
	return configcipher.New(options)
}
