package schema

import "gorm.io/plugin/soft_delete"

type PermissionMenu struct {
	ID             int64  `gorm:"column:id"`
	PermissionName string `gorm:"column:permission_name"`
	MenuIds        string `gorm:"column:menu_ids"`
	CreatedAt      int64  `gorm:"column:created_at"`
	UpdatedAt      int64  `gorm:"column:updated_at"`
	// DeletedAt 未删除时数据库为NULL，删除时由模型写入Unix秒并同步UpdatedAt。
	DeletedAt    soft_delete.DeletedAt `gorm:"column:deleted_at;default:null;softDelete:DeletedAtField:UpdatedAt"`
	BusinessCode string                `gorm:"column:business_code"`
}

func (u PermissionMenu) TableName() string {
	return "menu_group"
}

const SitePermissionId = 1 //平台管理默认权限组，固定值
