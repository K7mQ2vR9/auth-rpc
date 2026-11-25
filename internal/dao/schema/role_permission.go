package schema

import "gorm.io/plugin/soft_delete"

// RolePermission 统一保存平台默认1、站点默认2和站点普通模板3的具体权限集合，不绑定用户、操作者或菜单分组。
type RolePermission struct {
	// ID 为模板自增主键；站点根模板通过站点归属和类型识别，不依赖固定ID。
	ID int64 `gorm:"column:id;primaryKey;autoIncrement"`
	// RoleName 为模板展示名称，必填，最多255个UTF-8字符。
	RoleName string `gorm:"column:role_name;size:255;not null"`
	// MenuIds 为英文逗号分隔的具体菜单ID，空字符串表示空模板，保存时去重并升序排列。
	MenuIds string `gorm:"column:menu_ids;type:text;not null"`
	// TemplateType 为模板用途：1平台默认、2站点默认、3站点普通；普通CRUD仅写3，默认模板使用专用入口。
	TemplateType int `gorm:"column:template_type;not null"`
	// Version 首次保存为1，修改或软删除时按请求版本比较并递增，不参与权限计算。
	Version int64 `gorm:"column:version;not null;default:1"`
	// CreatedAt、UpdatedAt 为Unix秒级时间戳，由模型创建、修改及软删除维护。
	CreatedAt int64 `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt int64 `gorm:"column:updated_at;autoUpdateTime"`
	// DeletedAt 未删除时数据库为NULL，删除时由模型写入Unix秒并同步UpdatedAt。
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;default:null;softDelete:DeletedAtField:UpdatedAt"`
	// BusinessCode 为模板归属；类型1固定平台编码99999999，类型2/3为站点编码，创建后禁止转移。
	BusinessCode string `gorm:"column:business_code;size:10;not null"`
}

func (u RolePermission) TableName() string {
	return "role_permission"
}

const (
	RoleTemplateTypeRoot            = 2 // 站点根模板，由平台专用入口管理，每站点唯一，普通CRUD不可新增、修改或删除。
	RoleTemplateTypeCustom          = 3 // 站点自定义模板，作为同站点覆盖复制来源，菜单集合不能超出当前根模板。
	RoleTemplateTypePlatformDefault = 1 // 平台默认模板，全平台唯一，由账号10维护具体权限，供平台普通管理员11使用。
)
