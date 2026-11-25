package mysql

import (
	bmodel "authRpc/internal/dao/model"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"errors"
	"gorm.io/gorm"
)

type AdminInfoModel struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	db     *gorm.DB
}

// NewAdminInfoModel 获取绑定请求ctx的数据库会话，取库或组合模型初始化失败时返回错误。
func NewAdminInfoModel(ctx context.Context, svcCtx *svc.ServiceContext) (*AdminInfoModel, error) {
	db, err := svcCtx.DbSelect.GetDb(ctx, bmodel.DB)
	if err != nil {
		return nil, err
	}
	return &AdminInfoModel{
		ctx:    ctx,
		svcCtx: svcCtx,
		db:     db,
	}, nil
}

func (model *AdminInfoModel) getDb() *gorm.DB {
	return model.db
}

// FindOne 按管理员ID和状态读取现有资料，不再通过用户绑定角色筛选管理员。
func (model *AdminInfoModel) FindOne(in *schema.AdminInfo) schema.AdminInfo {
	var res schema.AdminInfo
	builder := model.getDb().Model(&schema.AdminInfo{})
	if in.ID != 0 {
		builder.Where("id = ?", in.ID)
	}
	if in.Status != 0 {
		builder.Where("status = ?", in.Status)
	}
	dbRes := builder.First(&res)
	if err := dbRes.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
		}
	}
	return res
}
