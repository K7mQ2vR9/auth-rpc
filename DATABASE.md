# auth-rpc 数据库设计

2026-11-25模板配置树与分组成员回显修正为代码及RPC协议调整，无新增表、列或迁移，未连接数据库。默认模板1/2创建后不可删除，由业务入口限制；不新增删除或恢复流程，已有唯一约束继续保留。

2026-11-24已在user_rpc执行迁移001、002；2026-11-25已执行003—008。目标库和连接标识仍分别为user_rpc、user-rpc，未创建或切换数据库，未改动其他项目。本次迁移前后六张相关权限表均为空，未初始化模板、账号或授权。

## 迁移008执行完成（2026-11-25）

用户已明确授权执行008，并要求同步清理无用表。[008_optimistic_catalog_version.sql](database/migrations/008_optimistic_catalog_version.sql)已在auth-rpc当前配置的user_rpc执行并核对，MySQL9.7.1，连接标识仍为user-rpc；未改配置或切换数据库，001—007没有重跑。

### 前置核对及备份

重新读取实际结构：menu和menu_group无version及相应CHECK，007活跃路径生成列、三列唯一索引、服务名utf8mb4_bin完整。旧permission_catalog_lock只有一列id、一条id=1元数据，限定id=1的CHECK启用，未发现引用它的外键、触发器或本库视图。本地进程清单未发现auth_rpc/auth-rpc/authRpc可执行进程，执行前数据库无活动查询或未结束事务；未关闭现有空闲连接或终止其他进程。该检查仅覆盖本地进程及当时数据库连接，不声称远端实例已由本轮部署或停机。

六张权限业务表精确COUNT均为0。已保存[008前结构及精确计数备份](database/snapshots/20261125_before_008.sql)，包含六张空表、旧目录锁DDL及唯一id=1元数据备份，不含业务账号、连接信息或凭证，非自动回滚脚本；迁移后另保存[008后实际结构及计数快照](database/snapshots/20261125_after_008.sql)。

### 实际执行结果

| 操作或检查 | 实际结果 |
|---|---|
| ALTER menu | 成功新增version：BIGINT UNSIGNED、NOT NULL、DEFAULT 1，启用chk_menu_version限定1..9223372036854775807 |
| ALTER menu_group | 成功新增相同版本列，启用chk_menu_group_version限定相同范围 |
| DROP旧目录锁表 | 成功，permission_catalog_lock已不存在，仅删除唯一id=1互斥元数据 |
| 保留007能力 | active_route_path生成列、uniq_active_service_path及所有原菜单/分组字段和索引保持 |
| 六张业务表计数 | admin_info、admin_permission、role_permission、menu、menu_group、permission_audit前后均为0 |
| 四张非目标权限表 | admin_info、admin_permission、role_permission、permission_audit结构摘要及计数不变 |
| 数据库表清单 | 原20张，后19张；集合差异仅permission_catalog_lock，无其他表删除或新增 |

### 无用表清理范围

本次确认废弃并删除的auth-rpc表仅permission_catalog_lock，源码已不再引用，依赖和唯一元数据核对均完成。实际库还包含13张非auth-rpc业务表：管理员登录设备/日志/Token、用户账号/资料/设备/日志/Token、操作日志、注册渠道、配置及代理等。部分在user-rpc等项目有明确引用，其余尚未认定为废弃；不因当前权限服务未读取或表暂为空就扩大删除共享业务表。menu、menu_group、role_permission、admin_permission、admin_info、permission_audit仍有业务职责，全部保留。

### 实际命令及执行限制

```text
python3 <临时目录>/migrate.py inspect
python3 <临时目录>/migrate.py apply
python3 <临时目录>/migrate.py verify
```

最初沙箱连接返回2003，未执行DDL；获准网络后inspect、apply、verify成功。本地进程清单最初读取受限，获准执行器内仅统计服务进程名，未输出进程参数。执行器从项目配置取得当前连接，使用0600临时凭证文件，密码不出现在命令参数或输出，SQL经标准输入提交。执行前再次比对结构、精确计数、SQL摘要及旧锁表唯一元数据；各阶段以普通只读空表条件检查后执行，未使用LOCK TABLES、SELECT FOR UPDATE/FOR SHARE或额外会话互斥方案。连接超时8秒，元数据锁/行锁等待预算各10秒，DDL客户端55秒；三个阶段均成功，无自动DDL重试。数据库内部执行DDL需要的元数据锁由引擎管理，并非业务锁表方案。

后置核对只使用SHOW CREATE TABLE、information_schema、精确COUNT及表清单，未写入验证样本，未创建账号、模板或授权，未运行Go测试、构建、服务、RPC或并发测试。DDL分阶段隐式提交，不可盲目重跑；其他环境需重新核对、停用旧写入实例并备份，当前库禁止重复执行008。菜单/分组调用方须显式回传已读version，旧缺版本写入不兼容；本轮未启动或部署服务。

执行结果、实际命令及限制登记后，已删除临时执行器、0600凭证、前后状态与表清单JSON、SQL摘要、执行标记及日志和所属临时目录，已核对该目录与定位文件均不存在。正式迁移及前后快照保留；未新增测试、测试样本、验证缓存，原有测试及用户文件保留。

## 迁移007执行完成（2026-11-25）

[007_permission_catalog_integrity.sql](database/migrations/007_permission_catalog_integrity.sql)已在当前配置的user_rpc执行成功，连接标识仍为user-rpc，MySQL9.7.1；001—006没有重跑，数据库配置未修改。实际结构已核对，不代表业务测试或并发场景已通过。

### 前置及备份

重新核对005/006的生成列、唯一索引及启用CHECK，菜单service_name仍为utf8mb4_bin；007活跃路径列、唯一索引和permission_catalog_lock均不存在。精确服务/路径重复和非法菜单检查均为0，六张业务表精确COUNT均为0，因此没有存量父级或分组成员待逐条核对。未以历史空表结果代替本次连接检查。

[迁移前结构及精确计数快照](database/snapshots/20261125_before_007.sql)覆盖六张空表的结构及空数据状态，不含业务记录、连接信息或凭证；[迁移后结构及计数快照](database/snapshots/20261125_after_007.sql)另记录目录锁表和唯一id=1元数据行。两份快照均非可直接执行的回滚脚本。

### 实际结果

| 检查项 | 实际结果 |
|---|---|
| 007执行 | ALTER menu、CREATE目录锁表、INSERT固定锁行三个步骤均成功，未重试DDL |
| active_route_path | VARBINARY(1020)、STORED；未删除且非空路径保留原始字节，软删除或空路径为NULL |
| 活跃路径唯一约束 | uniq_active_service_path按business_code、service_name、active_route_path建立三列唯一索引，服务名utf8mb4_bin及路径二进制精确比较 |
| 目录锁表 | InnoDB，id为TINYINT UNSIGNED NOT NULL、主键唯一，chk_permission_catalog_lock_id启用且限定id=1 |
| 锁行 | 有且仅有id=1的一条目录互斥元数据，不是账号、菜单、模板或授权 |
| 六张业务表记录数 | admin_info、admin_permission、role_permission、menu、menu_group、permission_audit迁移前后均为0 |
| 原结构 | menu原字段/索引/CHECK保持；其余五张表结构摘要及计数均未变化 |

MySQL将OCTET_LENGTH(path)和CAST(path AS BINARY)规范化显示为LENGTH(path)和CAST(path AS CHAR CHARSET binary)。首次后置执行器因函数文本匹配未识别该表示而停止；只读查看真实表达式后调整执行器识别，最终核对成功，未修改或重跑已执行SQL。

实际命令（临时目录为占位符）：

```text
python3 <临时目录>/migrate.py inspect
python3 <临时目录>/migrate.py apply
python3 <临时目录>/migrate.py verify
```

首次沙箱连接返回2003，未执行DDL；获准网络后inspect、apply和最终verify成功。执行前再次比对结构、空表、迁移SQL摘要，拒绝活动查询或未结束事务，不关闭现有空闲连接。apply使用会话互斥锁及菜单WRITE/分组READ锁和空表守卫执行ALTER；之后释放表锁、创建目录锁表并初始化固定行，结束释放会话互斥锁成功。连接超时8秒，元数据锁及行锁等待各10秒，DDL客户端预算55秒，无自动DDL重试。密码只保存于0600临时文件，不在命令参数或输出中；SQL通过标准输入提交。

核对仅SHOW CREATE TABLE、information_schema及精确COUNT/锁行读取，未插入业务或验证样本、创建账号或初始化授权。未运行Go测试、构建、示例、服务、RPC或并发业务验证。当前库禁止重跑007；其他环境必须另行核对、停写并备份，DDL隐式提交，失败按实际阶段处理。

用户在007执行后提出改用version乐观锁、不考虑跨菜单并发成环，并要求以后统一使用乐观锁，规则已写入根AGENTS.md及公共包开发规范。该段为执行007时的历史状态；用户随后已确认替换全部显式悲观锁，源码替换已完成，008随后获准执行并核对，实际锁表已删除、两个版本列已落库；本轮未部署服务，详见本页008实际执行记录。

执行结果、实际命令和限制登记后，已删除临时执行器、0600凭证、前后状态JSON、SQL摘要及执行标记/日志和所属临时目录，已核对该目录与定位文件均不存在。正式迁移SQL及快照保留；未新增测试文件、测试样本或验证缓存，原有测试与用户文件保留。

## 迁移005与006执行完成（2026-11-25）

最新模板类型为1平台默认、2站点默认、3本站普通。平台默认是真实可维护权限集合，仅10管理，不是10自身默认全操作权限的来源。11在模板1范围内逐人授权并维护不能超出1的站点模板2；本站唯一20默认取得本站全部有效操作权限，且类型3全部管理及复制仅20，30/40不能操作模板。

[005_platform_default_template.sql](database/migrations/005_platform_default_template.sql)已执行成功。前置重新核对003/004结构、旧类型CHECK和根生成列、唯一索引、新列及约束尚不存在，并排查旧模板类型及归属。005三阶段全部成功：移除旧类型CHECK和根唯一索引；执行旧1→新2、旧2→新3转换语句；根槽位改为类型2并增加平台默认唯一槽位、唯一索引及类型/归属CHECK。实际模板表为空，转换语句影响0行，未插入或更新业务记录。

[006_unique_site_super.sql](database/migrations/006_unique_site_super.sql)已执行成功。已排查20非法归属和重复身份，新增site_super_business_code生成列、uniq_site_super唯一索引及启用的chk_site_super_scope。含软删除记录保留名额，仅保证每站点最多一位20；至少一位仍由建站/账号初始化流程保证。未改身份值、合并账号或创建20，替换/删除/恢复流程仍需另行设计。

### 实际执行及结构核对

MySQL版本9.7.1。迁移前保存[六张表结构及精确记录数快照](database/snapshots/20261125_before_005_006.sql)，迁移后保存[实际结构及精确记录数快照](database/snapshots/20261125_after_005_006.sql)。六表均为空，这些快照覆盖其现有结构和空数据状态，不含连接凭证或账号数据，也不是可直接执行的回滚脚本。

| 检查项 | 实际结果 |
|---|---|
| 005执行 | 三个阶段全部成功；新类型1/2/3，role_permission共11列 |
| 模板生成列 | root_business_code仅类型2生成站点编码；platform_default_slot仅类型1生成1，均为STORED |
| 模板唯一约束 | uniq_site_root、uniq_platform_default均为对应单列唯一索引，软删除不释放槽位 |
| 模板CHECK | 类型1/2/3、归属、原业务编码和版本共4项，全部启用 |
| 006执行 | 成功；admin_info共23列，新增20专用STORED生成列、单列唯一索引及归属CHECK |
| 记录数 | admin_info、admin_permission、role_permission、menu、menu_group、permission_audit前后精确COUNT均为0 |
| 原结构保留 | 两目标表其他原列/索引/CHECK保持，除005明确修改的模板类型与根生成列；四张非目标表结构摘要完全一致 |
| 业务操作 | 未创建模板、账号、授权或测试样本，未删除role_id，无服务/RPC调用 |

实际命令如下，临时目录用占位符表示：

```text
python3 <临时目录>/migrate.py inspect
python3 <临时目录>/migrate.py apply
python3 <临时目录>/migrate.py verify
```

首次沙箱连接返回MySQL 2003，未执行DDL；获准网络后前置核对和备份成功。初次apply因存在其他连接在DDL前停止；只读确认3个连接均为长时间Sleep后，调整为拒绝活动查询或未结束事务，未关闭或终止现有连接。临时执行器的一次引号语法错误在运行前修正，该次未连接数据库。最终apply重新核对结构与空表后，以会话互斥锁、目标表WRITE锁和各阶段空表守卫执行；ALTER阶段后重新取得锁，不假设表锁跨DDL一直保留。005三阶段和006均返回完成标记，最后释放互斥锁成功，未自动重试任何已开始的DDL。

连接凭证只保存在权限0600的临时文件，密码不出现在命令参数或输出；项目配置未修改。SQL经标准输入提交，连接超时8秒，元数据锁及行锁等待各10秒，DDL客户端总预算55秒。核对只读SHOW CREATE TABLE、information_schema及精确COUNT，不插入样本验证约束；结构成功不代表三级权限业务、事务回滚或完整平台联调通过。本轮未运行Go测试、构建、范例、服务或RPC，相关测试仍待用户另行确认。

005/006已执行，禁止在当前库重跑。其他环境需先核对实际结构及数据、暂停相关写入口并备份；DDL分阶段隐式提交，失败须按实际阶段处理，不能依靠ROLLBACK或盲目从头重跑。003/004历史SQL及快照保留原执行含义，后文旧类型描述仅是历史记录。

迁移结果、实际命令和限制登记后，已删除本轮临时执行器、0600凭证、前后状态JSON、执行标记及输出日志和所属临时目录，已核对该目录及定位文件均不存在。正式迁移SQL和前后结构快照保留，未新增测试用例、测试样本或验证缓存，原有测试与用户文件保留。

源码已将模板创建/更新、普通模板增改删、手工授权和复制与审计同事务。操作者仅资格及审计，不作为下发集合上限；编辑页隐藏失效菜单，手工保存过滤失效ID后再检查当前模板上限。菜单树所有账号包括10/20按配置读取，默认操作权限不自动转为全量菜单。详细协议及示例见USER_PERMISSION.md。

## 当前表结构与职责

| 表 | 当前状态 |
|---|---|
| admin_permission | 已新增；一个用户一条独立权限记录，user_id唯一，menu_ids为英文逗号分隔的具体菜单ID |
| role_permission | 005已执行；1平台默认、2站点默认、3本站普通；平台唯一及站点唯一槽位已生效 |
| menu_group | 本次实际数据库与模型均使用此表名，成员为JSON格式，仅用于菜单展示；旧快照中的permission_menu不是本轮修改对象 |
| menu | 在原页面、按钮等字段基础上新增必填service_name及业务/服务/路径普通查询索引，其他原字段保持 |
| admin_info | auth-rpc已补齐AccountType映射及五种类型常量，并移除RoleId映射和按role_id筛选；数据库role_id列本轮未删除，其他项目引用未改动 |
| permission_audit | 迁移004已执行并核对16字段、4索引及空表；平台/站点默认创建更新、普通模板增改删、模板复制及SaveUserPermission已接入同事务成功审计源码 |

admin_info.account_type沿用：10=平台超级管理员、11=平台管理员、20=站点超级管理员、30=站点管理员、40=代理管理员。auth-rpc的schema.AdminInfo已映射AccountType，并定义与user-rpc数值一致的AdminInfoAccountType10/11/20/30/40常量及中文注释；0表示未设置，不属于这五种有效类型。不增加新的身份字段，不压缩为三种类型，不自动转换现有账号类型；数字值保持不变，20的最新业务含义已改为每站点唯一超级管理员，源码已接入相应默认操作权限与模板资格；006结构约束已执行并核对。账号身份与角色权限模板分别管理。

源码阶段已完成账号类型映射、用户角色解绑、独立授权登录菜单及模板基础CRUD。平台根模板创建、查询、更新和按模板覆盖用户的源码已同步，权限写入同事务审计。用户授权后已完成迁移003及004，未插入、更新或删除业务记录，未删除admin_info.role_id；运行时接口鉴权和登录菜单也已接入身份、当前根交集及完整祖先链；普通模板也已接入两级上限、唯一20资格及同事务审计；目录/分组管理的剩余边界另行处理。未修改其他项目，未运行Go测试、构建、服务、RPC或业务联调，结构迁移成功不代表完整三级权限业务可用。

### permission_audit（迁移004已执行）

[004_permission_audit.sql](database/migrations/004_permission_audit.sql)已于2026-11-25在user_rpc执行。前置核对003结构及同名表不存在后，仅创建空表，不新增根模板、账号、菜单或授权。当前库禁止重跑；其他环境先核对实际迁移状态和结构，不忽略已有表差异。

表使用InnoDB，无软删除和自动清理策略。字段包括自增id、服务端operation、操作者id/account_type/business_code、目标站点business_code、target_type/target_id、前后menu_ids及version、source_template_id/source_template_version、trace_id和created_at（Unix秒）。创建根模板时前集合为空、前版本0，后版本1；根更新及复制用户后版本递增1。复制记录目标用户ID、用户授权前后快照和来源模板ID/版本，其他操作来源字段为0。菜单集合使用规范化CSV，trace_id复用当前32位链路ID，不保存密码、Token或配置密钥。

索引为主键id、(business_code,created_at,id)、(operator_id,created_at,id)及trace_id；Trace列使用ascii_bin，其余默认utf8mb4_unicode_ci。根模板创建/更新、模板复制及SaveUserPermission分别与审计插入使用同一tx：审计表缺失或写入失败均返回错误并回滚权限写入，不降级跳过审计；普通模板增改删已接入同事务审计，删除保留原存储CSV并记录删除操作及递增版本。手工保存operation=save_user_permission、target_type=user_permission、target_id为用户ID，来源模板ID和版本均为0，使用已有字段，无需新增DDL。数据库保留user_rpc、DbKey保留user-rpc；源码改造阶段未连接数据库；本轮005/006仅迁移并核对结构，未执行业务调用，结构核对不代表新写入和回滚行为已验证。

### 迁移004执行状态

2026-11-25使用auth-rpc当前配置连接user_rpc，MySQL版本9.7.1。执行前保存[现有权限表结构快照](database/snapshots/20261125_before_004.sql)，仅含DDL和精确记录数，不含业务记录、连接信息、凭证或密钥，也不是可直接执行的回滚脚本。

| 操作 | 实际结果 |
|---|---|
| 前置核对 | permission_audit不存在；003类型、版本、根生成列、唯一索引及三个启用CHECK均符合预期 |
| 执行004 | 成功，仅CREATE TABLE permission_audit，不重跑001—003 |
| 字段 | 16个字段，逐项核对类型、非空、默认值及注释；id自增，trace_id为CHAR(32)、ascii_bin |
| 索引及引擎 | 主键及3个查询索引符合SQL；InnoDB，表排序规则utf8mb4_unicode_ci |
| 审计表记录数 | 精确COUNT为0，无样本写入或业务调用 |
| 非目标表 | admin_info、admin_permission、role_permission、menu、menu_group的结构摘要（不含AUTO_INCREMENT计数）和精确COUNT均未变化，均为0条 |

实际执行命令如下，临时目录用占位符表示：

```text
python3 <临时目录>/migrate.py inspect
python3 <临时目录>/migrate.py apply
python3 <临时目录>/migrate.py verify
```

首次沙箱内inspect连接返回MySQL 2003，未执行DDL；获准联网后inspect、apply和verify均成功。执行器从项目配置读取当前连接，使用0600临时凭证文件，密码不出现在命令参数或输出；SQL通过标准输入提交。连接超时8秒，DDL会话元数据锁及行锁等待均为10秒，客户端预算45秒，不自动重试DDL。SQL执行后只更新迁移文件头部状态注释，当前库不可重跑。

核对仅使用SHOW CREATE TABLE、information_schema及精确COUNT，未写入测试样本，未运行Go测试、构建、服务、RPC或业务联调，不能将结构完成视为事务回滚或业务逻辑已验证。正式迁移SQL和结构快照保留。结果登记后，已删除本次临时执行器、0600连接凭证、状态文件及其目录，并核对目录不存在；未新增Go测试、测试样本或验证缓存，原有测试与用户文件保留。

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

menu_ids由SaveUserPermission在写入前校验、去重、排序及检查存储字节上限；数据库TEXT字段本身不保证格式或菜单存在。version由保存接口进行条件比较和递增，数据库不会无条件自动递增。查询仍限定ctx当前站点，写入在同一事务内校验管理资格、目标账号归属、站点默认上限及共享菜单完整祖先链；手工提交先过滤失效项，不关联操作者本人权限；平台使用显式目标businessCode，站点禁止跨站点。写入与审计原子提交，首次版本0、已有或恢复原版本，软删除恢复NULL并保留原ID及创建时间，不写入角色绑定。

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

保留现有主键、业务索引和其他字段。根模板由平台专用CreateSiteRootTemplate显式创建，GetSiteRootTemplate/UpdateSiteRootTemplate负责读取及按版本覆盖；普通CRUD不能创建、修改或删除根模板，SQL不插入占位数据。普通自定义模板写入锁定当前站点根模板检查子集，再检查具体菜单存在、启用且为页面或按钮，不继承菜单分组。完整接口和失败语义见 [权限模板接口](USER_PERMISSION.md#权限模板接口)。

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

GetUserPermissionTree依次普通查询目录和用户勾选状态，不开启事务或读取旧角色字段；模型自动过滤软删除记录，查询限定当前business-code。SaveUserPermission现已与模板复制共用带身份资格、站点默认上限、菜单链及审计的事务覆盖流程，version=0首次INSERT，已有或软删除记录按版本UPDATE并恢复NULL；不再关闭默认写事务。CheckUserPermission和GetMenuByAdminId已共用身份、站点根交集及共享平台目录完整祖先链计算；平台10按真实身份默认全操作、11读取独立授权与平台默认模板交集；所有账号的菜单树仍读取已配置权限，普通读取不加事务或缓存，不修改任何记录。具体响应、错误和执行状态见 [用户权限接入](USER_PERMISSION.md)。本轮005/006已执行并核对，未测试、构建或调用服务；临时迁移执行器及凭证按本页记录清理，原有测试保留。

用户权限模型第一步已完成：新增internal/dao/schema/admin_permission.go中的AdminPermission，映射admin_permission全部八个字段、user_id唯一索引及业务归属索引。使用default:null使软删除默认查询匹配数据库NULL语义；CreatedAt/UpdatedAt由GORM常规创建、更新流程按Unix秒填写，Version默认1，未实现版本比较或读写接口。此步骤只执行gofmt格式化新增Go文件，未运行构建、测试、服务或数据库操作；没有新增测试文件、测试脚本或临时产物，无需清理。原有测试保留。

清理后按本次任务前缀核对，剩余临时凭证文件数为0。

模型软删除统一处理仅修改上述schema、相关DAO和三个删除Logic，并同步接入说明。实际执行gofmt格式化14个修改的Go文件，完成源码引用核对；未运行构建、测试、服务、RPC调用或数据库操作。没有新增测试文件、测试脚本、依赖缓存或临时产物，无需清理；原有测试及用户文件保留。

2026-11-24旧版SaveUserPermission曾使用单条版本写入及停用项保留规则，当时离线模拟SQL定向测试和本地Client/Server编译通过，没有连接或写入真实数据库；命令、范围与清理记录见 [保存接口历史验证](USER_PERMISSION.md#保存接口的开发及验证历史行为)。当前已改为同事务授权上限校验、严格菜单链检查和审计，旧结果不能证明本轮实现通过；本轮未运行测试或构建，另按用户授权执行005/006结构迁移，未验证业务接口。

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
