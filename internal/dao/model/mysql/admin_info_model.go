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
)

type AdminInfoModel struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	db     *gorm.DB
}

// withPlatformCatalogWrite 集中保护菜单和分组维护：仅平台归属且正常、未删除的真实账号10可写。
// 同一事务普通读取真实身份后写入，目标记录通过version条件防止覆盖，不使用目录锁表或显式行锁。
func withPlatformCatalogWrite(ctx context.Context, db *gorm.DB, write func(*gorm.DB) error) error {
	roles := &RoleModel{ctx: ctx, db: db}
	current, err := roles.businessCode("")
	if err != nil {
		return err
	}
	if current != headInfo.PLATFORM_BUSINESS_CODE {
		return status.Error(codes.PermissionDenied, "仅平台超级管理员10可维护菜单和分组")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		operator, err := roles.permissionOperator(tx, current)
		if err != nil {
			return err
		}
		if operator.AccountType != schema.AdminInfoAccountType10 {
			return status.Error(codes.PermissionDenied, "仅平台超级管理员10可维护菜单和分组")
		}
		return write(tx)
	})
}

// requireUniqueSiteSuper 校验20是本站唯一超级管理员，查询含软删除身份，避免删除后另建第二位而复用旧授权。
// 普通读取身份，数据库唯一索引防并发重复；至少一位由其他业务服务保证，不锁定账号记录。
func requireUniqueSiteSuper(db *gorm.DB, businessCode string, userID int64) error {
	if businessCode == headInfo.PLATFORM_BUSINESS_CODE || businessCode == "" || userID <= 0 {
		return status.Error(codes.PermissionDenied, "站点超级管理员归属无效")
	}
	var accounts []struct{ ID int64 }
	query := db.Unscoped().Model(&schema.AdminInfo{}).Select("id").
		Where("business_code = ? AND account_type = ?", businessCode, schema.AdminInfoAccountType20).Order("id ASC").Limit(2)
	if err := query.Find(&accounts).Error; err != nil {
		return fmt.Errorf("query unique site super administrator: %w", err)
	}
	if len(accounts) != 1 {
		return status.Error(codes.DataLoss, "本站超级管理员必须唯一，禁止缺失或重复身份")
	}
	if accounts[0].ID != userID {
		return status.Error(codes.PermissionDenied, "当前用户不是本站唯一超级管理员")
	}
	return nil
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
