# auth-rpc 权限接口使用说明

本轮已按[权限设计](PERMISSION_DESIGN.md)更新源码：11在平台默认范围逐人授权，20默认拥有本站全部有效操作权限，普通模板3全部操作仅本站唯一20。菜单树仍统一按已配置权限返回，默认操作权限不会自动展开菜单。源码所需005/006已于2026-11-25在user_rpc执行并核对，新类型及唯一约束已生效；六张相关表仍为空，未初始化模板、账号或授权，未运行Go测试、构建或业务联调。

## 模板及身份

| 类型 | 模板 | 接口与资格 |
|---|---|---|
| 1 | 平台默认模板，全平台一份真实权限集合 | Create/Get/UpdatePlatformDefaultTemplate，仅平台超级管理员10 |
| 2 | 站点默认模板，每站点一份，不能超过平台默认 | Create/Get/UpdateSiteRootTemplate，仅平台普通管理员11 |
| 3 | 本站普通模板，同站点可多份，不能超过两级默认交集 | AddRole/UpdateRole/DeleteRole/GetRoleList/GetOneRole/GetRoleMenus及CopyRolePermissionToUser，仅本站唯一20 |

account_type=20已改为站点超级管理员，每站点有且只有一位；30为站点管理员，40为代理管理员，30/40不能操作模板。身份按数据库精确值判断，不按枚举大小推断资格，不从请求体或roleId获取平台身份。所有管理入口读取可信ctx UID、合法Trace及归属，API仍须校验登录和目标操作许可，RPC公共中间件只做metadata/Trace准入。

平台10账号自身默认全操作权限，不受平台默认模板收缩限制；模板1是由10维护、供11使用的集合，两者不同。11实际操作权限为个人授权与模板1的交集；20实际操作权限默认是本站模板2与平台模板1的有效交集，不要求个人CSV授予。站点30/40仍是个人授权与两级默认模板的交集。操作者个人权限不作为向下授权集合的上限，但身份及API操作许可仍须校验。

## 平台默认模板接口

保持平台ctx的business-code=99999999，必须为正常且未删除的10；不由请求体指定操作者、类型或记录ID。

| RPC | 请求示例 | 响应 |
|---|---|---|
| CreatePlatformDefaultTemplate | `{"menuIds":"1,2"}` | id及version=1 |
| GetPlatformDefaultTemplate | `{}` | data为真实RoleListItem，含menuIds、templateType=1、version等 |
| UpdatePlatformDefaultTemplate | `{"menuIds":"1","version":1}` | 新version |

menuIds必须显式提交，空字符串表示无权限，不默认为全部；Get不自动创建或恢复。不存在固定allPermissions=true响应。全平台唯一索引保护含软删除记录的名额，重复创建AlreadyExists。修改只按已读版本覆盖集合，版本冲突Aborted，不重试覆盖。写入与成功审计同事务，收权影响下级后续实际访问，不裁剪站点、模板或用户的历史CSV。

10配置模板时可从全部有效共享目录选择；新菜单不自动写入已保存模板或用户授权。缺失平台默认模板不能作为全部权限下发，但10自身操作资格及平台模板初始化不依赖该记录。

## 站点默认及普通模板

站点默认沿用SiteRootTemplate的RPC名称，实际类型为2。平台11保持平台ctx，businessCode显式指定非平台目标站点；新增显式menuIds，修改另传正数已读version，查询只传目标编码。首次固定名称及版本1，已存在或软删除记录拒绝再建，不能删除、转移或转换为普通模板。新完整集合普通读取并检查模板1子集及菜单/完整祖先有效性，不检查11本人授权集合。

普通模板3仅本站唯一正常20可操作，30/40及平台10/11均拒绝。AddRole示例`{"data":{"roleName":"客服模板","menuIds":"1,2","templateType":3}}`；UpdateRole另传id及version；DeleteRole提交id/version。模板读取和列表也校验20资格，列表类型0或3只返回本站普通模板。名称无首尾空白且最多255字符，列表默认1页/20条、最大200，时间范围为Unix秒。

新增/修改普通读取平台、站点默认上限快照，完整集合必须处于交集中并通过目录链校验；空集合仍要求默认模板有效。删除普通模板不要求默认模板仍存在，模型软删除并比较递增版本，不修改已复制用户。增改删均与成功审计同事务，审计失败回滚。软删除审计保留原CSV，操作名体现删除及版本递增。

## 独立模板配置树

GetTemplatePermissionTree只读返回分组、有效目录、当前模板勾选及可选范围，不依赖目标用户。调用时仍须携带合法Trace、可信UID和business-code；身份以数据库中正常且未删除的账号为准。

| 用途 | 请求示例 | 调用方与可选范围 |
|---|---|---|
| 平台默认1 | `{"templateType":1}` | 平台ctx中的10，可选全部有效共享目录 |
| 站点默认2 | `{"templateType":2,"businessCode":"site001"}` | 平台ctx中的11，可选平台默认1范围 |
| 本站普通3新建 | `{"templateType":3}` | 本站ctx中的唯一20，可选平台默认1与本站默认2交集 |
| 本站普通3编辑 | `{"templateType":3,"templateId":123}` | 同上，指定模板必须属于本站且类型为3 |

类型1/2省略templateId或传0，自动定位当前归属的默认模板；正数必须匹配实际默认模板。类型3传0表示新建预览，不自动挑选已有普通模板。返回templateType、businessCode、templateId、templateName、version和groups，节点结构复用UserPermissionGroup/UserPermissionMenu；version是模板版本，不能当作用户授权版本。已有模板返回实际ID、名称、版本及checked；确实未创建或普通模板新建预览返回ID/version=0、名称为空、无勾选。

类型2必须已有平台默认1；类型3必须已有两级默认。必要上级缺失则拒绝，不将全部目录当作授权上限。配置树隐藏删除、禁用及祖先失效菜单，超出上限的权限项直接过滤，历史勾选不再返回；必要祖先仅保留displayOnly/disabled补齐树结构，上限外祖先不勾选、不提交为权限。查询不创建、恢复或修改记录，显式保存当前勾选完整覆盖后才清除旧ID，原有独立模板详情接口的资格不变。

默认模板1、2创建后不可删除，包括软删除，只能按资格及已读version修改；DeleteRole仅删除普通模板3。历史异常或外部误删的默认记录仍占唯一名额，配置树报错，不将其作为新建预览或自动恢复。

## 授权编辑与手工保存

模板配置页使用独立的GetTemplatePermissionTree，不传userId；本节GetUserPermissionTree继续用于真实用户授权。

GetUserPermissionTree请求userId及businessCode（第2号字段），SaveUserPermission请求userId、显式menuIds、显式version及businessCode（第4号字段）。平台调用明确指定目标归属；站点可省略使用ctx归属，显式传入不得跨站点。

给平台11赋权时，操作者必须为平台10，目标归属明确传99999999；返回目录不按目标已选项缩减，但可选范围受平台默认模板1限制，逐人保存独立admin_permission。不会自动授予整份模板。给站点用户赋权时使用目标站点，两级默认交集为上限；不与操作者个人授权取交集。唯一自改例外为平台10配置自身菜单展示，其可选全部有效目录且不依赖模板1，不改变默认操作权限；其他身份禁止自行修改权限；目标可停用但须未删除，操作不改变账号身份或状态。

编辑页使用共享menu/menu_group，隐藏删除、禁用、受无效祖先影响及超出模板上限的权限项，不再用禁用勾选表达旧权限。仅补齐组树结构的祖先displayOnly且disabled，上限外祖先不勾选，不自动授予父子节点。checked取用户已保存CSV与当前上限内有效目录交集，软删除授权只保留原版本不勾选；无记录version=0。分组成员JSON与授权CSV分别解析，未分组ID=0仅虚拟展示，空分组保留，不增加已选汇总区。

手工保存普通读取菜单和完整祖先链，删除、禁用、目录中已不存在或依赖不可用祖先的ID按取消勾选过滤，再检查剩余有效集合是否超过模板上限。有效越权、非法CSV、有效跨目录项或结构损坏仍整体拒绝，不静默裁剪有效越权项。查询不修改数据库，只有用户显式保存后才移除旧授权。

```go
// 示例仅说明格式，不执行真实调用。给平台11赋权时BusinessCode改为99999999，并使用平台10原ctx。
tree, err := authClient.GetUserPermissionTree(ctx, &roleauthservice.GetUserPermissionTreeReq{
    UserId: 1001, BusinessCode: "100001",
})
if err != nil { return err }
saved, err := authClient.SaveUserPermission(ctx, &roleauthservice.SaveUserPermissionReq{
    UserId: 1001, BusinessCode: "100001",
    MenuIds: proto.String("1,2"), Version: proto.Int64(tree.Version),
})
if err != nil { return err }
// 后续提交使用saved.Version；明确清空使用proto.String("")。
```

proto来自google.golang.org/protobuf/proto。CSV为正数int64英文逗号集合，允许单项空白、前导零和重复，规范化去重升序后最多65535字节，不截断。首次version=0，已有或恢复记录使用原版本且不能为MaxInt64。恢复原ID、created_at及deleted_at=NULL，updated_at由模型维护；并发首次撞唯一键或版本不匹配Aborted，不自动改为覆盖。写入与前后实际集合/版本/操作者/Trace审计同事务，失败整体回滚，不绑定用户角色。

## 直接按模板复制

CopyRolePermissionToUser全部仅本站唯一20操作，来源为同站点类型3普通模板；禁止默认模板、跨站点来源、自己或平台目标。显式roleVersion和用户version均必填，读取到的来源版本不匹配或目标用户CAS冲突返回Aborted。来源及两级上限在本次事务快照检查，不保证来源在目标提交时未变化。复制前过滤已删除、禁用、不存在、祖先失效及上限外权限，将剩余集合完整覆盖用户；全部失效或来源为空时明确清空。必要默认模板仍须存在，异常格式/结构、有效跨目录项及数据库错误仍拒绝。过滤不改写来源模板或来源版本，手工保存仍拒绝有效越权项。

GetRoleMenus用于模板选择预览，仅本站唯一20读取同站点类型3；返回相同规则下的有效ID和来源版本，查询不写库。例：来源保存1,2，2删除或移出上限后，选择只返回1，复制实际写入1；若1也失效，选择为空，显式复制会清空目标用户权限。来源记录仍保持1,2及原版本。缺失必要默认模板会返回错误，不能当作权限为空清空用户。复制前上限或目录若发生变化，复制仍按事务内最新读取快照重新过滤，来源version只校验来源记录，不固定上级/目录快照。

复制成功与审计同事务，来源ID/版本仅用于本次审计，不形成绑定；模板以后修改不自动影响已复制用户。只读取模板作为本地草稿并手工调整的，调用SaveUserPermission保存最终草稿，不调用复制覆盖调整。

## 操作鉴权与已配置菜单树

CheckUserPermission先验证参数、真实账号状态和ctx归属；正常平台10具有默认全操作权限，不需要个人授权或模板授予，操作权限分支与菜单显示分开。平台11逐人授权受模板1限制，20默认取得本站2与平台1全部有效权限，30/40取个人授权与两级默认交集。20还必须通过本站唯一身份检查。

除10默认身份分支外，匹配serviceName和完整path时以Go字符串精确比较，拒绝大小写/尾空格宽松比较、前缀或通配。userId来自API认证结果，serviceName来自可信配置，path来自实际路由，不按HTTP方法区分同一路径。候选及完整祖先必须存在、启用并属于共享目录；祖先无需被勾选，不自动授予后代。平台专用业务入口继续按身份及操作许可控制，不因站点用户持有某个菜单ID获得平台管理资格。

**GetMenuByAdminId对所有账号（包括10和20）都按已保存menuIds获取菜单树**，不因默认操作全权限自动返回完整目录；无配置、空配置或软删除授权返回空列表。11的菜单还受模板1限制，站点菜单受两级默认交集限制；所有节点过滤目录及祖先失效，按sort、ID升序。有效但未配置父节点的子节点可作为返回根保留MenuPid。10可复用GetUserPermissionTree/SaveUserPermission维护自身展示配置；未配置仍返回空菜单，不能把其操作默认全权当作菜单配置。

普通读取无事务、行锁或跨请求缓存，不保证账号、模板、授权、目录多个查询为同一快照。默认模板收权及目录禁用/删除影响后续实际访问，不批量删除历史CSV；恢复上限或启用后，保留授权须满足所有当前条件才再次有效。菜单树不能替代每次API操作鉴权。

## 站点20唯一性与迁移

requireUniqueSiteSuper在运行时、授权管理涉及20及模板管理时校验本站身份唯一。已执行的006以account_type20生成唯一槽位，软删除也保留名额，防止未经约定的替换创建第二位。替换、删除和恢复流程另行设计。索引只能保证最多一位；用户已确认账号创建及至少一位20由其他业务服务负责，auth-rpc不创建、补齐或升级账号。重复或缺失20身份拒绝，不自动合并或转移权限。

数据库仍为user_rpc，连接标识user-rpc；001—006已执行。2026-11-25完成005旧1→新2、旧2→新3转换及真实平台默认1唯一槽位、归属约束，完成006本站20唯一约束；相关表均为空，转换影响0行。前后结构及精确计数快照、执行与清理记录见DATABASE.md。禁止在当前库重跑；其他环境执行前仍须核对、停写并备份，DDL分阶段隐式提交。其他项目、user-rpc、JWT及外部rpc-client未改动，业务接口尚未测试或联调。

## 错误及本轮验证范围

InvalidArgument用于缺省字段、非法CSV/编码/版本/服务参数；Unauthenticated用于缺少必要metadata/UID/Trace；PermissionDenied用于身份、归属、模板资格、有效越权或无实际操作权限；FailedPrecondition用于缺失必要默认模板或严格模板写入的目录不可用；AlreadyExists用于重复默认模板；Aborted用于版本冲突；DataLoss用于损坏存储、祖先循环或站点20身份不唯一。数据库错误、取消和超时保留错误链并脱敏，不伪装为成功。

业务逻辑仍集中在既有模型，平台默认模板三个Logic入口集中在一个文件，不新建单函数辅助文件。源码阶段执行协议聚合、protoc生成、gofmt及引用核对；其后已按用户授权执行005/006并核对结构，临时迁移执行器和凭证按DATABASE.md登记清理。未运行Go测试、构建、示例、服务或业务联调，未新增测试用例、样本或缓存，原有测试保留。005/006和前后结构快照及生成源码为正式交付文件，结构核对及历史验证均不覆盖本轮新业务逻辑。

## 菜单、分组及10自身配置（最新确认）

菜单和分组维护仍仅平台有效账号10。UpdateMenu提交完整MenuItem，serviceName/path/menuType允许修改，菜单ID和已有模板/用户勾选保持，后续按新配置鉴权；不自动增加或清除授权。menuPid=0移动到根、sort=0和空path正常保存，名称/服务名必填、类型1/2、状态1/2；父级完整链必须存在、未删除、同共享目录且无循环，不允许自身或后代，目录修改使用目标version条件写入，单次请求检查完整父级链，不保证不同菜单并发改父级的串行性。

同服务精确非空path在未删除菜单中唯一，页面和按钮统一查重；禁用仍占用，空路径可重复，软删除后可使用新ID重建但不继承旧授权。AlreadyExists用于查重冲突。菜单删除不因分组引用而阻断，不改写分组/模板/用户历史CSV；子节点祖先失效后隐藏，不级联删除。分组删除只改变展示组织，未归入其他组的有效菜单进入未分组；固定ID=1保护已删除。

AddPermission的menuIds是字符串形式的正整数JSON数组，例如`"[1,2]"`，新增必须显式提交，`"[]"`为空组。UpdatePermission成员字段缺省不更新，显式`"[]"`清空；写入数组最多65535字节，去重升序，不存在/已删除成员过滤，禁用成员保留，跨目录有效成员拒绝。对象、字符串、小数、负数、0、null等格式拒绝。列表与编辑页共用解析，异常存储DataLoss，不返回上一组成员；历史SQL NULL/JSON null按空组读取。菜单/分组查询默认page=1、pageSize=20且最多200。

平台10配置自己的菜单，沿用以下请求结构，userId替换为当前已认证的10账号ID；必须是自己，不能借此配置其他10：

```json
{"userId":42,"businessCode":"99999999"}
```

上例调用GetUserPermissionTree；可选所有有效共享菜单，不受模板1范围或是否已创建限制。保存调用SaveUserPermission，示例：

```json
{"userId":42,"businessCode":"99999999","menuIds":"1,2","version":0}
```

version=0仅首次；已有/软删除记录必须传查询返回的原版本。menuIds为空字符串清空个人展示配置，缺省拒绝；按当前有效目录过滤后保存，同事务版本与成功审计，不改变10默认全操作权限，也不修改平台默认模板。11/20/30/40仍不能自改。账号创建和站点至少一位20由其他业务服务处理，auth-rpc不新增账号入口。

**源码依赖的迁移007已于2026-11-25在user_rpc执行并核对，当前库禁止重跑，执行记录及快照见DATABASE.md。** 它新增活跃路径二进制生成列及唯一索引，并建立含固定行1的permission_catalog_lock；固定行是目录互斥元数据，不是账号、模板或授权。007当时的源码依赖目录锁；本轮已按用户确认移除全部显式悲观锁及该依赖，版本列和锁表退役已随008执行并核对完成。001—006保留已执行状态，不重跑。

源码开发阶段执行`sh ./dsl/main.sh`、protoc、gofmt及引用核对，后续按授权完成007/008结构迁移；未运行Go测试、构建、示例、服务或RPC。保留原RPC方法，版本字段新增及传递见下方最新乐观锁规则。未新增测试用例、测试脚本、缓存或临时产物需清理，原有测试和用户文件保留。目录完整字段变更审计仍未实现，本轮不把既有用户授权审计扩称为目录审计。

## 全部乐观锁接入（最新规则）

菜单列表的父级筛选已支持显式根节点，见下方最新用法；与菜单写入的版本要求分别处理。

已移除整个auth-rpc的目录锁依赖及所有显式悲观行锁。模板、用户授权沿用已有version和原子条件写入，事务/唯一约束/身份检查/成功审计保留；新增创建唯一键竞争及旧版本冲突均不自动重试。账号、上限、来源与菜单链普通读取，以事务内读取快照为准，不保证跨记录提交时刻串行；并发收权后实际访问仍按当前有效上限计算。复制读取到的来源版本必须与roleVersion匹配，用户目标CAS仍须匹配version；不锁来源到提交，也不增加来源版本制造互斥。

菜单和分组新增版本字段；旧写入请求必须适配，缺省版本不能作为覆盖。仅按目标版本防并发覆盖，不使用独立锁表、固定锁行、全局版本互斥或FOR UPDATE/FOR SHARE。菜单跨记录并发改父级不作防环保证，按已确认取舍处理，单次检查及读取报错保留。

| 接口 | 版本传递 |
|---|---|
| AddMenu / AddPermission | data.version省略或0；成功返回id及version=1 |
| GetMenuList | 每条MenuItem.version返回实际版本 |
| GetPermissionList | 每条PermissionListItem.version返回实际版本 |
| UpdateMenu / UpdatePermission | data.version必须显式传已读1..MaxInt64-1，成功返回新version |
| DeleteMenu / DeletedPermission | 请求version必须显式传已读1..MaxInt64-1，成功返回新version |

修改菜单完整提交，例如：

```json
{"data":{"id":100,"menuPid":0,"menuName":"用户列表","menuType":1,"path":"/users","status":1,"sort":0,"serviceName":"admin-api","version":1}}
```

修改分组可按原语义仅提交待改字段，版本必须提交：

```json
{"data":{"id":2,"menuIds":"[100,101]","version":1}}
```

删除菜单或分组均传：

```json
{"id":100,"version":2}
```

成功响应包含新version；软删除同事务完成CAS递增和模型Delete，任一步失败回滚。目标不存在/已删除NotFound；缺省/非法版本InvalidArgument；读取不匹配或条件UPDATE受影响0行Aborted，刷新后由用户重新提交，不自动用新版本覆盖；路径唯一冲突仍为AlreadyExists。

**008_optimistic_catalog_version.sql已于2026-11-25在user_rpc执行并核对，当前库禁止重跑。** menu/menu_group默认1版本列及启用CHECK已存在，旧permission_catalog_lock已删除，007路径生成列及唯一索引保留。六张权限业务表仍为空，其他共享业务表未扩大删除。001—008不重跑；其他环境仍须先核对、停旧写入并备份后迁移，不能向旧结构部署新代码，调用方必须适配显式版本。执行记录、前后快照及清理见DATABASE.md。

本轮实际执行协议聚合、protoc、gofmt与源码/引用核对；本地Client/Server使用更新PB类型，无新增RPC方法。未测试、构建、运行示例、服务或RPC，未新增测试、脚本、验证缓存或临时产物需清理，原有测试及用户文件保留。

## 模板配置树与分组成员回显开发记录（2026-11-25）

本轮按已确认第1、2项调整：新增GetTemplatePermissionTree及协议、本地Client/Server/Logic，查询资格和上限集中在现有role_permission_model.go，分组树组装复用原Logic。默认模板1/2创建后禁止删除，现有删除分支仅允许类型3，错误及注释已统一。

GetPermissionList的成员项新增status：1启用、2禁用。维护查询按分组引用分批读取全部未删除共享菜单，不因禁用或祖先失效丢失成员；删除或不存在的引用不返回。前端保存完整成员时应保留需保留的禁用成员ID，仅改名时可省略menuIds并携带已读version；禁用成员不会进入有效授权范围。原查询/保存版本规则不变，无数据库迁移。

实际执行：`sh ./dsl/main.sh`聚合协议；`protoc --plugin=protoc-gen-go=/Users/m/go/bin/protoc-gen-go --plugin=protoc-gen-go-grpc=/Users/m/go/bin/protoc-gen-go-grpc --go_out=. --go-grpc_out=. dsl/auth_rpc.proto`生成PB及gRPC；对改动Go文件执行`gofmt -w`。这些命令已完成，另做源码与引用核对；未执行Go测试、构建、范例运行、服务联调或数据库操作，不能据此认定编译及业务验证通过。

清理情况：本轮未新增测试用例、临时脚本或验证缓存，无此类产物需要删除；保留原有测试及本轮正式Logic、协议生成文件。其他项目及外部rpc-client未同步。其余复查项不在本次改动范围。

## 配置树同步移除上限外权限（2026-11-25）

按最新确认，GetUserPermissionTree及GetTemplatePermissionTree统一只返回当前上限内有效权限，分组与未分组都过滤上限外权限和历史勾选，不再返回“已勾选、禁用”的权限项。必要祖先只为树结构保留，上限外祖先checked=false，displayOnly/disabled仅表达结构占位；不扩大父子授权。平台10维护模板1或自己的菜单展示仍可选全部有效共享目录。

查询不改写模板、用户授权、分组成员或版本；显式按当前勾选完整覆盖保存时清除旧ID。保存端的版本条件、成功审计及当前权限上限校验不变，主动提交越权ID或查询后上限变化仍可能被拒绝，需刷新后重试。分组维护列表继续保留禁用成员，不受配置树过滤影响。本轮未处理列表ID、时间范围及根菜单筛选复查项。

本轮实际执行`sh ./dsl/main.sh`、`protoc --plugin=protoc-gen-go=/Users/m/go/bin/protoc-gen-go --plugin=protoc-gen-go-grpc=/Users/m/go/bin/protoc-gen-go-grpc --go_out=. --go-grpc_out=. dsl/auth_rpc.proto`及改动Go文件的`gofmt -w`，均完成；做了源码、注释及引用核对。协议字段及编号不变，生成代码同步注释。未执行测试、构建、范例、联调或数据库操作，无需数据库迁移；不能据此认定编译或业务验证通过。

清理记录：未新增测试、临时脚本、缓存或其他验证产物，无此类文件需要清理；保留原有测试和正式生成文件。

## 菜单列表父级筛选（2026-11-25）

本轮仅处理复查第3项，第2项列表ID和时间范围筛选按用户要求跳过。

| GetMenuList请求 | 父级筛选行为 |
|---|---|
| `{}`或`{"data":{}}` | 不限制父级，沿用其他条件和分页 |
| `{"data":{"menuPid":0}}` | 只查根菜单，menu_pid=0 |
| `{"data":{"menuPid":100}}` | 只查父级100的直接子菜单，不递归 |
| `{"data":{"menuPid":-1}}` | InvalidArgument |

父级不存在时返回空结果，不将其改查根节点或全目录；total与分页列表使用相同父级条件。列表回显的menuPid始终显式赋值，根菜单也返回0。

MenuItem.menuPid改为optional int64，字段编号仍为2，Go生成字段由int64改为*int64，GetMenuPid()仍返回数值。Go调用根菜单筛选时使用`parentID := int64(0)`，然后设置`data.MenuPid = &parentID`；仅调用GetMenuPid()会丢失未传与0的区别。旧标量客户端不能可靠表达显式0，其他项目及外部rpc-client需同步生成；本轮只修改auth-rpc，不保留旧客户端显式0的兼容推断。

MenuItem也用于新增/修改，写入逻辑已显式用GetMenuPid()赋给模型；省略和显式0都按根保存，更新仍完整覆盖，不表示保留原父级，完整祖先链与版本校验继续沿用。数据库字段及其他树节点协议不变，无需迁移。

开发记录：实际执行`sh ./dsl/main.sh`、`protoc --plugin=protoc-gen-go=/Users/m/go/bin/protoc-gen-go --plugin=protoc-gen-go-grpc=/Users/m/go/bin/protoc-gen-go-grpc --go_out=. --go-grpc_out=. dsl/auth_rpc.proto`以及改动Go文件的`gofmt -w`，均完成；做源码和引用核对。未运行测试、构建、范例或服务联调，未连接数据库，不能据此认定编译及业务验证通过。未新增测试、临时脚本、验证缓存或其他临时产物，无此类文件需要清理，保留项目原有测试和正式生成文件。

## 模板选择与复制过滤统一（2026-11-25）

按用户明确确认，GetRoleMenus与CopyRolePermissionToUser共用现有role_permission_model.go中的filterTemplateMenuIDs；模板查询GetEffectiveMenuIDs先沿用FindByID校验本站唯一20、站点归属及普通模板类型，再读取必要两级默认上限。复制在原用户写事务内先比较来源roleVersion，再调用同一过滤逻辑，剩余有效集合完整覆盖目标用户，全部失效时写空CSV。无新辅助文件、无来源写入或跨记录锁。

目录有效性复用已有手工保存过滤及完整祖先检查，删除、禁用、不存在、祖先失效项过滤，再取两级上限交集；不隐式授予父子权限。必要默认模板缺失、存储格式/结构损坏、有效跨目录项及查询错误仍返回错误，不作为空集合覆盖。手工保存有效越权项仍拒绝，模板新增/修改完整集合校验、列表和详情的原始存储展示不变。

目标用户version/首次唯一约束、来源版本比较及Aborted语义继续沿用；写入失败或审计失败整体回滚。审计AfterMenuIDs记录过滤后实际写入集合，SourceTemplateID/Version保留本次来源ID及版本。查询和复制均不修改来源模板CSV、版本或审计其他用户。来源版本不固定默认上限/目录的状态，复制结果按事务内读取条件重新计算，不能把预览当作跨请求固定快照。

开发记录：实际执行`sh ./dsl/main.sh`、`protoc --plugin=protoc-gen-go=/Users/m/go/bin/protoc-gen-go --plugin=protoc-gen-go-grpc=/Users/m/go/bin/protoc-gen-go-grpc --go_out=. --go-grpc_out=. dsl/auth_rpc.proto`以及上述模型和两个Logic文件的`gofmt -w`，均完成；做源码、错误分支、调用引用和注释核对。协议字段及编号不变，仅同步行为注释；本次只改auth-rpc，无数据库迁移。未执行测试、构建、范例、联调或数据库操作，不能据此认定编译及业务验证通过。

清理记录：未新增测试、临时脚本、验证缓存或其他临时产物，无此类文件需要清理；保留项目原有测试、文件及正式生成代码。

## 业务对接清单整理（2026-11-25）

新增[业务对接清单](INTEGRATION_CHECKLIST.md)，汇总当前职责边界、身份及三级模板、RPC/Trace接入、完整接口清单、初始化顺序、页面授权与复制流程、字段及版本语义、错误处理、部署准备和待执行验收项。README增加入口并修正旧复制/鉴权描述，未改变业务行为。

本轮只整理文档并核对源码、协议及本地文档引用；未修改Go源码、协议、数据库或依赖，未执行测试、构建、生成或联调。验收清单均为待执行事项，不表示已通过。未新增测试、脚本、缓存或临时产物，无此类内容需要清理。

## 历史验证记录说明

以下记录保留实际执行命令、结果及清理状态，仅覆盖当时版本。当前类型编号、管理资格、平台默认模板与超级管理员身份区分、无操作者权限上限、编辑页隐藏和手工保存过滤均已变化，本轮未测试或构建，不能把历史结果视为新实现通过。

## 开发及验证状态

2026-11-24实现DTO、DAO、Logic、树组装、协议和本地服务端/客户端入口，并同步dsl/tpl/role.proto及聚合协议。

在auth-rpc目录实际执行：

```text
sh ./dsl/main.sh
/Users/m/go/bin/protoc --plugin=protoc-gen-go=/Users/m/go/bin/protoc-gen-go --plugin=protoc-gen-go-grpc=/Users/m/go/bin/protoc-gen-go-grpc --go_out=. --go-grpc_out=. dsl/auth_rpc.proto
gofmt -w internal/dao/dto/user_permission_tree.go internal/dao/model/mysql/admin_permission_model.go internal/logic/roleauthservice/get_user_permission_tree_logic.go internal/logic/roleauthservice/user_permission_tree.go internal/server/roleauthservice/role_auth_service_server.go client/roleauthservice/role_auth_service.go
```

协议拼接、protoc生成及gofmt均成功。生成代码标注工具版本为protoc 3.19.4、protoc-gen-go 1.36.11、protoc-gen-go-grpc 1.6.2；没有改动go.mod/go.sum、公共包或现有数据库。

本轮仅进行生成、格式化和源码核对，未运行Go构建、测试、RPC调用、服务启动或数据库联调，不能据此认定运行行为已经验证。未新增测试文件、测试脚本、依赖缓存、用户样本或临时产物，无需清理；生成的pb.go属于接口交付源码，保留。原有测试和用户文件保留。

模板CRUD当前已按menuIds、templateType和version改造，依赖新增迁移003；本接口仍通过admin_permission直接读取用户配置，不依赖角色模板继承。

模型软删除已统一为NULL约定，菜单、分组和角色的删除入口改用模型Delete；删除前的既有业务检查保持。此次只执行gofmt和源码核对，未运行构建、测试、RPC调用或数据库操作；未新增测试文件、测试脚本或临时产物，无需清理。

权限树查询已移除Transaction、只读事务选项和database/sql导入，改为请求会话上的普通查询，业务范围、模型软删除过滤、必要的Unscoped版本读取及错误返回保持。此次实际执行 `gofmt -w internal/dao/model/mysql/admin_permission_model.go internal/dao/dto/user_permission_tree.go`，并核对相关源码和说明；未运行构建、测试、服务或数据库操作，没有新增测试、脚本或临时产物，无需清理。

## 保存接口的开发及验证（历史行为）

以下为2026-11-24的实际执行记录。当前保存已改为带授权上限和审计的写事务，停用菜单保留分支已删除；本轮未测试或构建，历史结果不证明新实现通过。

2026-11-24实现SaveUserPermission的协议、本地Client/Server、Logic、模型覆盖保存及共享菜单ID解析。menuIds和version均使用proto3 optional保留字段存在性。查询与保存复用同一目标管理员及权限归属检查；原权限树的CSV解析也接入共享解析函数。直接引用已有MySQL驱动以识别1062重复键，将go.mod中mysql v1.8.1由间接依赖调整为直接依赖，没有升级版本或新增第三方库。

本次重新执行协议拼接、protoc生成和7个交付Go文件的gofmt，均成功。为验证保存行为，临时新增3个测试文件，使用标准库模拟database/sql驱动，禁止真实连接，并让任何Begin/BeginTx调用失败：

- menu_ids_task_test.go：规范化、空字符串、非法ID、越界及TEXT容量上限。
- admin_permission_save_task_test.go：首次创建、覆盖、清空、恢复、保留停用项、拒绝新增停用项、版本冲突、并发创建撞唯一键、条件更新未命中、管理员不存在、数据库读失败、归属不一致、版本溢出等18个场景。
- save_user_permission_task_test.go：缺失menuIds/version必须拒绝，显式空字符串和version=0正确区分。

在auth-rpc目录实际执行：

```text
env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/auth-rpc-save-check.s9H7JG/cache go test -count=1 -run '^TestTask' -v ./internal/dao/model ./internal/dao/model/mysql ./internal/logic/roleauthservice
env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/auth-rpc-save-check.s9H7JG/cache go build ./client/roleauthservice ./internal/server/roleauthservice
```

结果：3个包的定向测试通过，保存逻辑18个场景全部通过；本地客户端和服务端入口编译成功。使用本地Go 1.26.5、离线依赖及独立临时编译缓存，没有运行原有真实资源测试。没有启动服务或调用真实RPC，没有连接或写入业务数据库，没有执行DDL、发版或外部客户端依赖更新。

这些结果验证模拟SQL条件、版本冲突分支、字段存在性和代码编译，不代表真实MySQL并发、生产密文流程或API/前端联调已经验证。以上执行命令、结果和限制在清理前登记，接下来仅删除本次3个测试文件和独立临时编译缓存，原有测试继续保留。

清理结果：本次3个测试文件及独立临时编译缓存均已删除，清理后逐项核对均不存在。原有internal/logic/test/m_test.go及用户其他文件保留；生成的protobuf源码和实现文件作为交付内容保留。

## 文件合并验证

2026-11-24按确认将同一业务的中间逻辑合并到上方所列模型及Logic文件，移除4个拆分辅助文件，并更新引用及中文注释。只调整代码组织，没有改动协议、数据库、依赖或保存/查询行为。

实际执行两个修改文件的gofmt，以及：

```text
env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/auth-rpc-merge-check.HmhTLJ/cache go build ./internal/dao/model/mysql ./internal/logic/roleauthservice ./client/roleauthservice ./internal/server/roleauthservice
```

模型、Logic、本地客户端及服务端入口均编译成功。仅执行离线编译和源码引用核对，未新增或运行测试，未启动服务或连接业务数据库。先登记命令、结果及限制，再删除本次独立临时编译缓存；原有测试和业务文件保留。

合并清理结果：4个拆分辅助文件及本轮临时编译缓存均已删除，逐项核对不存在；未新增测试文件或测试脚本。

## 旧角色鉴权接口删除状态

已删除CheckAdminRoleMenu的协议请求/响应及服务方法、Logic实现、服务端转发和本地客户端入口；重新拼接dsl/auth_rpc.proto并生成两份pb.go。按源码及协议全文核对，auth-rpc已无旧接口引用。旧dsl/rpc.proto原本没有此接口，未作无关修改。

项目原有m_test.go文件保留，其中TestTime123仅测试已移除接口，已移除旧调用并改为明确Skip说明接口退役，不代表该测试通过。其他真实资源测试保留且未运行，不为保留旧测试而恢复旧接口。

实际执行协议拼接、protoc生成、3个修改文件gofmt，以及：

```text
env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/auth-rpc-remove-role-check.BfIJ69/cache go build ./...
```

结果：auth-rpc全部非测试包编译成功。未运行测试、服务、真实RPC、数据库操作，未发布外部客户端。编译成功不代表用户鉴权已实现或user-rpc完整登录链路可用。先记录命令、结果和限制，再删除本次临时编译缓存；无新增测试文件或测试脚本。

删除接口后的清理结果：本次独立临时编译缓存已删除，核对不存在；旧接口实现文件已删除，未新增测试文件或脚本。原有测试文件保留。

## 用户直接鉴权验证（历史行为）

以下为2026-11-24旧版直接授权行为的历史验证。本轮根交集、平台身份和完整祖先链实现未运行测试或构建，不能沿用旧结果认定新行为通过。

2026-11-24新增CheckUserPermission协议及单一Logic入口，数据库查询集中到现有admin_permission_model.go，未再拆分校验辅助文件。同步本地Client/Server，重新执行dsl拼接、protoc生成及4个修改Go文件的gofmt，均成功；未更改依赖、数据库或RPC中间件。

临时新增2个测试文件，以标准库模拟database/sql驱动，禁止真实连接和事务；模拟器同时核对查询限定用户/业务、模型软删除、菜单启用状态和页面/按钮类型，并禁止查询角色、权限分组和密码。

实际执行：

```text
env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/auth-rpc-user-check.vhsnUV/cache go test -count=1 -run '^TestTaskCheck' -v ./internal/dao/model/mysql ./internal/logic/roleauthservice
env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/auth-rpc-user-check.vhsnUV/cache go build ./...
```

结果：2个包的3个顶层测试通过，包含17个鉴权场景，以及非法参数、请求取消、构造错误传递、新方法存在及旧方法不存在的协议检查。17个场景包括明确授权放行、无有效用户或授权、停用用户、空/非法权限、无有效菜单、路径前缀/大小写/尾空格不能误匹配、重复路径候选中的精确匹配，以及三个查询阶段的数据库故障。auth-rpc全部非测试包编译成功。

仅进行了离线SQL模拟、协议及编译检查，未运行原有真实资源测试、启动服务、调用真实RPC或连接/写入业务数据库；模拟结果不替代MySQL实际数据、并发及外部调用方联调。先登记命令、结果和限制，再删除本次两个check_user_permission_task_test.go及独立临时编译缓存；原有测试文件保留。

清理结果：本次2个临时测试文件及独立编译缓存均已删除，逐项核对不存在；原有测试及用户文件保留，未新增验证数据或数据库导出。

## 菜单服务名开发与本轮验证记录

2026-11-24新增menu.service_name，菜单新增/修改、查询回显、权限树及CheckUserPermission均同步serviceName。迁移文件002及结构快照保留，执行前确认menu为空，执行后字段、组合索引及COUNT核对成功；没有写入业务记录，数据库命令及结果见DATABASE.md。

重新执行协议拼接、protoc生成及8个修改Go文件的gofmt，均成功。临时新增两个menu_service_task_test.go，以标准库模拟SQL检查服务范围、增改写入和相同路径不同服务，Logic用例检查必填校验和列表/树回显。

实际执行：

```text
env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/auth-rpc-service-name.OfjpMp/cache go test -count=1 -run '^TestTask(MenuService|Service)' -v ./internal/dao/model/mysql ./internal/logic/roleauthservice
env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/private/tmp/auth-rpc-service-name.OfjpMp/cache go build ./...
```

结果：2个包的4个顶层测试通过，其中鉴权6个子场景、菜单增改及重复检查5个子场景通过；必填长度及空白校验、菜单字段映射和权限树回显通过。auth-rpc全部非测试包编译成功。模拟结果不代表真实RPC、API或外部客户端已联调；没有执行原有真实资源测试或插入验证数据。

以上测试及编译均在用户提出“以后先开发，确认测试后再测试”之前完成，此后不再追加测试或构建。后续任务遵循新的全局规则，用户确认开发不自动授权测试。本次先登记命令、结果及限制，再删除两个临时测试文件、临时迁移执行脚本和独立编译缓存；原有测试、迁移SQL及必要结构快照保留。

清理结果：本次两个临时测试文件、临时迁移执行脚本、临时凭证和独立编译缓存均已删除，逐项核对不存在；原有测试及用户文件保留。后续测试等待用户另行确认。


### 2026-11-25 菜单与分组维护资格收口

用户确认菜单及分组新增、修改、删除仅平台超级管理员10可操作。本次对应AddMenu/UpdateMenu/DeleteMenu、AddPermission/UpdatePermission/DeletedPermission六个RPC，在既有模型集中调用withPlatformCatalogWrite：校验ctx平台归属、有效操作者及Trace，从数据库读取正常且未删除账号的真实account_type，并要求为10；同一写事务持有操作者行锁，资格不由请求体账号类型、个人授权CSV或默认模板授予。11/20/30/40均不能通过维护资格。

菜单和分组新增固定business_code=99999999，分组可省略请求businessCode，但显式其他归属拒绝；更新/删除限定平台共享记录，归属不转移，缺失、已删除或非平台目标返回NotFound。分组新增拒绝请求自定ID，分组业务字段仍沿用原更新语义；移除未被调用且未受保护的PermissionMenuModel.UpdateAll旁路。此段为第1项完成时的历史记录；本轮后续逐项确认已取消ID=1保护及菜单分组引用删除阻断，成员解析规则见下方最新说明。

读取、授权勾选、模板管理、运行时权限检查及菜单树的既有规则未修改。第1项当时未涉及完整目录审计、成员校验、防环、零值和并发约束；后续本轮已确认关键字段允许修改并保留原授权，其他确认项见最新说明。第1项无需迁移；新增的007仅准备，未执行。

本轮仅执行修改Go文件的gofmt格式化和源码/引用核对，未运行测试、构建、示例、服务、RPC或连接数据库。未新增测试文件、测试脚本、缓存或临时产物，无需清理；原有测试与用户文件保留，业务验证仍等待用户另行确认。


### 2026-11-25 迁移007执行补记

用户明确授权后执行007，三个SQL步骤成功：活跃路径生成列及唯一索引、目录锁表、唯一id=1元数据行。六张业务表仍为空，原菜单字段/索引及五张非目标表结构保持，前后结构和计数快照已保留。未运行Go测试、构建、示例、服务、RPC或并发业务验证；执行命令、初次表达式文本核对差异及最终结果、临时凭证/执行器清理记录见DATABASE.md。历史段落中的“仅准备”只描述其当时状态。

执行007后用户已明确确认将auth-rpc全部显式悲观锁替换为version；当前源码已替换全部显式悲观锁，008已执行并核对，实际旧锁表已删除且两个版本列及范围CHECK已落库。007的执行及快照仍是历史事实，不重跑或修改其DDL。


### 2026-11-25 迁移008及无用表清理执行补记

用户授权后008三个阶段均成功；menu和menu_group新增version（BIGINT UNSIGNED、NOT NULL、DEFAULT 1）及启用范围CHECK，退役的permission_catalog_lock已删除，原路径生成列/唯一索引及其他字段保持。六张权限业务表前后精确COUNT均为0，库表清单从20到19，差异仅旧目录锁表；其他共享业务表未认定废弃，保留。前后结构、空数据状态及旧锁表唯一元数据已备份，执行/核对命令和临时文件清理见DATABASE.md。未运行Go测试、构建、服务、RPC、并发测试或写入业务样本，迁移成功不替代业务验证。
