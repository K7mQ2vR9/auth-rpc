package model

import (
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"errors"
	"gorm.io/gorm"
	"time"
)

type DemoInfoModel struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	db     *gorm.DB
}

// NewDemoInfoModel 获取绑定请求ctx的数据库会话，取库或组合模型初始化失败时返回错误。
func NewDemoInfoModel(ctx context.Context, svcCtx *svc.ServiceContext) (*DemoInfoModel, error) {
	db, err := svcCtx.DbSelect.GetDb(ctx, DB)
	if err != nil {
		return nil, err
	}
	return &DemoInfoModel{
		ctx: ctx,
		db:  db,
	}, nil
}

func (model *DemoInfoModel) getDb() *gorm.DB {
	return model.db
}

func (model *DemoInfoModel) FindOne(in *schema.DemoInfo) schema.DemoInfo {
	var res schema.DemoInfo

	builder := model.getDb().Model(&schema.DemoInfo{})
	if in.ID != 0 {
		builder.Where("id = ?", in.ID)
	}
	dbRes := builder.First(&res)
	if err := dbRes.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
		}
	}
	return res
}

func (model *DemoInfoModel) InsertSchema(data *schema.DemoInfo) error {
	data.CreatedAt = time.Now().Unix()
	dbRes := model.getDb().Model(&schema.DemoInfo{}).Create(data)
	if err := dbRes.Error; err != nil {
		return err
	}

	return nil
}

func (model *DemoInfoModel) UpdateByMap(id int64, data *schema.DemoInfo) error {
	upData := map[string]interface{}{}
	return model.getDb().Model(&schema.DemoInfo{}).Where("id = ?", id).Updates(upData).Error
}

// UpdateAll 保存示例业务字段，排除软删除列，避免零值覆盖数据库的未删除NULL。
func (model *DemoInfoModel) UpdateAll(id int64, data *schema.DemoInfo) error {
	return model.getDb().Model(&schema.DemoInfo{}).Where("id = ?", id).Omit("deleted_at").Save(data).Error
}

func (model *DemoInfoModel) GetList(in *schema.DemoInfo, sTime, eTime int64, pageQuery *PageQuery) (int64, []*schema.DemoInfo, error) {
	builder := model.getDb().Model(&schema.DemoInfo{})
	if sTime != 0 {
		builder.Where("created_at >= ?", sTime)
	}
	if eTime != 0 {
		builder.Where("created_at <= ?", eTime)
	}

	total := int64(0)
	err := builder.Count(&total).Error
	if err != nil {
		return 0, nil, err
	}

	rows := make([]*schema.DemoInfo, 0)
	err = builder.Order("id desc").Offset(pageQuery.Offset()).Limit(pageQuery.GetPageSize()).Find(&rows).Error
	return total, rows, err
}
