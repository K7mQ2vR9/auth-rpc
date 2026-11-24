package middleware

import "context"

// GetWhitePath 是本服务RPC白名单的唯一配置入口，值不参与判断，不会自动加入已注册方法。
// 命中只豁免business-code必填，metadata、合法Trace及字段格式仍校验；空名单不豁免任何方法。
// 使用实际RPC完整方法名，只有与当前服务的方法匹配才生效；名单不用于API用户认证。
func GetWhitePath(ctx context.Context) map[string]int {
	return map[string]int{
		//"/commonRpc.CommunicationService/sendTgMsg": 1,
	}
}
