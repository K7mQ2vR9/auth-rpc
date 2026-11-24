package schema

import "gorm.io/plugin/soft_delete"

type Menu struct {
	ID       int64  `gorm:"column:id"`
	MenuPid  int64  `gorm:"column:menu_pid"`
	MenuName string `gorm:"column:menu_name"`
	MenuType int    `gorm:"column:menu_type"`
	Path     string `gorm:"column:path"`
	// ServiceName 为接口所属API服务名，必填，最多64字符；区分大小写，无默认服务或空值通配。
	ServiceName string `gorm:"column:service_name;size:64;not null"`
	Status      int    `gorm:"column:status"`
	Sort        int    `gorm:"column:sort"`
	CreatedAt   int64  `gorm:"column:created_at"`
	UpdatedAt   int64  `gorm:"column:updated_at"`
	// DeletedAt 未删除时数据库为NULL，删除时由模型写入Unix秒并同步UpdatedAt。
	DeletedAt    soft_delete.DeletedAt `gorm:"column:deleted_at;default:null;softDelete:DeletedAtField:UpdatedAt"`
	BusinessCode string                `gorm:"column:business_code"`
}

func (u Menu) TableName() string {
	return "menu"
}

const (
	MenuStatus1 = 1 //启用
	MenuStatus2 = 2 //禁用

	MenuMenuType1 = 1 //页面
	MenuMenuType2 = 2 //按钮
)
