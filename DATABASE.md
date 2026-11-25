# auth-rpc 数据库设计

2026-11-24已在user_rpc执行迁移001、002；2026-11-25已执行迁移003并核对新列、根模板唯一索引及三个启用的CHECK约束。当前角色表结构已对齐本地模板模型、协议及CRUD，仍为0条记录，未创建根模板或授权数据。其他项目及前端未改动。

## 当前表结构与职责

| 表 | 当前状态 |
|---|---|
| admin_permission | 已新增；一个用户一条独立权限记录，user_id唯一，menu_ids为英文逗号分隔的具体菜单ID |
| role_permission | 迁移001、003均已执行；template_type、version、根模板生成列、唯一索引及CHECK约束已存在，当前0条记录 |
| menu_group | 本次实际数据库与模型均使用此表名，成员为JSON格式，仅用于菜单展示；旧快照中的permission_menu不是本轮修改对象 |
| menu | 在原页面、按钮等字段基础上新增必填service_name及业务/服务/路径普通查询索引，其他原字段保持 |
| admin_info | auth-rpc已补齐AccountType映射及五种类型常量，并移除RoleId映射和按role_id筛选；数据库role_id列本轮未删除，其他项目引用未改动 |

admin_info.account_type沿用：10=平台超级管理员、11=平台管理员、20=商户超级管理员、30=站点管理员、40=代理管理员。auth-rpc的schema.AdminInfo已映射AccountType，并定义与user-rpc数值一致的AdminInfoAccountType10/11/20/30/40常量及中文注释；0表示未设置，不属于这五种有效类型。不增加新的身份字段，不压缩为三种类型，不自动转换现有账号类型；字段映射本身不新增身份鉴权或自动授予权限。账号身份与角色权限模板分别管理。

源码阶段已完成账号类型映射、用户角色解绑、独立授权登录菜单及模板基础CRUD。用户确认后，本轮在user_rpc仅执行迁移003，未插入、更新或删除业务记录，未删除admin_info.role_id，未修改其他项目。未运行Go测试、构建、服务、RPC或业务联调。平台根模板管理、操作者自身上限及审计仍待实施，结构迁移成功不代表完整三级权限业务可用。

### admin_permission

| 字段 | 数据库类型 | 默认值及用途 |
|---|---|---|
| id | BIGINT UNSIGNED | 自增主键 |
| user_id | BIGINT UNSIGNED NOT NULL | 对应admin_info.id，唯一，无默认值 |
| menu_ids | TEXT NOT NULL | 无默认值，写入时显式提供；例如1,2,5，空字符串表示无权限 |
| business_code | VARCHAR(10) NOT NULL | 默认空字符串，业务写入时明确归属 |
| version | BIGINT UNSIGNED NOT NULL | 默认1，用于业务层实现并发覆盖保护 |
| created_at | BIGINT NOT NULL | 默认0，应用写入创建时间，单位Unix秒 |
| updated_at | BIGINT NOT NULL | 默认0，应用写入修改时间，单位Unix秒 |
| deleted_at | BIGINT NULL | 默认NULL表示未删除；删除后为Unix秒时间戳 |

索引为PRIMARY(id)、UNIQUE uniq_user_id(user_id)、普通索引idx_business_code(business_code)。使用InnoDB、utf8mb4、utf8mb4_unicode_ci，字段和表用途均有中文注释。

user_id与现有admin_info.id类型一致；应用使用int64时仍须校验ID和版本的有效范围。同一user_id只有一条记录，唯一键不包含deleted_at；软删除后重新授权应恢复该记录，不能新增多份。

menu_ids由SaveUserPermission在写入前校验、去重、排序及检查存储字节上限；数据库TEXT字段本身不保证格式或菜单存在。version由保存接口进行条件比较和递增，数据库不会无条件自动递增。business_code字段不替代业务范围校验，当前查询和保存均限定请求business-code，未开放跨业务授权。

### 模型软删除约定

auth-rpc现有六个schema的DeletedAt字段统一使用：

```go
DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;default:null;softDelete:DeletedAtField:UpdatedAt"`
```

数据库未删除状态为NULL，删除后为Unix秒时间戳；Go零值仅表示模型中的未删除状态。普通Model查询自动附加软删除作用域，DAO不再手写deleted_at过滤条件。

菜单、分组和模板删除统一调用模型Delete，由插件同时维护DeletedAt和UpdatedAt，不在Logic生成删除时间或用普通Updates更新deleted_at。普通更新不写软删除列，避免Go零值写成数据库0。模板删除在短事务内锁定同站点记录、检查自定义类型和版本，递增version后再调用模型Delete，任一步失败都回滚；不再检查用户绑定或旧超级角色字段。

需要读取已删除用户权限的版本时，显式使用Unscoped；该查询只用于后续恢复时的版本比较，不授予已删除权限。此约定未修改数据库结构、已有数据或其他项目源码，共用表的其他服务后续也需采用相同NULL约定。

### role_permission

迁移003后实际字段为id、role_name、menu_ids、template_type、version、created_at、updated_at、deleted_at、business_code、root_business_code。当前库结构已对齐模板模型；其他环境须先核对迁移状态。

menu_ids为TEXT NOT NULL，写入时显式提供英文逗号分隔的具体菜单ID，空字符串表示空模板。旧permission_ids的JSON权限组数组已退出结构；is_super_role已删除。角色变更不应自动同步到用户，模板覆盖勾选项也应在用户点击保存后才写入用户权限。

迁移003已新增并核对：

| 字段或约束 | 目标定义 |
|---|---|
| template_type | TINYINT UNSIGNED NOT NULL，无默认类型；1根模板、2自定义模板，CHECK限制有效值 |
| version | BIGINT UNSIGNED NOT NULL DEFAULT 1，CHECK范围1..MaxInt64，业务更新及软删除时比较递增 |
| root_business_code | VARCHAR(10)存储生成列；根模板取business_code，自定义模板取NULL，不由应用写入 |
| uniq_site_root | 生成列唯一索引，每站点最多一条根模板，普通自定义模板可多条，软删除不释放根槽位 |
| chk_role_template_business | 要求business_code非NULL、非空白，避免空归属绕过根模板唯一性 |

保留现有主键、业务索引和其他字段。根模板由未来平台专用入口初始化，普通CRUD不能创建、修改或删除根模板；本轮SQL不插入占位数据。普通自定义模板写入锁定当前站点根模板检查子集，再检查具体菜单存在、启用且为页面或按钮，不继承菜单分组。完整接口和失败语义见 [权限模板接口](USER_PERMISSION.md#权限模板接口)。

本次执行前已核对001结构、空表、新列及约束均不存在，满足003前提。其他环境若已有模板，须先明确类型、归属及版本，另定数据迁移，不自动分类或清空，不无条件重复执行。

### 迁移003执行状态

2026-11-25按auth-rpc当前配置连接user_rpc，MySQL版本9.7.1。执行前已保存role_permission结构快照，不含记录、连接凭证或密钥。

| 操作 | 实际结果 |
|---|---|
| 前置核对 | role_permission含软删除记录的精确COUNT为0；003目标列、索引及约束不存在 |
| 执行003 | 成功，仅ALTER TABLE role_permission，不重跑001、002 |
| 新列 | template_type为非空TINYINT UNSIGNED且无默认值；version为非空BIGINT UNSIGNED默认1；root_business_code为VARCHAR(10) STORED GENERATED |
| 根唯一索引 | uniq_site_root存在、NON_UNIQUE=0，索引root_business_code |
| CHECK | chk_role_template_type、chk_role_template_business、chk_role_template_version均存在且ENFORCED=YES |
| 迁移后数据 | role_permission仍为0条，没有插入模板、用户或授权数据 |
| 非目标表 | admin_info、admin_permission、menu、menu_group结构摘要（不含AUTO_INCREMENT计数）及精确COUNT未变化，记录数均为0 |

实际执行器依次运行，临时路径以占位符表示：

```text
python3 <临时目录>/migrate.py inspect
python3 <临时目录>/migrate.py apply
python3 <临时目录>/migrate.py verify
```

三步均成功返回。执行器通过0600临时凭证文件调用mysql，密码不出现在命令参数或输出；SQL经标准输入提交，连接超时8秒，DDL会话元数据锁及行锁等待各10秒，DDL客户端预算45秒。首次沙箱连接返回2003，使用获准网络执行权限后连接、迁移和核对成功；未修改配置或更换目标库。

本次核对仅使用SHOW CREATE TABLE、information_schema和精确COUNT，未插入约束样本，未运行Go测试、构建、RPC、服务或业务联调。结构结果不代替模板业务、真实并发或完整权限验证。SQL和结构快照保留为交付资料。结果登记后，已删除本次临时目录及其执行器、0600连接凭证、中间状态和错误摘要，并核对目录不存在；未新增Go测试、验证缓存或授权样本，原有测试保留。

## 迁移文件与结构快照

- 执行文件：[001_admin_permission_and_role_template.sql](database/migrations/001_admin_permission_and_role_template.sql)。
- 迁移前快照：[20261124_before_001.sql](database/snapshots/20261124_before_001.sql)。快照只含结构，不含账户记录、连接凭证或其他用户数据，也不是可直接执行的回滚脚本。

菜单服务字段另由 [002_menu_service_name.sql](database/migrations/002_menu_service_name.sql) 完成；对应迁移前结构为 [20261124_before_002.sql](database/snapshots/20261124_before_002.sql)。迁移前menu精确COUNT为0，因此不涉及历史服务归属回填，执行后仍为0。

已执行文件：[003_role_template_type_and_version.sql](database/migrations/003_role_template_type_and_version.sql)，迁移前快照为 [20261125_before_003.sql](database/snapshots/20261125_before_003.sql)。执行后仅更新SQL头部状态注释，禁止重跑。下方001、002历史记录不与003核对结果混用。

service_name为VARCHAR(64) NOT NULL、utf8mb4_bin排序，无默认值，中文注释明确必填及区分大小写。idx_service_path(business_code, service_name, path)为普通查询索引，不承担并发唯一性约束。普通表级排序规则仍保持原值，权限记录的菜单ID格式未变。

应用在新增、修改和实际鉴权时均校验serviceName非空、无首尾空白且最多64个UTF-8字符；不会把空值作为通配或默认API服务。已有数据的其他环境不能直接套用空表迁移流程，须先明确各菜单所属服务再制定回填方案。

本迁移针对role_permission为空、仍有旧permission_ids和is_super_role列、admin_permission尚不存在的库，已在执行前重新核对。旧字段保存的是权限组ID，不能将非空历史数据直接当作菜单ID转换。

本次迁移已执行，不能直接对当前库重复运行旧列转换。CREATE TABLE IF NOT EXISTS不保证已有同名表符合设计；再次迁移前须先检查实际结构。MySQL DDL隐式提交，不能用ROLLBACK撤销。

## 实际执行及核对结果

使用项目最新配置连接user_rpc，MySQL返回版本9.7.1。此前错误库名导致的1049仅为连接失败，没有创建该库或执行任何DDL；本次配置已由用户修正，未另改连接地址或凭证。

实际命令如下，密码不在命令行中传递：

```text
mysql --defaults-file=<0600临时凭证文件> --no-login-paths --protocol=TCP --connect-timeout=5 --batch --raw --skip-column-names
mysql --defaults-file=<0600临时凭证文件> --no-login-paths --protocol=TCP --connect-timeout=5 --default-character-set=utf8mb4 --batch --raw --skip-column-names < 001_admin_permission_and_role_template.sql
```

执行器在DDL前设置utf8mb4及5秒元数据锁/行锁等待预算，客户端进程预算25秒。SQL通过标准输入提交给mysql，未在shell命令中展开凭证。

| 检查或操作 | 实际结果 |
|---|---|
| 迁移前只读检查 | 连接成功；role_permission、admin_info、menu、permission_menu精确COUNT均为0 |
| 迁移前重复检查 | 角色表旧列顺序和类型符合预期，角色记录数为0，admin_permission不存在 |
| 结构快照 | SHOW CREATE TABLE保存角色、管理员、菜单、分组四张表结构，成功 |
| CREATE TABLE admin_permission | 成功 |
| ALTER TABLE role_permission | 成功，permission_ids→menu_ids，JSON→TEXT，删除is_super_role |
| 迁移后列与索引 | information_schema核对符合目标，user_id唯一键存在，角色旧两列不存在 |
| 非目标表结构 | admin_info、menu、permission_menu的SHOW CREATE TABLE结果与迁移前完全一致 |
| 迁移后记录数 | admin_permission、role_permission、admin_info、menu、permission_menu均为0 |

没有插入角色、用户、权限或验证样本，没有执行DROP TABLE、TRUNCATE或授权初始化。没有运行服务、Go构建、业务测试或前端验证；以上结果仅证明数据库结构迁移及元数据核对成功，不代表业务接口已可用。

## 临时文件清理与下一阶段

每次连接的执行结果和限制均先登记，再删除对应临时凭证文件；本轮未新增测试用例、测试脚本、依赖缓存或账户数据导出。迁移SQL和必要结构快照作为交付文件保留，原有测试及用户文件保留。

当前Go模型和RPC业务仍使用旧permission_ids、is_super_role及角色继承逻辑，与数据库新角色结构不匹配，角色相关接口需等下一阶段适配后再使用。下一阶段讨论并实现用户权限读写、模板覆盖、按用户ID鉴权及协议变更；本阶段不提前修改这些接口。

GetUserPermissionTree依次普通查询目录和用户勾选状态，不开启事务或读取旧角色字段；模型自动过滤软删除记录，查询限定当前business-code。SaveUserPermission以最终菜单IDs覆盖保存，version=0首次INSERT，已有或软删除记录按版本UPDATE并恢复NULL；写入会话关闭GORM默认事务，唯一键和版本条件保护用户记录。CheckUserPermission已按用户直接权限查询有效菜单路径，不需要新增数据库结构。具体响应、错误和执行状态见 [用户权限接入](USER_PERMISSION.md)。模板读写、旧菜单树及外部调用方仍待后续调整。

用户权限模型第一步已完成：新增internal/dao/schema/admin_permission.go中的AdminPermission，映射admin_permission全部八个字段、user_id唯一索引及业务归属索引。使用default:null使软删除默认查询匹配数据库NULL语义；CreatedAt/UpdatedAt由GORM常规创建、更新流程按Unix秒填写，Version默认1，未实现版本比较或读写接口。此步骤只执行gofmt格式化新增Go文件，未运行构建、测试、服务或数据库操作；没有新增测试文件、测试脚本或临时产物，无需清理。原有测试保留。

清理后按本次任务前缀核对，剩余临时凭证文件数为0。

模型软删除统一处理仅修改上述schema、相关DAO和三个删除Logic，并同步接入说明。实际执行gofmt格式化14个修改的Go文件，完成源码引用核对；未运行构建、测试、服务、RPC调用或数据库操作。没有新增测试文件、测试脚本、依赖缓存或临时产物，无需清理；原有测试及用户文件保留。

SaveUserPermission已实现全量覆盖、首次创建、条件版本更新及显式恢复NULL，新增授权不得包含停用菜单；写入会话关闭默认事务。保存接口开发仅做离线模拟SQL定向测试及本地Client/Server编译，结果通过，没有连接或写入真实数据库；命令、范围与清理记录见 [保存接口验证](USER_PERMISSION.md)。本次没有执行额外数据库迁移。

CheckUserPermission已在同一用户权限模型中实现：普通查询校验当前业务正常管理员、未删除的独立权限和已启用菜单路径，不读取角色或模板，不写库、不使用Unscoped或事务，不需要新增迁移。协议及本地入口已同步；外部调用方和旧菜单树仍待后续迁移。
2026-11-24迁移002前检查：user_rpc.menu精确COUNT为0，新列及索引不存在；结构已保存到database/snapshots/20261124_before_002.sql。

开始执行002：仅ALTER TABLE menu新增必填service_name和业务/服务/路径普通索引；DDL隐式提交，锁等待预算5秒，客户端进程预算25秒，不回填或写入记录。

002迁移成功，实际列及索引核对通过：

```text
service_name	varchar(64)	NO	<NULL>	utf8mb4_bin	接口所属API服务名，如admin-api；增改及权限校验必填，区分大小写，不提供默认服务或空值通配
idx_service_path	1	1	business_code
idx_service_path	1	2	service_name
idx_service_path	1	3	path
menu_count	0
```

本次数据库命令使用0600临时凭证文件和标准输入SQL，未在命令行传密码；结果先登记再删除凭证文件，原始错误输出不展示。未插入或修改任何业务记录。


迁移002及开发检查结果登记后，本次临时执行脚本、凭证、两个测试文件和独立缓存均已清理；迁移SQL及结构快照作为交付资料保留。后续测试或构建验证按新约定等待用户确认。
