package schema

import "gorm.io/plugin/soft_delete"

type AdminInfo struct {
	ID           int64  `gorm:"column:id"`
	Account      string `gorm:"column:account"`
	Name         string `gorm:"column:name"`
	Password     string `gorm:"column:password"`
	PasswordSign string `gorm:"column:password_sign"`
	Status       int    `gorm:"column:status"`
	// AccountType 映射已有账号类型：10平台超级管理员、11平台管理员、20站点超级管理员、30站点管理员、40代理管理员。
	// 数据库默认0表示未设置，不属于有效类型；身份不从角色模板推导，本字段映射不授予业务权限。
	AccountType int   `gorm:"column:account_type"`
	CreatedAt   int64 `gorm:"column:created_at"`
	UpdatedAt   int64 `gorm:"column:updated_at"`
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

// 账号类型沿用现有数据库及user-rpc定义，不按数值大小推断管理级别或可授予范围。
const (
	AdminInfoAccountType10 = 10 // 平台超级管理员。
	AdminInfoAccountType11 = 11 // 平台管理员。
	AdminInfoAccountType20 = 20 // 站点超级管理员，每个站点有且只有一位。
	AdminInfoAccountType30 = 30 // 站点管理员。
	AdminInfoAccountType40 = 40 // 代理管理员。
)
