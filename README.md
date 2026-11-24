# auth-rpc 权限服务接入与迁移说明

auth-rpc提供角色、权限、菜单管理、管理员菜单查询、路径权限判断及超级角色初始化。Go模块名为 `authRpc`，项目内导入统一使用 `authRpc/...`，例如protobuf Go包路径为 `authRpc/authRpc`。protobuf协议包仍为 `rpc`，服务名仍为 `rpc.RoleAuthService`；module改名只影响Go导入及链接注入路径，不改变RPC方法路径。现有 `option go_package = "./authRpc"` 继续对应authRpc子目录，代码生成使用go.mod声明的module。

实际数据库结构、只读连接结果及待讨论的代码问题见 [数据库与代码分析](CODE_REVIEW.md)。本轮分析未修改业务代码或数据库，未运行服务、构建或业务测试。

用户独立授权及角色模板的数据库设计见 [数据库设计与执行状态](DATABASE.md)。2026-11-24已按最新配置在user_rpc完成结构迁移：新增admin_permission，角色表改用menu_ids并删除is_super_role；管理员、菜单和分组表保持。AdminPermission模型、GetUserPermissionTree查询及SaveUserPermission覆盖保存已实现，见 [用户权限接入](USER_PERMISSION.md)。保存要求显式传入菜单IDs及版本号，支持清空和按版本恢复，单条写入不额外开启事务。旧角色业务仍待适配新结构后使用。

deleted_at统一通过模型软删除管理：未删除为NULL，模型Delete写入Unix秒并同步updated_at，普通查询自动过滤；读取已删除权限的版本号时才显式使用Unscoped。模型与相关删除入口已同步，本次未修改数据库或运行构建、测试。

用户权限代码集中维护：查询、保存、校验和菜单ID解析放在admin_permission_model.go；菜单树组装放在get_user_permission_tree_logic.go。保留每个RPC的Logic入口，同一业务的中间逻辑不再按小功能拆成多个辅助文件。

旧CheckAdminRoleMenu已删除，新CheckUserPermission(userId, serviceName, path)已接入协议、本地Client/Server及Logic，实际鉴权集中在admin_permission_model.go。仅放行当前业务正常管理员明确获得的未删除、已启用页面或按钮，服务名和完整路径同时精确匹配，无权限返回PermissionDenied，不开启事务或读取角色模板。menu新增service_name，菜单增改必填，列表及权限树同步回显。user-rpc仍引用外部客户端中的旧调用，需同步客户端并迁移，当前不能据此认定完整登录链路已联调。

go.mod当前仍声明 `github.com/K7mQ2vR9/base-pkg v0.1.0`，旧common-pkg和GitLab replace已移除。**当前RPC准入新接口及configcipher组件尚未发布，v0.1.0既不含该组件，也不支持当前RPC构造签名，仅在根工作区使用本地common-pkg时可接入；独立部署须先发布新公共包版本并更新go.mod/go.sum，不能直接以GOWORK=off使用当前源码。** 公共资源说明见 [base-pkg首版](https://github.com/K7mQ2vR9/base-pkg/tree/v0.1.0)。配置加解密统一使用公共包configcipher，本项目不再保留重复算法。

## RPC准入与显式白名单

用户身份由API层校验；RPC入口不根据UID缺失、空、0或合法正数决定放行，不验证JWT或服务凭证。原角色、权限、菜单及路径权限的业务Logic保持，不因入口调整删除业务权限判断。

| 情况 | GetWhitePath命中 | 未命中 |
|---|---|---|
| metadata缺失/空 | InvalidArgument | InvalidArgument |
| traceparent缺失、非法或冲突 | InvalidArgument | InvalidArgument |
| 已知字段重复、UID/IP格式非法 | InvalidArgument | InvalidArgument |
| 协议校验成功，business-code缺失或空字符串 | 放行 | Unauthenticated |
| 协议校验成功，business-code非空 | 放行 | 放行 |
| UID缺失、空、0或合法正数 | 不参与登录判断 | 不参与登录判断 |

唯一白名单配置入口是internal/middleware/white_header_path.go的GetWhitePath。启动直接使用NewRpcAuthMiddleware().ServerOption(middleware.GetWhitePath)，已删除internal/rpcauth的自动ServiceDesc全量名单及单独business-code回调包装。名单为空时不豁免任何方法，也不会自动添加本服务的方法。

当前GetWhitePath原配置保留，其中/commonRpc.CommunicationService/sendTgMsg不是本服务方法，不会命中auth-rpc。需要允许缺少business-code的本服务方法，由业务显式填写完整FullMethod，例如/rpc.RoleAuthService/getMenuList；本次没有自动加入此示例。

用户ID只作为业务上下文传递，结构校验不等于身份认证。调用方继续使用W3C traceparent/tracestate及中划线metadata，原下划线名称和旧Trace别名不兼容；白名单不能免除metadata或Trace。

## 配置密文与公共加解密组件

配置字段ConfigCipher直接使用公共包configcipher.Config。公共包提供New(Config)、Cipher.Encrypt和Cipher.Decrypt；加密使用范例及协议说明见 [配置加解密](../common-pkg/configcipher/README.md)。本项目仅在internal/svc/config_cipher.go解析密钥来源，加解密算法、密钥格式和认证校验统一由公共包处理。

保留旧协议：`v1:` 加无填充标准Base64编码的 `nonce | ciphertext | tag`。算法为AES-256-GCM，版本前缀 `v1:` 原样作为AAD参与认证。保持原32字节密钥时，原密文本身无需重新生成；这与公共包API响应AES-GCM的Base64URL协议不同，不能混用解密器。

生产模式 `Mode: pro` 对每项 `SelectDb[].MySQL.DataSource` 及已配置Redis的Password执行解密；dev/test按原样使用配置值。生产模式不尝试把解密失败的内容作为明文继续使用，缺失密钥或密文错误会返回启动错误，日志不回显配置值、密钥或部分明文。

密钥来源按优先级排列：

1. `ConfigCipher.KeyBase64`。
2. 环境变量 `MYSQL_CONFIG_KEY_BASE64`。
3. 本项目链接注入变量 `authRpc/internal/svc.encryptionKeyBase64`。

密钥值沿用旧密钥的无填充标准Base64，解码后须恰好32字节。若部署使用 `-ldflags -X` 注入，**统一使用authRpc/internal/svc.encryptionKeyBase64，并随module改名同步修改部署命令**；不保留旧路径别名。密钥由部署环境提供，不将实际值提交到源码或样例配置。公共包本身不读取MYSQL_CONFIG_KEY_BASE64，也不保存链接注入密钥。该环境变量虽以MYSQL命名，仍用于当前MySQL DSN及Redis Password的配置解密；部署时须同步改名，密钥值不变。

## 数据库、Redis与资源生命周期

配置直接使用公共包 `[]dbM.SelectDbConfig` 和 `*dbM.RedisConfig`。项目自定义的DatabaseConfig平铺包装及转换层已删除，不兼容 `SelectDb[].DataSource`；MySQL参数必须放在 `SelectDb[].MySQL` 子结构内。

```yaml
SelectDb:
  - DbKey: user-rpc
    MySQL:
      DataSource: ${AUTH_RPC_MYSQL_DSN}
      StartupTimeout: 5s
      LogLevel: warn
```

权限数据仍使用当前连接标识 `user-rpc`，没有更换实际数据库、表或数据。连接池、超时和日志的默认值由公共包提供；SQL日志各运行模式统一默认warn，删除dev/test自动info、pro自动error的覆盖。需要其他等级时显式配置MySQL.LogLevel，参数化日志默认开启。

Redis直接传入NewRedisClient，返回UniversalClient，支持node/cluster；RedisConf未配置时不创建客户端。TLS必须显式使用TLS字段控制，InsecureSkipVerify不再隐式开启TLS；TLS=false且InsecureSkipVerify=true时按公共包规则返回配置错误。Timeout、Pool、Retry等结构未配置时统一使用公共包默认值。

生产模式保留v1:密文解密。MySQL配置形状不再转换，只在同类型副本中解密MySQL.DataSource；Redis同样复制配置后解密Password，不覆盖原配置中的密文。解密后的新格式配置直接交给NewSelectDb/NewRedisClient，连接校验、探活及超时由公共包处理。

NewServiceContext接收启动ctx并返回组件与error，初始化失败回收已创建资源；成功后启动ctx取消不关闭共享连接池。模型构造正确接收GetDb双返回值，取库失败由Logic在业务执行前返回。RPC停止后ServiceContext.Close统一回收资源，不在请求中关闭共享客户端。

## 部署配置样例

无实际凭证的样例位于 [etc/auth_rpc.example.yaml](etc/auth_rpc.example.yaml)。程序加载配置时启用环境变量替换；生产DSN和Redis密码环境变量应提供原v1:密文，配置密钥也从部署环境注入。

etc/auth_rpc.yaml已同步为新结构：DbKey字段使用规范名称，原MySQL参数移入MySQL子节点，Redis显式配置Type=node。原连接地址、DSN、用户名、密码、密文及其他配置值保持，不生成替代凭证。部署配置也须按新结构迁移；生产模式继续接入既定密钥来源，原v1:密文本身无需重新生成。字段默认值及适用条件以公共包注释为准。

根工作区纳入auth-rpc和common-pkg，本地开发由工作区公共包提供实现。当前仅在根工作区使用新入口；待新版本发布并更新声明依赖后，独立项目才可配置私有仓库认证及GOPRIVATE，并使用GOWORK=off。未向go.mod添加本地replace，也未迁移其他业务服务。

## 本次迁移状态

截至2026-11-24，已完成公共包导入迁移、新格式MySQL/Redis接入、公共configcipher接入、模型取库错误返回、RPC metadata准入及GetWhitePath接入。用户权限阶段已新增AdminPermission模型及GetUserPermissionTree，只读返回当前业务全部菜单分组及用户勾选状态，协议和本地Client/Server已同步。旧角色及其他CRUD接口尚未重构，其他业务项目源码保持；原有测试仅适配接口、关闭入口和导入路径，未运行其真实资源操作。

Go module已由auth改为authRpc，并同步原有源码、生成Client/Server及原有测试中的Go导入路径；makefile的导出路径替换规则和密钥链接注入说明已同步。只调整导入，不重新生成protobuf或改动协议内容；根go.work继续按原目录引用，无须改动。此次执行gofmt格式化修改的Go文件，未执行构建、测试、代码生成或范例；无新增测试、脚本、缓存或临时产物，无需清理。

依赖维护在GOWORK=off、独立临时GOMODCACHE和当次私有仓库SSH映射下执行 `go mod download -json github.com/K7mQ2vR9/base-pkg@v0.1.0` 及 `go mod tidy`，均成功。下载版本对应提交 `7351c5b008eb63af63d89471091a0d744f39bed6`，校验和已写入go.sum；go.sum不再被本项目忽略。模块整理会解析依赖和既有测试导入，不会运行测试或连接业务数据库。

本轮仅格式化修改文件并维护依赖，按前述安排未执行构建、测试、范例或运行时联调；密文兼容和RPC行为依据旧版源码适配，不声称已做实际密钥/密文或调用方联调。没有新增测试用例或测试脚本。登记命令、结果和限制后，已删除本轮下载的依赖源码、模块元数据、zip及sumdb临时缓存，任务目录已不存在；原有模块缓存、测试和配置文件保留。

当前RPC新接口及configcipher组件仍只在本地工作区生效，未覆盖v0.1.0或发布新版本。配置加解密沿用v1:协议和既定密钥来源，尚无实际密钥/密文联调结论。用户权限保存已通过离线模拟SQL的定向测试和本地Client/Server编译，具体命令、范围及本次测试文件清理状态见 [用户权限接入](USER_PERMISSION.md)。该结果不替代旧角色逻辑、真实并发、API或前端联调；没有提交、推送或发版。
