package schema

import "gorm.io/plugin/soft_delete"

type DemoInfo struct {
	ID        int64                 `gorm:"column:id"`
	CreatedAt int64                 `gorm:"column:created_at"`
	UpdatedAt int64                 `gorm:"column:updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at"`
}

func (u DemoInfo) TableName() string {
	return "demo"
}
