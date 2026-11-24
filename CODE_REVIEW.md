# auth-rpc 数据库与代码分析

分析日期：2026-11-24。依据当前工作区源码和项目配置完成只读数据库连接、表结构与索引检查、汇总查询。本文用于讨论后续修改，不代表已经修复。未修改业务源码、数据库结构或数据。

## 数据库连接与当前数据

已使用 `etc/auth_rpc.yaml` 的MySQL连接配置成功连接，当前项目运行模式为dev，DSN按明文连接，不需要配置解密密钥。连接凭证、地址、数据库名和账户明细不写入本报告。

服务端返回版本为MySQL 9.7.1。四张核心表均为InnoDB，表级排序规则为utf8mb4_unicode_ci。数据库中还存在其他业务表，本轮只分析权限服务实际引用的四张表。

| 核心表 | 职责 | 精确记录数 | deleted_at为NULL/0/正数 |
|---|---|---|---|
| admin_info | 管理员及关联角色 | 0 | 0 / 0 / 0 |
| role_permission | 角色及权限组ID数组 | 0 | 0 / 0 / 0 |
| permission_menu | 权限组及菜单ID数组 | 0 | 0 / 0 / 0 |
| menu | 页面、按钮及路径 | 0 | 0 / 0 / 0 |

`permission_menu.id=1` 的记录数为0，四张表未定义外键。空表意味着目前不能用已有数据复现跨业务访问、禁用菜单或关联删除场景；以下相关结论来自源码及实际表结构，并未通过插入样本验证。

关键结构：

- 四张表的created_at、updated_at、deleted_at均为可空BIGINT，默认NULL。
- permission_menu.menu_ids为可空JSON；role_permission.permission_ids为非空JSON。
- admin_info、role_permission、menu主键为BIGINT UNSIGNED，permission_menu主键为有符号BIGINT；当前Go模型和protobuf均使用int64。
- menu.status和menu_type默认0，业务常量分别从1开始；admin_info.status默认1。
- role_permission.is_super_role默认2，代码定义1为超级管理员、2为普通管理员、3为代理管理员。

实际索引：

| 表 | 主键之外的索引 | 说明 |
|---|---|---|
| admin_info | uniq_account(account, deleted_at) | 唯一索引；deleted_at可空时不能依赖它限制所有未删除账户重复 |
| role_permission | idx_business(business_code) | 普通索引，没有超级角色的唯一约束 |
| permission_menu | 无 | 仅主键 |
| menu | uniq(path) | 实际NON_UNIQUE=1，是普通索引，名称不能代表唯一约束 |

## 业务结构与已有基础

protobuf服务为rpc.RoleAuthService，共17个方法：权限组管理4个、角色管理6个、路径权限检查1个、菜单管理及管理员菜单查询5个、超级角色初始化1个。Go module为authRpc，协议服务名不随Go module改名。

主要关系为：管理员role_id → 角色permission_ids(JSON数组) → 权限组menu_ids(JSON数组) → 菜单/按钮path。权限查询和菜单树都依赖这条关系的完整性。

启动时使用公共包创建共享MySQL连接池和可选Redis客户端；模型通过GetDb(ctx, key)使用请求上下文，资源在服务退出时统一关闭。生产模式在配置副本中通过configcipher解密后连接；本次连接的是dev配置，不能据此认定生产密文流程已联调。

RPC入口按既定规则要求metadata、合法Trace及非白名单business-code，不按用户ID校验登录。当前GetWhitePath为空，没有方法豁免business-code。API层用户认证与RPC数据范围校验是不同职责，以下建议不要求恢复RPC用户ID认证。

当前Redis只在ServiceContext创建和关闭，未发现17个业务方法实际调用；GetAllRoleByCache也只是数据库查询，不能按函数名称认定已经实现缓存。

## 待讨论的修改清单

### 1. 统一软删除语义：NULL与0不匹配（高）

依据：[菜单模型字段](internal/dao/schema/menu.go:15)、[角色模型字段](internal/dao/schema/role_permission.go:11)及实际四张表定义。当前schema使用soft_delete.DeletedAt，gorm标签未配置default:null；本地依赖gorm.io/plugin/soft_delete v1.2.1默认追加deleted_at=0，只有字段默认值标记为null时才生成IS NULL条件。

影响：按数据库默认值或项目约定写入deleted_at=NULL的正常记录，普通查询及更新会被当作不可见。当前表为空，没有已存在的漏查记录；代码创建的记录可能写入0，后续还会形成两种约定混用。

建议：以项目数据库约定的“NULL表示未删除、时间戳表示已删除”为依据，统一模型映射和各写入方；实施前统计历史0值及其他业务服务的写法，不直接把NULL批量改成0。本次未执行任何迁移。

### 2. 角色和权限组按ID访问缺少一致的业务范围限制（高）

依据：[RoleModel.FindOneById](internal/dao/model/mysql/role_permission_model.go:65)、[RoleModel.UpdateByMap](internal/dao/model/mysql/role_permission_model.go:90)、[PermissionMenuModel.UpdateByMap](internal/dao/model/mysql/permission_menu_model.go:85)、[GetMenuIds](internal/dao/model/mysql/permission_menu_model.go:125)、[WhereBusinessCode](internal/dao/model/base_model.go:36)。

部分列表使用business-code过滤，但按ID查询、更新和删除仅限制id。请求体BusinessCode在部分入口又优先于上下文；AddRole直接采用请求体BusinessCode。DeleteRole先按业务范围查询，却不要求查询到记录，随后可以用不带业务条件的ID执行更新。

影响：业务A的调用携带业务B的记录ID时，模型未统一阻止跨范围读写。metadata字段存在不等于记录归属检查通过。

建议：先明确平台管理、普通业务、层级业务的可访问范围，并在查询及修改的WHERE中统一落实。保留经确认的公共权限组id=1例外。菜单已有business_code字段，但当前多处按全局菜单处理，需先决定菜单是平台共享还是业务独立，不能直接给所有菜单查询加过滤。此项不增加RPC用户ID认证。

### 3. JSON权限关联错误使用FIND_IN_SET（高，SQL已核对）

依据：[FindOneByMenuId](internal/dao/model/mysql/permission_menu_model.go:59)、[PermissionMenuModel.FindOne](internal/dao/model/mysql/permission_menu_model.go:38)、[RoleModel.FindOne](internal/dao/model/mysql/role_permission_model.go:41)、[DeleteMenu](internal/logic/roleauthservice/delete_menu_logic.go:57)。

真实字段为JSON数组，FIND_IN_SET用于逗号字符串成员查询。已在只读连接执行以下常量表达式：

```sql
SELECT FIND_IN_SET('1', JSON_ARRAY(1,2)),
       FIND_IN_SET('2', JSON_ARRAY(1,2)),
       JSON_CONTAINS(JSON_ARRAY(1,2), '1'),
       JSON_CONTAINS(JSON_ARRAY(1,2), '2');
```

返回依次为0、0、1、1。现有菜单删除前的关联检查因此可能漏掉仍被引用的菜单。

建议：统一JSON成员查询，并校验数组元素为合法ID、去重及关联对象归属。检查引用和删除之间还需定义并发一致性策略；只替换SQL函数不能消除并发绑定造成的竞态。

### 4. 查询、解析和更新错误没有完整返回（高）

依据：[RoleModel.FindOne](internal/dao/model/mysql/role_permission_model.go:57)、[AdminInfoModel.FindOne](internal/dao/model/mysql/admin_info_model.go:47)、[PermissionMenuModel.FindOneByMenuId](internal/dao/model/mysql/permission_menu_model.go:64)、[GetRoleMenus](internal/logic/roleauthservice/get_role_menus_logic.go:71)、[GetRolePermissionById](internal/dao/model/mysql/role_permission_model.go:140)。

多个FindOne函数遇到非ErrRecordNotFound错误也进入空分支，仅返回零值结构。GetRoleMenus取得GetMenuIds的err后仍返回成功；多处json.Unmarshal错误被忽略。更新只返回Error，不判断RowsAffected。

影响：断连、取消或SQL错误可能被误判为“没有引用”“记录不存在”或空列表，检查后继续删除；修改不存在的记录也可能返回成功。JSON虽然由数据库校验语法，但不保证一定能解析为[]int64。

建议：查询返回对象和error，明确区分未找到与数据库故障；解析失败直接返回；修改时区分记录不存在、范围不允许与值未变化，不能把RowsAffected=0一律等同于错误。

### 5. 禁用菜单仍可能通过路径权限校验（高）

依据：[CheckAdminRoleMenu](internal/logic/roleauthservice/check_admin_role_menu_logic.go:80)、[GetMenuByIds](internal/dao/model/mysql/menu_model.go:156)、[GetMenuByAdminId](internal/logic/roleauthservice/get_menu_by_admin_id_logic.go:54)。

路径检查只按path和menu_type=按钮查菜单，没有要求status=启用。管理员菜单树按ID取菜单同样未过滤状态；管理员菜单查询没有选择有效管理员状态。

影响：只要角色权限数组仍包含某菜单，禁用该菜单不一定撤销路径访问，菜单也可能继续显示。当前为空表，未通过实际角色调用验证。

建议：明确菜单及上级菜单禁用后的生效规则，统一权限检查和菜单展示的数据状态条件。管理员状态属于对应数据使用规则，不作为RPC中间件的用户身份认证条件。

### 6. 请求参数、ID和分页缺少边界检查（高）

依据：[GetRoleList](internal/logic/roleauthservice/get_role_list_logic.go:63)、[AddRole](internal/logic/roleauthservice/add_role_logic.go:48)、[UpdateMenu](internal/logic/roleauthservice/update_menu_logic.go:47)、[AdminInfoModel.FindOne](internal/dao/model/mysql/admin_info_model.go:35)、[PageQuery](internal/dao/model/base_model.go:16)。

GetRoleList在判空前访问in.Data，并直接截取business-code前5字节；多处新增/更新直接解引用in.Data。分页未设置默认值、最大条数或乘法溢出边界。AdminId=0时，FindOne不追加id条件，非空库中会选择首条符合默认条件的管理员记录。

建议：各RPC入口校验必需Data、正数ID、枚举、字符串长度和JSON数组；缺少ID不能退化为首条查询；分页约定默认值及上限。业务编码层级转换需要单独明确，避免在列表逻辑中无检查切片或原地改写请求。

### 7. 更新不能可靠表达零值、清空和顶级菜单（中）

依据：[MenuModel.UpdateAll](internal/dao/model/mysql/menu_model.go:83)、[UpdateMenu](internal/logic/roleauthservice/update_menu_logic.go:65)、[RoleModel.UpdateByMap](internal/dao/model/mysql/role_permission_model.go:90)。

Updates(struct)默认跳过零值字段，无法可靠把menu_pid从非零改回0或把sort改成0。手写map又只加入非空字符串，无法表达清空角色名等操作。protobuf当前标量没有字段存在性，无法区分“没传”和“明确传0”。

建议：先确定接口是全量更新还是部分更新，再决定显式字段map、optional字段或FieldMask等方式；不能单纯对所有字段Select(*)，避免覆盖ID、归属和创建时间。

### 8. 超级角色创建、修改和初始化缺少统一约束（高）

依据：[AddRole](internal/logic/roleauthservice/add_role_logic.go:49)、[UpdateRole](internal/logic/roleauthservice/update_role_logic.go:47)、[CreatedSuperRole](internal/dao/model/mysql/role_permission_model.go:150)、实际role_permission索引。

通用新增/修改可以带入is_super_role，没有统一的业务保护；初始化先查后插，已有事务只包住插入，没有覆盖检查阶段，也没有对应唯一约束。并发请求可能创建多个同一业务的超级角色。

建议：明确允许管理超级角色的内部业务入口，限制通用CRUD修改超级角色标记；使用数据库约束或锁实现初始化幂等。普通角色允许多个，不能直接为(business_code, is_super_role)建立覆盖全表的唯一索引。

### 9. 菜单路径唯一性只有应用预检查（中）

依据：[AddMenu](internal/logic/roleauthservice/add_menu_logic.go:57)、[UpdateMenu](internal/logic/roleauthservice/update_menu_logic.go:56)、实际menu.uniq索引。

先查路径是否存在，再创建或更新，缺少数据库唯一约束。并发请求能同时通过预检查；同路径多条记录还会让权限检查的First结果不确定。

建议：菜单范围确定后，再设计包含平台/业务范围、菜单类型、软删除语义的有效记录唯一性，并处理重复键错误。admin_info的(account, deleted_at)唯一索引也应按NULL软删除语义复核。本轮没有现存重复数据。

### 10. 菜单树缺少循环、父节点完整性和排序规则（中）

依据：[UpdateMenu](internal/logic/roleauthservice/update_menu_logic.go:47)、[AdminRoleModel.buildMenuTree](internal/dao/model/mysql/admin_role.go:82)。

更新只检查父节点存在，未阻止自身作为父节点或形成祖先循环；删除没有检查下级菜单。树组装只保留menu_pid=0或父节点在当前结果集中的节点，权限只配置子节点时可能出现平面菜单非空、最终树为空。展示未按sort进行稳定排序。

建议：确认授权子节点是否自动补齐祖先及祖先是否只用于展示，阻止循环，定义删除父节点策略，并统一sort和ID排序。祖先展示不应自动授予按钮权限。

### 11. 超级角色初始化所需基础数据尚不存在（接入阻塞）

依据：[SitePermissionId](internal/dao/schema/permission_menu.go:19)、[CreatedSuperRole](internal/dao/model/mysql/role_permission_model.go:158)及实际id=1计数。

当前没有默认权限组id=1，CreatedSuperRole会在插入角色之前返回init permission menu not found。未确认基础菜单、默认权限组所属范围和初始化入口前，不应自行插入占位数据。

建议：讨论可重复执行、可追踪的初始数据方案及调用顺序，并与超级角色幂等约束一起落实。本次没有调用初始化接口或写入任何记录。

### 12. 独立部署仍依赖未发布的公共组件（接入阻塞）

依据：[go.mod](go.mod:8)、[公共包接入状态](../common-pkg/README.md)、[ServiceContext](internal/svc/service_context.go:45)。

go.mod仍依赖base-pkg v0.1.0，该已发布版本不含新增configcipher，也不支持当前RPC准入构造签名。根工作区使用本地common-pkg可提供源码，但不能据此认定独立模块已具备同样依赖。

建议：在独立部署前发布包含已确认公共改动的新版本，并同步依赖及校验和。本轮未构建服务，未发布、推送或调整依赖版本。

### 13. 角色详情没有组装协议要求的权限明细（中）

依据：[GetOneRole](internal/logic/roleauthservice/get_one_role_logic.go:44)、[RoleListItem协议](dsl/auth_rpc.proto:107)、[角色数据库模型](internal/dao/schema/role_permission.go:8)。

数据库模型PermissionIds是JSON字符串，详情返回的RoleListItem.PermissionIds却是权限组对象切片。GetOneRole只调用copier.Copy，没有解析ID数组或查询权限组名称；本地copier v0.4.0对这种不支持的字符串到结构切片转换会跳过，不能自动完成业务关系展开。列表入口另有手工组装，详情入口没有，因此配置过权限的角色详情仍缺少权限明细。角色不存在时还返回空Data成功，属于第4项的错误语义问题。

建议：明确角色详情是返回可编辑ID数组还是权限组明细；如果继续沿用现有协议，应复用显式DTO组装逻辑，完整返回ID和名称，并处理不存在/已删除关联，避免编辑页面将遗漏项误当作未授权。此结论为源码分析，未运行copier范例或RPC测试。

### 14. 预期业务错误被映射为系统内部错误（高）

依据：[路径权限拒绝](internal/logic/roleauthservice/check_admin_role_menu_logic.go:61)、[角色删除保护](internal/logic/roleauthservice/delete_role_logic.go:58)、[RPC错误转换](../common-pkg/result/xcode/grpc.go:22)。

业务Logic大量直接返回errors.New，例如没有访问权限、超级角色不能删除、父菜单不存在。当前公共ToGRPCStatus对不带业务码或gRPC状态的普通错误统一转换为Internal和固定Internal Server Error。因此调用方无法按PermissionDenied、NotFound或业务冲突区分这些预期结果，错误文案也不会按原字符串透传。

建议：由auth-rpc定义业务错误并使用公共XCode或明确的gRPC状态返回；数据库及内部异常继续脱敏。该项调整业务错误的分类方式，不要求公共包放开内部错误文案，也不修改既定RPC用户ID准入规则。

### 15. 协议提供的部分列表筛选字段实际不生效（中）

依据：[权限列表请求](dsl/auth_rpc.proto:36)、[角色列表请求](dsl/auth_rpc.proto:94)、[菜单列表请求](dsl/auth_rpc.proto:163)、[权限组列表查询](internal/dao/model/mysql/permission_menu_model.go:104)。

三种列表请求均带STime/ETime，但相应Logic和模型未应用时间范围。权限组列表把Data复制给模型后，GetList只处理BusinessCode，没有按ID或PermissionName过滤。角色列表的PermissionIds条件也未实现。传入这些字段不会得到协议表面上暗示的筛选结果。

建议：先明确各字段是否用于查询；保留的筛选字段实现一致的查询条件及时间边界，不需要的字段通过明确的协议变更处理，不静默接受后忽略。若分页查询需要稳定结果，角色和权限组列表也应有明确且稳定的排序。

### 16. 当前Docker启动配置与项目文件不一致（接入阻塞）

依据：[Dockerfile启动命令](Dockerfile:29)、[程序默认配置](auth_rpc.go:25)及当前etc目录文件。

Dockerfile最终指定etc/app.yaml，项目现有配置却是etc/auth_rpc.yaml，程序默认路径也是后者。在没有部署额外挂载app.yaml的情况下，按该镜像默认命令启动会加载配置失败。

同时构建步骤仍依赖COPY .netrc，而当前检出的项目没有该文件；GOPRIVATE只设置旧GitLab域名，新公共包位于私有GitHub仓库。即使第12项完成发版，也仍需要明确镜像构建的凭证来源及新模块访问设置。此处未查看或复制任何凭证文件，未构建镜像。

建议：统一镜像默认配置路径，并按实际部署方式明确外部配置和构建依赖访问。本次只记录现状，不生成凭证、修改Dockerfile或尝试构建。

## 数据规范需要明确的事项

项目“数据库相关约定.md”规定主键使用bigint→int64、时间使用时间戳、deleted_at使用NULL或删除时间戳、动态内容使用JSON、默认状态0且枚举从1开始。本轮已发现的软删除不一致应优先处理。

其余需要在后续模型/数据库设计时统一：BIGINT UNSIGNED超出MaxInt64的范围不能由当前int64协议表示；部分状态列默认值与文档不一致；created_at/updated_at为可空BIGINT但模型使用int64。这里只登记设计差异，不直接改表、改协议或推定所有可空字段都已有脏数据。

建议处理顺序：先讨论并统一软删除语义与业务数据范围，再处理JSON关联及错误返回/业务错误分类、状态与参数边界；随后完善更新语义、超级角色幂等、索引、菜单树、详情和列表筛选；最后接入初始数据并解决独立部署版本及镜像配置。具体改动按平台规则先讨论方案、确认后实施。

## 实际执行、限制与临时文件清理

实际只读命令（连接参数从项目配置读入，不在命令行传密码）：

```text
mysql --defaults-file=<0600临时凭证文件> --no-login-paths --protocol=TCP --connect-timeout=5 --batch --raw --skip-column-names
```

- 首次在受限网络内连接失败，MySQL错误码2003；经受限网络外执行，元数据查询成功。
- 查询采用READ ONLY事务，设置SESSION MAX_EXECUTION_TIME=5000，进程预算25秒；完成后ROLLBACK。此设置约束本次查询，不修改服务配置或业务数据。
- 元数据查询包括SELECT VERSION()及information_schema的TABLES、COLUMNS、STATISTICS和TABLE_CONSTRAINTS。
- 统计查询对上述四表执行精确COUNT、NULL/0/正数deleted_at分布，对permission_menu执行WHERE id=1的计数；均为0。JSON常量表达式结果为0、0、1、1。服务器系统时区返回+07，未因此调整服务或数据库时区。
- 只查询元数据、计数和常量表达式，未读取管理员账号、密码、令牌、密钥或个人资料记录。
- 每次连接结果和限制均先登记，再删除对应临时凭证文件；三次连接使用的临时文件均已删除，清理后按任务前缀核对剩余文件数为0。本文将分散记录归并为当前分析结果。
- 未新增测试用例、测试脚本、依赖缓存、数据库导出文件或业务验证样本。原有测试及用户文件保留。
- 未启动RPC服务，未运行go test、go build、原有真实资源测试或数据初始化；数据库只读连接成功不代表RPC链路、生产密文解密、并发或业务权限流程已经验证。
