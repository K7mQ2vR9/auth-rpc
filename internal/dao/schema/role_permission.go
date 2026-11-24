package schema

import "gorm.io/plugin/soft_delete"

type RolePermission struct {
	ID            int64  `gorm:"column:id"`
	RoleName      string `gorm:"column:role_name"`
	PermissionIds string `gorm:"column:permission_ids"`
	CreatedAt     int64  `gorm:"column:created_at"`
	UpdatedAt     int64  `gorm:"column:updated_at"`
	// DeletedAt 未删除时数据库为NULL，删除时由模型写入Unix秒并同步UpdatedAt。
	DeletedAt    soft_delete.DeletedAt `gorm:"column:deleted_at;default:null;softDelete:DeletedAtField:UpdatedAt"`
	BusinessCode string                `gorm:"column:business_code"`
	IsSuperRole  int                   `gorm:"column:is_super_role"`
}

func (u RolePermission) TableName() string {
	return "role_permission"
}

const (
	RolePermissionIsSuperRole1 = 1 //是否超级管理员 ，1=是 ，2=普通管理员，3=代理管理员
	RolePermissionIsSuperRole2 = 2
	RolePermissionIsSuperRole3 = 3
)
