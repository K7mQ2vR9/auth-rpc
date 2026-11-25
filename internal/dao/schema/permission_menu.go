package schema

import "gorm.io/plugin/soft_delete"

type PermissionMenu struct {
	ID             int64  `gorm:"column:id"`
	PermissionName string `gorm:"column:permission_name"`
	// MenuIds 为正整数JSON数组；写入去重，缺失/已删除成员过滤，禁用成员保留，仅用于分组展示。
	MenuIds   string `gorm:"column:menu_ids"`
	CreatedAt int64  `gorm:"column:created_at"`
	UpdatedAt int64  `gorm:"column:updated_at"`
	// Version 为单条记录乐观锁版本，初始1；修改/软删除必须按旧版本条件写入并递增。
	Version int64 `gorm:"column:version;not null;default:1"`
	// DeletedAt 未删除时数据库为NULL，删除时由模型写入Unix秒并同步UpdatedAt。
	DeletedAt    soft_delete.DeletedAt `gorm:"column:deleted_at;default:null;softDelete:DeletedAtField:UpdatedAt"`
	BusinessCode string                `gorm:"column:business_code"`
}

func (u PermissionMenu) TableName() string {
	return "menu_group"
}
