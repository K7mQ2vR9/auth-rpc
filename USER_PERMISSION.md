# 用户独立权限与角色模板接入

当前已实现授权编辑页查询GetUserPermissionTree、覆盖保存SaveUserPermission、实际鉴权CheckUserPermission及改为独立授权的GetMenuByAdminId。角色模板CRUD已改为同站点menuIds、templateType、version结构，迁移003已于2026-11-25在user_rpc执行；平台根模板管理入口、按模板直接覆盖用户及其他项目调用迁移尚未完成。用户权限仍独立保存，不持续绑定角色。

旧按角色鉴权的CheckAdminRoleMenu已从协议、生成代码、本地Client/Server及Logic删除，不保留兼容分支。CheckUserPermission已实现并接入本地Client/Server。user-rpc中的parse_admin_token_logic.go仍依赖外部rpc-client并调用旧方法，需要同步客户端并迁移调用；本次未修改该项目，也未关闭其权限检查。

## 当前代码组织

同一业务的中间逻辑集中维护，保留模型层和每个RPC的Logic入口：

| 文件 | 集中的内容 |
|---|---|
| internal/dao/model/mysql/admin_permission_model.go | 用户权限查询、保存、版本校验、菜单ID解析及规范化、登录菜单及路径鉴权共用的有效用户授权读取 |
| internal/dao/model/mysql/menu_model.go | 菜单读写及授权编辑树、登录菜单树共用的父级循环校验 |
| internal/dao/model/mysql/role_permission_model.go | 同站点模板查询、保存、根权限子集及菜单校验、版本比较与模型软删除；不展开分组 |
| internal/logic/roleauthservice/get_user_permission_tree_logic.go | 查询RPC入口、勾选状态及分组菜单树组装 |
| internal/logic/roleauthservice/save_user_permission_logic.go | 保存RPC参数校验及模型调用 |
| internal/logic/roleauthservice/check_user_permission_logic.go | 鉴权RPC参数校验及模型调用；数据库鉴权集中在admin_permission_model.go |
| internal/logic/roleauthservice/get_menu_by_admin_id_logic.go | 登录菜单RPC参数校验、用户独立菜单查询及有序菜单树组装 |

已删除原来单独拆出的admin_permission_save.go、menu_ids.go、dto/user_permission_tree.go和roleauthservice/user_permission_tree.go。schema字段映射及protobuf生成文件仍按现有项目结构维护。合并只调整文件归属和包内引用，接口、数据库及业务行为保持；下方此前的验证命令保留当时实际路径。

## 用户角色解绑与登录菜单

完整RPC方法：`/rpc.RoleAuthService/getMenuByAdminId`。请求继续使用adminId，响应结构不变，保留服务名及children，不再要求用户绑定角色。

参数adminId须为正数，business-code读取现有上下文。模型校验目标管理员属于当前业务、未删除且正常，然后读取未删除的admin_permission；无记录、权限已软删除或空集合均返回空菜单列表。停用、已删除或跨业务管理员返回PermissionDenied；非法CSV返回DataLoss，数据库故障保留错误链，不伪装成空列表。缺少business-code返回Unauthenticated。此业务查询不改变RPC中间件仅做metadata准入的职责。

菜单只查询该用户明确获授、属于当前业务、未删除、自身启用的页面或按钮，按sort、ID升序组装。不会因页面授权自动拥有按钮，也不加载角色、分组或管理员密码。父节点不在获授集合时，当前节点放入返回的根列表，MenuPid保持原值；不额外查询或补授父节点。集合内自身引用、负父ID、祖先循环或无效目录节点返回DataLoss，避免生成不可序列化的循环树。

auth-rpc的AdminInfo.RoleId及按role_id筛选已删除；角色删除入口不再检查是否有用户绑定该角色，也不修改用户独立权限。旧admin_role.go、GetRolePermissionById和未使用的LoginRoleMenuItem已删除；菜单树辅助逻辑留在对应Logic，共用父级校验放在现有菜单模型文件。

角色模板随后已完成下节所述结构改造，DeleteRole改为按templateType保护站点根模板，不再使用旧超级角色字段。登录菜单和路径鉴权仍限定当前业务，站点根模板上限、共享目录、完整祖先停用校验及平台跨站点身份分支尚未接入这两个用户查询入口。

用户解绑阶段仅修改auth-rpc源码及文档，未修改数据库、user-rpc、JWT或后台，用户菜单RPC请求和响应字段保持；模板协议的后续变化见下节。解绑阶段只进行源码引用核对和gofmt格式化，未运行测试、构建、服务或联调，未新增测试用例、脚本、缓存或临时产物，无需清理；原有测试保留。

## 权限模板接口

以下为本地源码目标行为。[迁移003](database/migrations/003_role_template_type_and_version.sql)已于2026-11-25在user_rpc执行并核对新列、唯一索引和CHECK，角色表仍为0条。当前库不可重跑，其他环境先核对实际结构；根模板不由迁移自动创建。未进行业务调用、测试或构建，结构成功不等于接口已联调通过。

RoleItem不再包含旧权限组或超级角色字段，新增menuIds、templateType、version；模板版本不是账号身份，account_type现有五种定义保持不变。RoleListItem直接返回具体menuIds字符串、类型、版本、名称及站点，不再展开权限组对象。已删除旧初始化RPC及其请求/响应、Logic、Client和Server入口，也删除了无人使用的dsl/rpc.proto副本；协议源为dsl/tpl/role.proto，聚合生成到dsl/auth_rpc.proto。

所有模板操作严格限定ctx中的business-code；请求体businessCode可缺省，显式传入必须与ctx一致。平台业务编码也不自动取消过滤，不截取编码、不推导商户上级、不修改输入请求。普通写接口只维护templateType=2的自定义模板，templateType=1的站点根模板仅可读取，不能通过这些接口新增、修改、删除或转移归属。

| RPC | 请求及行为 | 响应 |
|---|---|---|
| AddRole | data.roleName、显式data.menuIds、templateType=2；id必须为0，version可省略或传0 | 新模板id、version=1 |
| UpdateRole | 正数id、完整roleName、显式menuIds、templateType=2及显式已读正数version；空menuIds清空，缺省拒绝 | 新version |
| DeleteRole | 正数id及显式已读正数version；只软删除同站点自定义模板 | 空响应 |
| GetOneRole | 正数id；普通查询当前站点未删除模板，不存在或跨站点返回NotFound | 模板完整快照 |
| GetRoleMenus | 正数id，businessCode可省略；直接解析模板逗号集合，不读取分组，不保存用户权限 | 菜单ID数组及来源version |
| GetRoleList | data支持id、roleName包含、templateType筛选及一致businessCode；STime/ETime为创建时间Unix秒闭区间，0不限制 | 模板快照列表、total及页码 |

名称必填、最多255个UTF-8字符且不能含首尾空白。menuIds使用与用户授权相同的正数ID解析、去重和排序规则，规范化后最多65535字节；空字符串明确表示空模板。修改、删除的version须为1..MaxInt64-1，冲突返回Aborted且不自动重试覆盖。删除在短事务内先比较并递增版本，再使用模型Delete维护deleted_at和updated_at，任一步失败均回滚，不手写软删除时间、不修改用户权限。

新增、修改在短写事务中先锁当前站点唯一有效根模板；缺失返回FailedPrecondition，重复根或损坏的根类型、版本及菜单格式返回数据异常，不从固定分组、自定义模板或全部菜单推导上限。所选集合必须完整包含在当前根权限内，任一越界项整体返回PermissionDenied。即使新增空模板或清空已有模板，也要求根模板存在且有效。

通过根集合检查后，按具体ID查询共享菜单，菜单必须未删除、自身启用且类型为页面或按钮；不会为本站复制新菜单，也不会根据菜单分组扩大授权。修改再锁定目标记录，核对旧版本后完整覆盖名称、menuIds并递增版本；所有数据库操作使用同一tx。保存顺序为根记录、目标模板、按ID排序的菜单记录；新增没有目标旧记录。普通读取不额外开启事务，模板列表默认page=1、pageSize=20，每页最多200；负数、溢出分页或无效时间范围返回InvalidArgument。

模板读取返回其保存快照及版本，不根据后来缩小的根集合清理或改写历史ID。模板修改或删除不联动已复制用户；根权限变化不触发本轮自动裁剪。普通选择器应使用templateType=2且当前站点范围，避免将根模板当作用户复制来源；GetRoleMenus读取本身不等于直接向用户授权，CopyRolePermissionToUser仍未实现。

新增示例（仅说明请求，不执行调用）：

```json
{
  "data": {
    "roleName": "客服模板",
    "menuIds": "1,2",
    "templateType": 2
  }
}
```

修改示例（完整覆盖为空模板）：

```json
{
  "data": {
    "id": 10,
    "roleName": "客服模板",
    "menuIds": "",
    "templateType": 2,
    "version": 1
  }
}
```

删除提交 `{"id":10,"version":2}`。GetRoleMenus返回的version用于后续来源模板比较，不能代替目标用户权限版本。

本阶段完成模板类型、站点边界、根集合及自身启用菜单检查和版本写入。API仍须执行身份及操作资格校验；模板写入的操作者自身权限上限、完整祖先停用校验、同事务审计、平台根模板专用管理和跨站点管理入口尚未接入，不能认定完整三级权限已具备上线条件。根模板未配置时新增、修改会拒绝，本轮不插入占位根模板或旧超级角色。

实际只执行协议聚合、代码生成及格式化，协议生成命令为：

```text
sh ./dsl/main.sh
protoc --plugin=protoc-gen-go=/Users/m/go/bin/protoc-gen-go --plugin=protoc-gen-go-grpc=/Users/m/go/bin/protoc-gen-go-grpc --go_out=. --go-grpc_out=. dsl/auth_rpc.proto
```

生成命令成功返回，只说明协议源码已生成，不代表通过编译或业务验证。源码阶段未运行测试、构建、示例、服务或数据库迁移；用户另行确认后已执行003，过程见DATABASE.md，未运行Go测试、构建或业务联调。迁移SQL、结构快照及生成源码作为正式交付保留，原有测试保留；未执行向外部rpc-client复制的Makefile目标，未修改其他项目。

## 实际请求权限校验

完整RPC方法：`/rpc.RoleAuthService/checkUserPermission`。请求为userId、serviceName、path，成功返回空响应，无权限返回PermissionDenied。userId必须来自API已认证的管理员身份，不能直接使用前端任意提交的用户ID；serviceName来自API自身服务配置，不直接采信前端传值；path使用当前路由/请求完整路径，不包含查询参数。

按当前请求business-code依次执行三个普通模型查询：

1. 管理员存在、未删除、属于当前业务且status=1；只读取id和status，不读取密码、角色或个人资料。
2. admin_permission中存在该用户及业务的未删除权限记录；按完整int64解析逗号菜单IDs，空权限拒绝。实际鉴权不使用Unscoped，不恢复已删除权限。
3. 仅在用户明确保存的菜单ID中查询当前业务未删除、status=1、menu_type为页面或按钮、service_name及path匹配的菜单；最终以Go字符串精确匹配服务名和路径，避免数据库排序规则对大小写或尾空格的宽松比较造成误放行。同一路径存在于不同服务时，不能互相替代授权。

不读取role_permission、权限分组、admin_info.role_id或模板，不按页面继承按钮、不补授父子节点，不提供超级角色绕过、前缀匹配、通配符、大小写别名、空服务名兜底或默认放行。管理员、权限记录、菜单不存在、已删除、停用、不属于当前业务或请求服务不匹配时均拒绝。userId非法、path为空白或serviceName缺失/非法返回InvalidArgument，business-code缺失由现有入口或模型拒绝。存储菜单格式异常返回DataLoss，数据库故障单独返回并由公共中间件转换为内部错误；取消及超时保留错误链。

本接口不修改RPC中间件的metadata/Trace准入规则，不开启事务，不使用权限缓存。查询间不保证同一快照，不能把它描述成跨表原子鉴权；后续若增加缓存或跨业务管理，需要另行明确失效及授权规则。

```go
_, err := authClient.CheckUserPermission(ctx, &roleauthservice.CheckUserPermissionReq{
    UserId: authenticatedAdminID,
    ServiceName: configuredAPIServiceName,
    Path: requestPath,
})
if err != nil {
    return err // 无权限、内部故障及取消/超时均不放行业务操作。
}
```

本地接口及实现完成不代表外部rpc-client或user-rpc调用已经更新。原有GetMenuByAdminId仍是待重构的旧角色链；新鉴权不依赖它。

## 菜单所属API服务

menu.service_name为VARCHAR(64) NOT NULL，utf8mb4_bin排序，未配置默认服务；菜单新增、修改及CheckUserPermission均必须显式传入对应serviceName。允许1至64个有效UTF-8字符，拒绝首尾空白，不自动转小写或修剪。菜单增改的Data不能为空，修改须提供正数菜单ID。

路径重复检查限定同一business_code、service_name和完整path，更新排除自身ID。不同服务可以拥有相同路径，空路径目录不参与路径重复检查。检查使用普通查询，数据库idx_service_path是查询索引而非唯一约束，本次不宣称可以防止并发新增相同菜单。

MenuItem、GetMenuByAdminIdItem、UserPermissionMenu及本地树DTO均包含serviceName：菜单列表通过字段映射回显，用户权限树直接回显；列表Data.serviceName可用于筛选。新增与修改的服务校验、父菜单/目标业务范围校验及路径重复检查集中在MenuModel，不再各自在Logic按全局路径检查。修改不会覆盖ID、创建时间、业务归属或软删除列。

当前只修改auth-rpc协议及本地Client/Server，调用方必须同步传入serviceName；不提供旧的仅userId/path鉴权降级。已有原始数据导入或其他服务直写menu时，也必须填入真实所属服务，不能使用空字符串、任意统一默认值或从用户角色推导。

## 保存用户权限

完整RPC方法：`/rpc.RoleAuthService/saveUserPermission`。请求包含userId、menuIds、version；响应只返回成功保存后的version。业务范围从当前ctx的business-code读取，目标管理员校验与查询接口一致，API仍负责操作者的管理许可。

| 请求字段 | 规则 |
|---|---|
| userId | 必填正数，对应admin_info.id，目标管理员未删除且属于当前业务 |
| menuIds | optional string，必须显式提供；空字符串明确清空，缺省拒绝 |
| version | optional int64，必须显式提供；0表示首次保存，已有或软删除记录提交查询返回的实际版本 |

menuIds按英文逗号解析正数ID，允许单项前后空白、前导零及重复项，保存时统一去空白、去重、按数值升序。例如 `2,001,2` 保存为 `1,2`。空项、非数字、正负号、零、int64越界或JSON数组格式拒绝；规范化结果超过TEXT的65535字节上限也拒绝，不截断。

保存前查询本业务未删除的菜单，传入的每个ID都必须存在。停用菜单仅允许保留该用户原有的有效授权，不允许新增授予；软删除权限的旧菜单不作为保留依据。空集合可直接清空权限，仍须通过管理员和版本校验。菜单组只影响展示，不参与此次授权计算。

| 保存状态 | 写入行为 |
|---|---|
| 无记录且version=0 | 单条INSERT，创建version=1的独立记录；空权限也会创建记录 |
| 已有记录且版本匹配 | 按user_id、business_code和原version执行单条UPDATE，完整覆盖menu_ids并令version加1 |
| 软删除记录且版本匹配 | 更新原记录并通过模型显式恢复deleted_at=NULL，保留created_at，不新增第二条 |
| 首次保存并发撞唯一键 | 返回Aborted，不把INSERT改成覆盖更新 |
| 版本不匹配或条件更新影响0行 | 返回Aborted，提示刷新后重试，不自动重读并覆盖 |

version不得为负数或MaxInt64，避免递增越界。即使内容未变化，成功保存仍递增版本，返回值用于前端下一次提交；UpdatedAt由模型维护。普通查询后只执行一次写入，写入会话显式SkipDefaultTransaction=true，不修改共享配置，也不开启额外事务。

版本条件保护用户权限记录本身；菜单与管理员校验是普通查询，保存流程不承诺跨表一致快照。菜单之后发生停用/删除时，后续实际鉴权仍须检查最新有效状态。数据库查询或写入错误直接返回，不能伪装成空权限或保存成功。

模板使用方式仍为：读取模板→覆盖页面勾选草稿→可继续手工调整→点击保存后调用本接口。请求不接收roleId或追加模式，模板变化不会自动修改用户记录。

保存示例（仅展示调用方式，不写入真实验证数据）：

```go
saved, err := authClient.SaveUserPermission(ctx, &roleauthservice.SaveUserPermissionReq{
    UserId: 1001,
    MenuIds: proto.String("1,2,5"),
    Version: proto.Int64(tree.Version),
})
if err != nil {
    return err
}
// 更新页面保存的版本为saved.Version；明确清空时使用proto.String("")。
```

proto来自google.golang.org/protobuf/proto。缺省menuIds/version、非法菜单或新增授予停用项返回InvalidArgument；目标管理员不可见返回NotFound；权限归属不一致返回FailedPrecondition；版本冲突返回Aborted，公共错误转换恢复为409业务码。数据库故障和内部异常由现有公共中间件脱敏。

## 查询接口与范围

完整RPC方法：`/rpc.RoleAuthService/getUserPermissionTree`。

请求只有userId，必须为正数，对应admin_info.id。调用ctx沿用现有metadata和合法W3C Trace；business-code读取当前请求，不接受请求体覆盖。

本接口严格限定当前business-code：目标管理员、菜单分组和菜单必须属于该业务。没有自动开放平台跨业务查询、固定分组id=1或角色继承。管理员未删除即可用于权限编辑，不因停用而隐藏其配置；停用管理员能否实际操作仍由认证和实际鉴权流程决定。API负责确认操作者是否可管理该用户，RPC查询目标用户不是RPC登录认证。

响应字段：

| 字段 | 用途 |
|---|---|
| userId | 目标管理员ID |
| version | 无权限记录时为0；已有记录返回实际版本 |
| groups | 全部分组和各自完整菜单树，不按用户已有权限裁剪 |
| groups[].id | 分组ID；0为未分组菜单的虚拟分组 |
| groups[].permissionName | 分组名称 |
| groups[].menus | 菜单节点及children |

节点保留id、menuPid、menuName、menuType、path、status和sort，并增加：

- checked：该具体菜单ID存在于用户已保存权限中时为true。未保存或权限已软删除时全部为false；软删除记录仍返回原version，方便后续恢复时比较。
- disabled：节点停用或仅补齐祖先结构时为true。停用但曾获授权的节点可以同时checked=true、disabled=true，以保留真实配置状态；这不代表实际接口允许访问。
- displayOnly：该节点仅用于补齐本组父级结构，不是本组直接成员，前端不能通过它自动授予权限。
- children：父子菜单树，节点按sort升序，同值按ID升序。分组按ID升序，虚拟未分组放最后。

没有单独的已选权限汇总字段。分组全选/半选由前端根据直接可选项目计算，跨组出现的同一菜单按id同步状态、保存时去重。checkbox应允许页面和按钮分别选择，父节点或分组状态不能自动变成隐含授权。

## 数据读取与异常

依次执行普通查询：校验目标管理员→读取用户权限及版本→读取全部分组→读取全部未删除菜单，不开启事务。所有查询继承请求ctx，但不保证使用同一数据库快照。管理员查询仅取id，不载入密码或其他个人资料；version保留供后续保存时做并发比较。

用户admin_permission.menu_ids按英文逗号分隔正数ID解析；分组menu_group.menu_ids继续按JSON数组解析，两种格式不混用。空用户字符串表示无权限，空分组或数据库NULL表示空组；非法CSV、JSON结构或非正数成员ID返回错误，不伪装成空授权。

菜单中已删除或不在当前业务范围的记录不返回，过期分组引用不制造虚假节点。存在但尚未分组的菜单放入“未分组”，保证完整目录可以展示。只引用子菜单时补齐当前业务中可见的祖先，并标记displayOnly；父级不可见时保留子菜单作为根，不读取其他业务目录。检测到自身引用或父级循环时返回错误。

管理员、分组和菜单查询通过Model指定模型，由统一的soft_delete.DeletedAt及default:null作用域自动过滤已删除记录，DAO不手写deleted_at条件。用户权限为保留软删除记录版本，显式使用Unscoped查询；这不把已删除权限恢复为勾选，也不读取role_permission或admin_info.role_id。

| 情况 | 返回规则 |
|---|---|
| userId缺失或非正数 | InvalidArgument |
| business-code空白或缺失 | 按RPC入口/查询校验拒绝 |
| 管理员不存在、已删除或业务不匹配 | NotFound，不返回其他用户目录 |
| 没有权限记录 | 完整目录、全部未勾选、version=0 |
| 权限记录软删除 | 完整目录、全部未勾选、保留原版本 |
| 无分组、无菜单 | 对应空集合，不创建默认数据 |
| 权限记录业务归属不一致 | FailedPrecondition |
| 权限版本异常、存储格式异常、菜单循环 | DataLoss；公共错误中间件按内部异常脱敏 |
| 数据库失败、取消或超时 | 返回错误，沿用公共中间件错误分类 |

## 客户端示例

在调用方已持有当前auth-rpc客户端、有效ctx和操作者管理许可时：

```go
resp, err := authClient.GetUserPermissionTree(ctx, &roleauthservice.GetUserPermissionTreeReq{
    UserId: 1001,
})
if err != nil {
    return err
}
// 用resp.Groups渲染分组与勾选状态，保留resp.Version供后续保存时提交。
```

1001只是示意ID，本次没有向真实数据库插入该用户或任何验证数据。服务端和本地client/roleauthservice已接入新方法；外部客户端仓库和user-rpc尚未同步，不将本地协议修改视作外部依赖已发布。

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

## 保存接口的开发及验证

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

## 用户直接鉴权验证

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
