package schema

import "gorm.io/plugin/soft_delete"

// AdminPermission 映射管理员独立权限，一名管理员只保存一条记录，不绑定角色或权限分组。
// 本结构仅定义字段映射，菜单校验、业务范围及版本比较由后续读写逻辑处理。
type AdminPermission struct {
	// ID 为数据库自增主键；对应BIGINT UNSIGNED，应用使用时须限制在int64正数范围内。
	ID int64 `gorm:"column:id;primaryKey;autoIncrement"`
	// UserId 对应admin_info.id，必须为正数；唯一键不包含软删除字段，重新授权时复用原记录。
	UserId int64 `gorm:"column:user_id;not null;uniqueIndex:uniq_user_id"`
	// MenuIds 保存英文逗号分隔的具体菜单ID，空字符串表示无权限；保存前须校验、去重并排序。
	MenuIds string `gorm:"column:menu_ids;type:text;not null"`
	// BusinessCode 为权限记录的业务归属，默认空字符串；实际归属由业务写入，不从角色推导。
	BusinessCode string `gorm:"column:business_code;size:10;not null;default:'';index:idx_business_code"`
	// Version 默认1；覆盖保存时须由读写逻辑校验并递增，本模型不会自动实施并发控制。
	Version int64 `gorm:"column:version;not null;default:1"`
	// CreatedAt 为创建时间，单位Unix秒；零值在GORM常规创建时自动填写。
	CreatedAt int64 `gorm:"column:created_at;not null;autoCreateTime"`
	// UpdatedAt 为最后修改时间，单位Unix秒；GORM常规创建、更新时自动填写。
	UpdatedAt int64 `gorm:"column:updated_at;not null;autoUpdateTime"`
	// DeletedAt 为软删除时间，单位Unix秒；Go零值对应未删除NULL，模型自动过滤并在Delete时同步UpdatedAt。
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;default:null;softDelete:DeletedAtField:UpdatedAt"`
}

// TableName 指定用户独立权限的数据库表名，不依赖GORM复数命名规则。
func (AdminPermission) TableName() string {
	return "admin_permission"
}
