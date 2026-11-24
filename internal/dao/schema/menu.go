package schema

import "gorm.io/plugin/soft_delete"

type Menu struct {
	ID           int64                 `gorm:"column:id"`
	MenuPid      int64                 `gorm:"column:menu_pid"`
	MenuName     string                `gorm:"column:menu_name"`
	MenuType     int                   `gorm:"column:menu_type"`
	Path         string                `gorm:"column:path"`
	Status       int                   `gorm:"column:status"`
	Sort         int                   `gorm:"column:sort"`
	CreatedAt    int64                 `gorm:"column:created_at"`
	UpdatedAt    int64                 `gorm:"column:updated_at"`
	DeletedAt    soft_delete.DeletedAt `gorm:"column:deleted_at"`
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
