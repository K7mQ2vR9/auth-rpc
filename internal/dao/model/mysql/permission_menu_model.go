package mysql

import (
	bmodel "authRpc/internal/dao/model"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"errors"
	"fmt"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"time"
)

type PermissionMenuModel struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	db     *gorm.DB
}

// NewPermissionMenuModel 获取绑定请求ctx的数据库会话，取库或组合模型初始化失败时返回错误。
func NewPermissionMenuModel(ctx context.Context, svcCtx *svc.ServiceContext) (*PermissionMenuModel, error) {
	db, err := svcCtx.DbSelect.GetDb(ctx, bmodel.DB)
	if err != nil {
		return nil, err
	}
	return &PermissionMenuModel{
		ctx: ctx,
		db:  db,
	}, nil
}

func (model *PermissionMenuModel) getDb() *gorm.DB {
	return model.db
}

func (model *PermissionMenuModel) FindOne(in *schema.PermissionMenu) schema.PermissionMenu {
	var res schema.PermissionMenu

	builder := model.getDb().Model(&schema.PermissionMenu{}).Scopes(bmodel.WhereBusinessCode(model.ctx, in.BusinessCode))
	if in.ID != 0 {
		builder.Where("id = ?", in.ID)
	}
	if in.MenuIds != "" {
		builder.Where("FIND_IN_SET(?,menu_ids) > 0", in.MenuIds)
	}
	if in.PermissionName != "" {
		builder.Where("permission_name like ?", fmt.Sprintf("%s", "%"+in.PermissionName+"%"))
	}
	dbRes := builder.First(&res)
	if err := dbRes.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
		}
	}
	return res
}

func (model *PermissionMenuModel) FindOneByMenuId(MenuId string) schema.PermissionMenu {
	var res schema.PermissionMenu

	builder := model.getDb().Model(&schema.PermissionMenu{})
	builder.Where("FIND_IN_SET(?,menu_ids) > 0", MenuId)
	dbRes := builder.First(&res)
	if err := dbRes.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
		}
	}
	return res
}

func (model *PermissionMenuModel) InsertSchema(data *schema.PermissionMenu) error {
	data.CreatedAt = time.Now().Unix()
	if data.MenuIds == "" {
		data.MenuIds = "[]"
	}
	dbRes := model.getDb().Model(&schema.PermissionMenu{}).Create(data)
	if err := dbRes.Error; err != nil {
		return err
	}

	return nil
}

// UpdateByMap 仅更新分组业务字段，软删除统一使用DeleteByID。
func (model *PermissionMenuModel) UpdateByMap(id int64, data *schema.PermissionMenu) error {
	upData := map[string]interface{}{}
	if data.MenuIds != "" {
		upData["menu_ids"] = data.MenuIds
	}
	if data.PermissionName != "" {
		upData["permission_name"] = data.PermissionName
	}
	upData["updated_at"] = time.Now().Unix()
	return model.getDb().Model(&schema.PermissionMenu{}).Where("id = ?", id).Updates(upData).Error
}

// DeleteByID 通过模型软删除指定分组，自动写入Unix秒并同步UpdatedAt，不物理删除记录。
func (model *PermissionMenuModel) DeleteByID(id int64) error {
	if id <= 0 {
		return status.Error(codes.InvalidArgument, "分组ID必须为正数")
	}
	return model.getDb().Where("id = ?", id).Delete(&schema.PermissionMenu{}).Error
}

// UpdateAll 更新分组业务字段，软删除字段由DeleteByID统一处理。
func (model *PermissionMenuModel) UpdateAll(id int64, data *schema.PermissionMenu) error {
	return model.getDb().Model(&schema.PermissionMenu{}).Where("id = ?", id).Omit("deleted_at").Updates(data).Error
}

func (model *PermissionMenuModel) GetList(in *schema.PermissionMenu, pageQuery *bmodel.PageQuery) (int64, []*schema.PermissionMenu, error) {
	builder := model.getDb().Model(&schema.PermissionMenu{})

	if in.BusinessCode != "" {
		builder.Where("business_code = ?", in.BusinessCode)
	} else {
		builder.Where("business_code = ?", headInfo.GetBusinessCode(model.ctx))
	}
	builder.Or("id = ?", schema.SitePermissionId)

	total := int64(0)
	err := builder.Count(&total).Error
	if err != nil {
		return 0, nil, err
	}

	rows := make([]*schema.PermissionMenu, 0)
	err = builder.Offset(pageQuery.Offset()).Limit(pageQuery.GetPageSize()).Find(&rows).Error
	return total, rows, err
}

func (model *PermissionMenuModel) GetAll(in *schema.PermissionMenu) ([]*schema.PermissionMenu, error) {
	rows := make([]*schema.PermissionMenu, 0)
	builder := model.getDb().Model(&schema.PermissionMenu{})
	if in.BusinessCode != "" {
		builder.Where("business_code = ?", in.BusinessCode)
	} else {
		builder.Where("business_code = ?", headInfo.GetBusinessCode(model.ctx))
	}
	builder.Or("id = ?", schema.SitePermissionId)
	err := builder.Find(&rows).Error
	return rows, err
}
