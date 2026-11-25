# auth-rpc 权限服务接入说明

当前规则见[权限设计](PERMISSION_DESIGN.md)及[使用说明](USER_PERMISSION.md)。模块authRpc，protobuf服务rpc.RoleAuthService；继续使用user_rpc数据库和user-rpc连接标识，其他项目未改动。

后续 API、管理后台、账号及站点服务接入，先按[业务对接清单](INTEGRATION_CHECKLIST.md)核对职责、身份与模板、RPC 上下文、初始化顺序、页面流程、版本和错误处理，再安排联调验收。

模板配置新增独立GetTemplatePermissionTree，按类型及维护身份返回可选范围、当前勾选和模板版本，不依赖用户，也不初始化记录。默认模板1/2创建后不可删除；普通模板3保留删除入口。分组维护GetPermissionList完整回显未删除成员及status，禁用成员可以保留；授权树仍隐藏失效菜单。本次仅更新auth-rpc协议、本地客户端、服务端及业务源码，其他项目和外部rpc-client需同步新协议；不需要数据库迁移，未运行测试、构建或联调。

用户和模板配置树直接过滤超出当前默认模板上限的权限项及历史勾选，不返回禁用勾选；必要祖先仅补齐展示结构。显式保存当前勾选完整覆盖后才清除存储中的旧ID，查询不写库，保存仍校验当前上限及已读version。

GetMenuList的data.menuPid未传时不限制父级，显式0只查根菜单，正数只查直接子级，负数拒绝；列表回显实际父ID，包括0。MenuItem.menuPid已改为optional（编号2不变），Go调用方需同步*int64类型以显式传0；新增/修改省略时仍按根保存。列表ID、时间筛选本轮按要求跳过，未测试或构建，无数据库迁移。

GetRoleMenus与CopyRolePermissionToUser已统一按当前有效目录及两级默认上限过滤来源权限；复制完整覆盖剩余集合，全部失效时清空目标用户授权。保留来源/用户版本校验与同事务审计，不改写来源模板；必要默认模板缺失及数据/查询异常仍返回错误。协议字段不变，无数据库迁移，本轮未测试或构建。

## 权限模型

| 模板类型 | 规则 | 维护资格 |
|---|---|---|
| 1平台默认 | 全平台一份真实CSV权限集合，供平台11使用，不等于10自身默认全权限 | 平台超级管理员10 |
| 2站点默认 | 每站点一份，集合不能超过模板1 | 平台普通管理员11 |
| 3本站普通 | 同站点可多份，不超过两级默认交集 | 本站唯一超级管理员20，含创建、查询、修改、删除和复制 |

account_type20已定义为站点超级管理员，每站点有且只有一位，30/40不能操作模板。11在模板1范围内逐人勾选并保存独立权限；20默认拥有本站模板2与模板1交集的全部有效操作权限，不依赖个人CSV授予；10自身默认全操作权限不由模板授予。操作者身份决定资格及审计，不作为可下发权限集合的上限。

配置页面按相应默认模板提供可选目录，过滤删除、禁用、无效祖先下级及上限外权限。手工保存再次过滤失效ID，再整体检查有效项是否越权；模板选择和复制只取当前有效目录与两级上限的交集，复制仍校验来源及用户双版本，全部失效时明确清空用户授权。模板、授权写入均同事务成功审计，审计失败回滚，不级联裁剪下级历史CSV。

操作权限与已配置菜单树分开：**GetMenuByAdminId对所有账号（包括10/20）都按已配置menuIds获取菜单树**，无配置为空，不自动返回全量。实际鉴权中10默认全操作，20默认本站有效上限，11/30/40仍按个人授权及对应上限计算。

菜单与分组的新增、修改、删除仅允许平台归属99999999、正常且未删除的真实账号10；模型在同一事务内普通读取真实身份并按目标旧version写入，写入固定或限定平台归属，11/20/30/40不得因接口权限而绕过身份限制。已补齐关键字段可改且保留授权、单次父级防环、合法零值、分组JSON校验与失效过滤、统一删除规则；10可通过权限配置维护自身菜单展示。详见[目录维护变更记录](USER_PERMISSION.md#2026-11-25-菜单与分组维护资格收口)。源码开发阶段未测试或构建；随后007/008已执行并完成结构核对，路径唯一约束及版本列已落库。

所有身份、上限、复制来源与目录检查改为普通读取，不增加全局版本互斥或显式行锁；version保护同一记录的并发覆盖，不承诺跨记录串行。单次父级防环保留，不处理不同菜单同时改父级形成环；并发收权后实际访问仍按当前有效上限及目录校验。各次普通读取受数据库隔离级别控制，不承诺多次查询为同一快照。

## 迁移与验证状态

001—008已在user_rpc执行，实际记录及快照见[DATABASE.md](DATABASE.md)。**2026-11-25已完成005/006并核对：数据库采用1平台默认、2站点默认、3本站普通，平台/站点模板及本站20唯一约束已生效，禁止在当前库重跑。** 六张相关表仍为空，未自动创建模板、账号或授权；仅完成结构迁移，未运行Go测试、构建或业务联调。

**[008_optimistic_catalog_version.sql](database/migrations/008_optimistic_catalog_version.sql)已于2026-11-25在user_rpc执行并核对，当前库禁止重跑。** menu/menu_group版本列及1..MaxInt64的启用CHECK已落地，permission_catalog_lock已删除，007路径生成列及唯一约束保留；六张权限业务表仍为空，未删除其他共享业务表。源码已移除全部显式悲观锁，调用方修改/删除菜单、分组必须显式回传已读版本，冲突Aborted，不自动重试。实际记录和前后快照见DATABASE.md，本轮未运行服务或业务测试。

006唯一索引保证最多一位20，至少一位仍需建站/账号初始化流程落实；auth-rpc没有该账号创建/删除入口，本轮不自动创建。账号创建及至少一位20由其他业务服务负责；10自身菜单配置已复用原查询/保存接口。20替换、删除或恢复流程及其他项目旧角色/JWT/调用迁移仍需后续处理。

源码开发阶段仅执行协议生成、格式化与引用核对，未测试或构建；之后按用户授权完成007迁移和结构核对，临时迁移执行器及凭证按DATABASE.md登记清理，原有测试保留。其他项目和外部rpc-client尚未同步，源码更新不代表联调完成。下面资源与部署说明保留，当前权限行为以上述设计和接口说明为准。

deleted_at统一通过模型软删除管理：未删除为NULL，模型Delete写入Unix秒并同步updated_at，普通查询自动过滤；读取已删除权限的版本号时才显式使用Unscoped。模型与相关删除入口已同步，本次未修改数据库或运行构建、测试。

用户权限代码集中维护：查询、保存、校验和菜单ID解析放在admin_permission_model.go；授权编辑树组装放在get_user_permission_tree_logic.go，登录菜单组装放在get_menu_by_admin_id_logic.go。两个入口共用的父级循环检查集中在menu_model.go，不新建辅助文件。保留每个RPC的Logic入口。

实际操作权限与菜单配置分开：10默认全部操作，20默认本站上限全部有效操作；GetMenuByAdminId对10/20也按已配置menuIds获取，不自动返回全量。具体接口见[使用说明](USER_PERMISSION.md)。

旧CheckAdminRoleMenu已删除，新CheckUserPermission(userId, serviceName, path)已接入协议、本地Client/Server及Logic，实际鉴权集中在admin_permission_model.go。正常平台10默认全操作；本站20使用两级默认上限全部有效权限；11/30/40按个人授权及对应默认上限计算，精确匹配serviceName/path并校验目录及祖先有效性。无权限返回PermissionDenied，普通查询不加读事务，不继承普通模板或建立角色绑定。menu的service_name新增/修改必填，列表及权限树同步回显。其他业务仍需同步外部客户端并迁移旧调用，完整登录及鉴权链路须另行联调。

go.mod当前仍声明 `github.com/K7mQ2vR9/base-pkg v0.1.0`，旧common-pkg和GitLab replace已移除。**当前RPC准入新接口及configcipher组件尚未发布，v0.1.0既不含该组件，也不支持当前RPC构造签名，仅在根工作区使用本地common-pkg时可接入；独立部署须先发布新公共包版本并更新go.mod/go.sum，不能直接以GOWORK=off使用当前源码。** 公共资源说明见 [base-pkg首版](https://github.com/K7mQ2vR9/base-pkg/tree/v0.1.0)。配置加解密统一使用公共包configcipher，本项目不再保留重复算法。

## RPC准入与显式白名单

用户身份由API层校验；RPC入口不根据UID缺失、空、0或合法正数决定放行，不验证JWT或服务凭证。入口规则与各模板、权限、菜单业务判断分开，模板用途和站点边界仍由业务模型校验。

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
