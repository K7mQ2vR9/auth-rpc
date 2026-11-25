package dto

type MenuTree struct {
	ID       int64  `gorm:"column:id"`
	MenuPid  int64  `gorm:"column:menu_pid"`
	MenuName string `gorm:"column:menu_name"`
	MenuType int    `gorm:"column:menu_type"`
	Path     string `gorm:"column:path"`
	// ServiceName 区分相同路径所属的API服务，与菜单表保持一致。
	ServiceName string `gorm:"column:service_name"`
	Children    []*MenuTree
}
