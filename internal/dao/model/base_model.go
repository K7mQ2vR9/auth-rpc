package model

import (
	"authRpc/internal/config"
	"context"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

const (
	// DB 沿用服务配置的连接标识，保持模型和启动必需连接的检查一致。
	DB = config.DatabaseKey
)

type PageQuery struct {
	Page     int64
	PageSize int64
}

func NewPageQuery(page, pageSize int64) *PageQuery {
	return &PageQuery{
		Page:     page,
		PageSize: pageSize,
	}
}

func (s *PageQuery) Offset() int {
	return cast.ToInt((s.Page - 1) * s.PageSize)
}

func (s *PageQuery) GetPageSize() int {
	return int(s.PageSize)
}

func WhereBusinessCode(ctx context.Context, businessCode string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if businessCode != "" {
			return db.Where("business_code = ?", businessCode)
		}

		businessCode = headInfo.GetBusinessCode(ctx)
		if businessCode != headInfo.PLATFORM_BUSINESS_CODE {
			return db.Where("business_code = ?", businessCode)
		}
		return db
	}
}
