package schema

// PermissionAudit 保存已提交的权限变更快照，与对应权限写入使用同一数据库事务。
// 审计不绑定用户和角色，不提供软删除或自动清理；操作者资料只取可信身份所需字段，不保存凭证。
type PermissionAudit struct {
	// ID 为审计自增主键，应用限制在int64正数范围内。
	ID int64 `gorm:"column:id;primaryKey;autoIncrement"`
	// Operation 为服务端定义的操作名，最长64字符，不接受前端任意指定。
	Operation string `gorm:"column:operation;size:64;not null"`
	// OperatorID、OperatorAccountType、OperatorBusinessCode 为事务内查询到的操作者身份快照。
	OperatorID           int64  `gorm:"column:operator_id;not null"`
	OperatorAccountType  int    `gorm:"column:operator_account_type;not null"`
	OperatorBusinessCode string `gorm:"column:operator_business_code;size:10;not null"`
	// BusinessCode 为本次变更的目标站点，独立于平台操作者归属。
	BusinessCode string `gorm:"column:business_code;size:10;not null"`
	// TargetType、TargetID 标识被变更实体；根模板使用模板ID，user_permission使用目标用户ID。
	TargetType string `gorm:"column:target_type;size:32;not null"`
	TargetID   int64  `gorm:"column:target_id;not null"`
	// BeforeMenuIDs、AfterMenuIDs 保存规范化的逗号权限集合；创建前为空，空字符串不表示全部权限。
	BeforeMenuIDs string `gorm:"column:before_menu_ids;type:text;not null"`
	AfterMenuIDs  string `gorm:"column:after_menu_ids;type:text;not null"`
	// BeforeVersion、AfterVersion 为目标实体前后版本；首次创建0到1，其余更新递增1，复制记录用户授权版本。
	BeforeVersion int64 `gorm:"column:before_version;not null"`
	AfterVersion  int64 `gorm:"column:after_version;not null"`
	// SourceTemplateID、SourceTemplateVersion 仅供模板复制审计，非复制操作为0，不构成持续绑定。
	SourceTemplateID      int64 `gorm:"column:source_template_id;not null;default:0"`
	SourceTemplateVersion int64 `gorm:"column:source_template_version;not null;default:0"`
	// TraceID 复用当前合法链路的32位小写十六进制ID，不单独生成或接受请求体覆盖。
	TraceID string `gorm:"column:trace_id;type:char(32);not null"`
	// CreatedAt 为Unix秒级记录时间，由模型创建时填写。
	CreatedAt int64 `gorm:"column:created_at;not null;autoCreateTime"`
}

// TableName 指定权限审计表名，避免依赖GORM复数命名约定。
func (PermissionAudit) TableName() string {
	return "permission_audit"
}

const (
	PermissionAuditCreateSiteRootTemplate = "create_site_root_template"    // 平台显式创建站点根模板的成功变更。
	PermissionAuditUpdateSiteRootTemplate = "update_site_root_template"    // 平台完整覆盖根权限，记录前后版本，不联动下级。
	PermissionAuditCopyRoleToUser         = "copy_role_permission_to_user" // 自定义模板覆盖用户独立授权，记录来源快照版本。
	PermissionAuditTargetSiteRootTemplate = "site_root_template"           // 审计目标为站点根模板，TargetID对应其主键。
	PermissionAuditTargetUserPermission   = "user_permission"              // 审计目标为用户独立授权，TargetID对应目标用户ID。
)
