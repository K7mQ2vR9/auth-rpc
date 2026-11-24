package schema

import "gorm.io/plugin/soft_delete"

type PermissionMenu struct {
	ID             int64                 `gorm:"column:id"`
	PermissionName string                `gorm:"column:permission_name"`
	MenuIds        string                `gorm:"column:menu_ids"`
	CreatedAt      int64                 `gorm:"column:created_at"`
	UpdatedAt      int64                 `gorm:"column:updated_at"`
	DeletedAt      soft_delete.DeletedAt `gorm:"column:deleted_at"`
	BusinessCode   string                `gorm:"column:business_code"`
}

func (u PermissionMenu) TableName() string {
	return "permission_menu"
}

const SitePermissionId = 1 //平台管理默认权限组，固定值
