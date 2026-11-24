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

type LoginRoleMenuItem struct {
	Id       int64  `json:"id"`       //菜单ID
	MenuName string `json:"menuName"` //菜单名字
	Path     string `json:"path"`     //菜单路径地址
	MenuPid  int64  `json:"menuPid"`  //父级ID
	MenuType int    `json:"menuType"` // 1=菜单，2=按钮
	// ServiceName 为菜单所属API服务名，精确匹配，不提供默认值。
	ServiceName string               `json:"serviceName"`
	Children    []*LoginRoleMenuItem `json:"children"` //子级菜单
}
