package mysql

import (
	bmodel "authRpc/internal/dao/model"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"time"
)

type RoleModel struct {
	ctx               context.Context
	svcCtx            *svc.ServiceContext
	db                *gorm.DB
	RolePermissionKey string
}

// NewRoleModel 获取绑定请求ctx的数据库会话，取库或组合模型初始化失败时返回错误。
func NewRoleModel(ctx context.Context, svcCtx *svc.ServiceContext) (*RoleModel, error) {
	db, err := svcCtx.DbSelect.GetDb(ctx, bmodel.DB)
	if err != nil {
		return nil, err
	}
	return &RoleModel{
		ctx:               ctx,
		svcCtx:            svcCtx,
		db:                db,
		RolePermissionKey: "rolePermission:",
	}, nil
}

func (model *RoleModel) getDb() *gorm.DB {
	return model.db
}

func (model *RoleModel) FindOne(in *schema.RolePermission) schema.RolePermission {
	var res schema.RolePermission

	builder := model.getDb().Model(&schema.RolePermission{}).Scopes(bmodel.WhereBusinessCode(model.ctx, in.BusinessCode))
	if in.ID != 0 {
		builder.Where("id = ?", in.ID)
	}
	if in.RoleName != "" {
		builder.Where("role_name like ?", fmt.Sprintf("%s", "%"+in.RoleName+"%"))
	}
	if in.PermissionIds != "" {
		builder.Where("FIND_IN_SET(?,permission_ids) > 0", in.PermissionIds)
	}
	if in.IsSuperRole != 0 {
		builder.Where("is_super_role = ?", in.IsSuperRole)
	}
	dbRes := builder.First(&res)
	if err := dbRes.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
		}
	}
	return res
}

func (model *RoleModel) FindOneById(id int64) schema.RolePermission {
	var res schema.RolePermission

	builder := model.getDb().Model(&schema.RolePermission{}).Where("id = ?", id)
	dbRes := builder.First(&res)
	if err := dbRes.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
		}
	}
	return res
}

func (model *RoleModel) InsertSchema(data *schema.RolePermission) error {
	data.CreatedAt = time.Now().Unix()
	if data.PermissionIds == "" {
		data.PermissionIds = "[]"
	}
	dbRes := model.getDb().Model(&schema.RolePermission{}).Create(data)
	if err := dbRes.Error; err != nil {
		return err
	}

	return nil
}

// UpdateByMap 仅更新角色业务字段，软删除统一使用DeleteByID。
func (model *RoleModel) UpdateByMap(id int64, data *schema.RolePermission) error {
	upData := map[string]interface{}{}
	if data.PermissionIds != "" {
		upData["permission_ids"] = data.PermissionIds
	}
	if data.RoleName != "" {
		upData["role_name"] = data.RoleName
	}
	if data.IsSuperRole != 0 {
		upData["is_super_role"] = data.IsSuperRole
	}
	upData["updated_at"] = time.Now().Unix()
	return model.getDb().Model(&schema.RolePermission{}).Where("id = ?", id).Updates(upData).Error
}

// DeleteByID 通过模型软删除指定角色，自动写入Unix秒并同步UpdatedAt，不物理删除记录。
func (model *RoleModel) DeleteByID(id int64) error {
	if id <= 0 {
		return status.Error(codes.InvalidArgument, "角色ID必须为正数")
	}
	return model.getDb().Where("id = ?", id).Delete(&schema.RolePermission{}).Error
}

// UpdateAll 更新角色业务字段，软删除字段由DeleteByID统一处理。
func (model *RoleModel) UpdateAll(id int64, data *schema.RolePermission) error {
	return model.getDb().Model(&schema.RolePermission{}).Where("id = ?", id).Omit("deleted_at").Updates(data).Error
}

func (model *RoleModel) GetList(in *schema.RolePermission, pageQuery *bmodel.PageQuery) (int64, []*schema.RolePermission, error) {
	builder := model.getDb().Model(&schema.RolePermission{}).Scopes(bmodel.WhereBusinessCode(model.ctx, in.BusinessCode))
	if in.RoleName != "" {
		builder.Where("role_name like ?", fmt.Sprintf("%s", "%"+in.RoleName+"%"))
	}
	if in.ID != 0 {
		builder.Where("id = ?", in.ID)
	}
	if in.IsSuperRole != 0 {
		builder.Where("is_super_role = ?", in.IsSuperRole)
	}
	total := int64(0)
	err := builder.Count(&total).Error
	if err != nil {
		return 0, nil, err
	}

	rows := make([]*schema.RolePermission, 0)
	err = builder.Offset(pageQuery.Offset()).Limit(pageQuery.GetPageSize()).Find(&rows).Error
	return total, rows, err
}

func (model *RoleModel) GetAllRoleByCache() ([]*schema.RolePermission, error) {
	rows := make([]*schema.RolePermission, 0)
	err := model.getDb().Model(&schema.RolePermission{}).Find(&rows).Error
	return rows, err
}

func (model *RoleModel) GetRolePermissionById(roleId int64) (res []int64, err error) {
	res = make([]int64, 0)
	roleInfo := model.FindOneById(roleId)
	if roleInfo.ID == 0 {
		return res, nil
	}
	json.Unmarshal([]byte(roleInfo.PermissionIds), &res)
	return
}

func (model *RoleModel) CreatedSuperRole(businessCode string) (*schema.RolePermission, error) {
	resp := &schema.RolePermission{}

	roleInfo := model.FindOne(&schema.RolePermission{BusinessCode: businessCode, IsSuperRole: schema.RolePermissionIsSuperRole1})
	if roleInfo.ID != 0 {
		return &roleInfo, nil
	}

	var permissionMenu schema.PermissionMenu
	model.getDb().Model(&schema.PermissionMenu{}).Where("id = ?", schema.SitePermissionId).First(&permissionMenu)
	if permissionMenu.ID == 0 {
		return nil, errors.New("init permission menu not found")
	}

	err := model.getDb().Transaction(func(tx *gorm.DB) error {
		rolePermissionNew := &schema.RolePermission{
			RoleName:      "site super admin",
			PermissionIds: fmt.Sprintf("[%d]", permissionMenu.ID),
			CreatedAt:     time.Now().Unix(),
			BusinessCode:  businessCode,
			IsSuperRole:   schema.RolePermissionIsSuperRole1,
		}
		err := tx.Model(schema.RolePermission{}).Create(rolePermissionNew).Error
		if err != nil {
			return err
		}
		resp = rolePermissionNew
		return err
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (model *RoleModel) CountPermissionId(id int64) schema.RolePermission {

	var res schema.RolePermission

	builder := model.getDb().Model(&schema.RolePermission{}).Where("JSON_CONTAINS(permission_ids, ?)", cast.ToString(id))
	dbRes := builder.First(&res)
	if err := dbRes.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
		}
	}
	return res
}
