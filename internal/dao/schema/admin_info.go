package schema

import "gorm.io/plugin/soft_delete"

type AdminInfo struct {
	ID           int64  `gorm:"column:id"`
	Account      string `gorm:"column:account"`
	Name         string `gorm:"column:name"`
	Password     string `gorm:"column:password"`
	PasswordSign string `gorm:"column:password_sign"`
	RoleId       int64  `gorm:"column:role_id"`
	Status       int    `gorm:"column:status"`
	CreatedAt    int64  `gorm:"column:created_at"`
	UpdatedAt    int64  `gorm:"column:updated_at"`
	// DeletedAt 未删除时数据库为NULL，删除时由模型写入Unix秒并同步UpdatedAt。
	DeletedAt    soft_delete.DeletedAt `gorm:"column:deleted_at;default:null;softDelete:DeletedAtField:UpdatedAt"`
	BusinessCode string                `gorm:"column:business_code"`
}

func (u AdminInfo) TableName() string {
	return "admin_info"
}

const (
	AdminInfoStatus1 = 1 //正常
	AdminInfoStatus2 = 2 //异常
)
