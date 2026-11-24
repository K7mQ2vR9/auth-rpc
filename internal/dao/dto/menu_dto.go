package dto

type MenuTree struct {
	ID       int64  `gorm:"column:id"`
	MenuPid  int64  `gorm:"column:menu_pid"`
	MenuName string `gorm:"column:menu_name"`
	MenuType int    `gorm:"column:menu_type"`
	Path     string `gorm:"column:path"`
	Children []*MenuTree
}

type LoginRoleMenuItem struct {
	Id       int64                `json:"id"`       //菜单ID
	MenuName string               `json:"menuName"` //菜单名字
	Path     string               `json:"path"`     //菜单路径地址
	MenuPid  int64                `json:"menuPid"`  //父级ID
	MenuType int                  `json:"menuType"` // 1=菜单，2=按钮
	Children []*LoginRoleMenuItem `json:"children"` //子级菜单
}
