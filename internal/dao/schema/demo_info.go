package schema

import "gorm.io/plugin/soft_delete"

type DemoInfo struct {
	ID        int64 `gorm:"column:id"`
	CreatedAt int64 `gorm:"column:created_at"`
	UpdatedAt int64 `gorm:"column:updated_at"`
	// DeletedAt 未删除时数据库为NULL，删除时由模型写入Unix秒并同步UpdatedAt。
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;default:null;softDelete:DeletedAtField:UpdatedAt"`
}

func (u DemoInfo) TableName() string {
	return "demo"
}
