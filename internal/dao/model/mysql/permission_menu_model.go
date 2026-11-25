package mysql

import (
	bmodel "authRpc/internal/dao/model"
	"authRpc/internal/dao/schema"
	"authRpc/internal/svc"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// PermissionMenuModel 管理平台共享展示分组，成员为JSON菜单ID数组，不形成权限授予或固定ID特权。
type PermissionMenuModel struct {
	ctx context.Context
	db  *gorm.DB
}

// NewPermissionMenuModel 获取请求级数据库会话；分组写入资格统一由目录维护事务校验。
func NewPermissionMenuModel(ctx context.Context, svcCtx *svc.ServiceContext) (*PermissionMenuModel, error) {
	if ctx == nil || svcCtx == nil {
		return nil, errors.New("permission group requires request context and service context")
	}
	db, err := svcCtx.DbSelect.GetDb(ctx, bmodel.DB)
	if err != nil {
		return nil, err
	}
	return &PermissionMenuModel{ctx: ctx, db: db}, nil
}

// ParseGroupMenuIDs 供分组列表与授权编辑页共用，严格解析正整数JSON数组、去重升序。
// 历史SQL NULL映射的空字符串或JSON null视为空组；读取最多131070字节，容纳MySQL规范化增加的空白。
// 写入输入最多65535字节；任何错误均不返回部分成员。
func ParseGroupMenuIDs(value string) ([]int64, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return []int64{}, nil
	}
	if len(value) > 2*MaxMenuIDsBytes || value[0] != '[' {
		return nil, errors.New("invalid group menu ID array")
	}
	var parsed []int64
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return nil, errors.New("invalid group menu ID array")
	}
	ids := make([]int64, 0, len(parsed))
	seen := make(map[int64]bool, len(parsed))
	for _, id := range parsed {
		if id <= 0 {
			return nil, errors.New("invalid group menu ID array")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// normalizeMembers 在写事务中普通读取成员；缺失/软删除按取消选择，禁用但未删除的成员保留。
// 写入只接受JSON数组，拒绝null、对象、字符串、小数、非正数及有效跨目录项，最多65535字节。
func (model *PermissionMenuModel) normalizeMembers(tx *gorm.DB, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > MaxMenuIDsBytes || value[0] != '[' {
		return "", status.Error(codes.InvalidArgument, "menuIds必须为正整数JSON数组，清空请传[]")
	}
	ids, err := ParseGroupMenuIDs(value)
	if err != nil {
		return "", status.Error(codes.InvalidArgument, "menuIds必须为正整数JSON数组且不超过65535字节")
	}
	kept := make([]int64, 0, len(ids))
	if len(ids) > 0 {
		var menus []schema.Menu
		if err := tx.Unscoped().Model(&schema.Menu{}).Select("id", "business_code", "menu_type", "deleted_at").
			Where("id IN ?", ids).Order("id ASC").Find(&menus).Error; err != nil {
			return "", fmt.Errorf("query group members: %w", err)
		}
		for _, menu := range menus {
			if menu.DeletedAt != 0 {
				continue
			}
			if menu.BusinessCode != headInfo.PLATFORM_BUSINESS_CODE {
				return "", status.Error(codes.PermissionDenied, "分组只能引用平台共享目录菜单")
			}
			if menu.ID <= 0 || menu.MenuType != schema.MenuMenuType1 && menu.MenuType != schema.MenuMenuType2 {
				return "", status.Error(codes.InvalidArgument, "分组成员必须为有效页面或按钮")
			}
			kept = append(kept, menu.ID)
		}
	}
	encoded, err := json.Marshal(kept)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// validateGroupName 检查展示名称，最多255个UTF-8字符，不接受空值及首尾空白。
func validateGroupName(name string) error {
	if name == "" || name != strings.TrimSpace(name) || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 {
		return status.Error(codes.InvalidArgument, "分组名称必填，最多255个UTF-8字符且不能有首尾空白")
	}
	return nil
}

// InsertSchema 仅账号10新增共享分组；名称和成员数组必填，归属固定为平台，[]表示空分组。
func (model *PermissionMenuModel) InsertSchema(data *schema.PermissionMenu) error {
	if data == nil || data.ID != 0 || data.Version != 0 {
		return status.Error(codes.InvalidArgument, "分组data不能为空，新增不能指定ID或已有版本")
	}
	return withPlatformCatalogWrite(model.ctx, model.db, func(tx *gorm.DB) error {
		if data.BusinessCode != "" && data.BusinessCode != headInfo.PLATFORM_BUSINESS_CODE {
			return status.Error(codes.PermissionDenied, "分组只能归属平台共享目录")
		}
		if err := validateGroupName(data.PermissionName); err != nil {
			return err
		}
		members, err := model.normalizeMembers(tx, data.MenuIds)
		if err != nil {
			return err
		}
		data.BusinessCode = headInfo.PLATFORM_BUSINESS_CODE
		data.MenuIds = members
		data.Version = 1
		data.CreatedAt = time.Now().Unix()
		return tx.Create(data).Error
	})
}

// UpdateByMap 仅账号10按已读版本修改共享分组；名称/成员缺省保持，[]清空；CAS成功提交后返回新版本。
func (model *PermissionMenuModel) UpdateByMap(id int64, data *schema.PermissionMenu) error {
	if id <= 0 || data == nil || data.Version <= 0 || data.Version == math.MaxInt64 {
		return status.Error(codes.InvalidArgument, "分组ID/version须为正数且version小于MaxInt64")
	}
	err := withPlatformCatalogWrite(model.ctx, model.db, func(tx *gorm.DB) error {
		if data.BusinessCode != "" && data.BusinessCode != headInfo.PLATFORM_BUSINESS_CODE {
			return status.Error(codes.PermissionDenied, "分组只能归属平台共享目录")
		}
		var target schema.PermissionMenu
		err := tx.Model(&schema.PermissionMenu{}).Select("id", "version").Where("id = ? AND business_code = ?", id, headInfo.PLATFORM_BUSINESS_CODE).Take(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Error(codes.NotFound, "分组不存在或不属于平台共享目录")
		}
		if err != nil {
			return err
		}
		if target.Version <= 0 {
			return status.Error(codes.DataLoss, "分组版本异常")
		}
		if target.Version != data.Version {
			return catalogVersionConflict()
		}
		fields := map[string]interface{}{"updated_at": time.Now().Unix(), "version": gorm.Expr("version + 1")}
		if data.PermissionName != "" {
			if err := validateGroupName(data.PermissionName); err != nil {
				return err
			}
			fields["permission_name"] = data.PermissionName
		}
		if data.MenuIds != "" {
			members, err := model.normalizeMembers(tx, data.MenuIds)
			if err != nil {
				return err
			}
			fields["menu_ids"] = members
		}
		result := tx.Model(&schema.PermissionMenu{}).Where("id = ? AND business_code = ? AND version = ?", id, headInfo.PLATFORM_BUSINESS_CODE, data.Version).Updates(fields)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return catalogVersionConflict()
		}
		return nil
	})
	if err == nil {
		data.Version++
	}
	return err
}

// DeleteByID 仅账号10按旧版本软删除共享分组，CAS递增与模型Delete同事务；保留菜单/授权及统一ID规则。
func (model *PermissionMenuModel) DeleteByID(id, version int64) error {
	if id <= 0 || version <= 0 || version == math.MaxInt64 {
		return status.Error(codes.InvalidArgument, "分组ID/version须为正数且version小于MaxInt64")
	}
	return withPlatformCatalogWrite(model.ctx, model.db, func(tx *gorm.DB) error {
		var target schema.PermissionMenu
		err := tx.Model(&schema.PermissionMenu{}).Select("id", "version").Where("id = ? AND business_code = ?", id, headInfo.PLATFORM_BUSINESS_CODE).Take(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Error(codes.NotFound, "分组不存在或不属于平台共享目录")
		}
		if err != nil {
			return err
		}
		if target.Version <= 0 {
			return status.Error(codes.DataLoss, "分组版本异常")
		}
		if target.Version != version {
			return catalogVersionConflict()
		}
		updated := tx.Model(&schema.PermissionMenu{}).Where("id = ? AND business_code = ? AND version = ?", id, headInfo.PLATFORM_BUSINESS_CODE, version).Update("version", gorm.Expr("version + 1"))
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return catalogVersionConflict()
		}
		deleted := tx.Where("id = ? AND business_code = ? AND version = ?", id, headInfo.PLATFORM_BUSINESS_CODE, version+1).Delete(&schema.PermissionMenu{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return catalogVersionConflict()
		}
		return nil
	})
}

// ListMemberMenus 为分组维护读取全部未删除共享成员，包括禁用及祖先失效节点；不用于授权判断。
// status原样回显1/2，缺失/软删除成员不返回。每批最多500个ID，避免分页多组成员超过MySQL绑定参数上限。
func (model *PermissionMenuModel) ListMemberMenus(menuIDs []int64) ([]*schema.Menu, error) {
	ids := make([]int64, 0, len(menuIDs))
	seen := make(map[int64]bool, len(menuIDs))
	for _, id := range menuIDs {
		if id <= 0 {
			return nil, status.Error(codes.DataLoss, "分组成员ID异常")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	result := make([]*schema.Menu, 0)
	const batchSize = 500
	for start := 0; start < len(ids); start += batchSize {
		if err := model.ctx.Err(); err != nil {
			return nil, err
		}
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		var rows []*schema.Menu
		if err := model.db.Model(&schema.Menu{}).Select("id", "menu_name", "status", "menu_type").
			Where("business_code = ? AND id IN ?", headInfo.PLATFORM_BUSINESS_CODE, ids[start:end]).Order("id ASC").Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("query group maintenance members: %w", err)
		}
		for _, menu := range rows {
			if menu == nil || menu.ID <= 0 || (menu.Status != schema.MenuStatus1 && menu.Status != schema.MenuStatus2) ||
				(menu.MenuType != schema.MenuMenuType1 && menu.MenuType != schema.MenuMenuType2) {
				return nil, status.Error(codes.DataLoss, "分组成员状态或类型异常")
			}
			result = append(result, menu)
		}
	}
	return result, nil
}

// GetList 只读平台共享展示分组，不再混入站点或固定ID；page=0默认1、pageSize=0默认20，最多200。
// 数据库与异常成员错误不掩盖为空组，成员解析由调用层共用ParseGroupMenuIDs。
func (model *PermissionMenuModel) GetList(in *schema.PermissionMenu, pageQuery *bmodel.PageQuery) (int64, []*schema.PermissionMenu, error) {
	if in == nil || pageQuery == nil || in.ID < 0 || in.BusinessCode != "" && in.BusinessCode != headInfo.PLATFORM_BUSINESS_CODE {
		return 0, nil, status.Error(codes.InvalidArgument, "分组查询仅支持平台共享目录")
	}
	if pageQuery.Page == 0 {
		pageQuery.Page = 1
	}
	if pageQuery.PageSize == 0 {
		pageQuery.PageSize = 20
	}
	if pageQuery.Page < 1 || pageQuery.PageSize < 1 || pageQuery.PageSize > 200 || pageQuery.Page-1 > math.MaxInt64/pageQuery.PageSize {
		return 0, nil, status.Error(codes.InvalidArgument, "页码必须为正数，pageSize范围为1到200")
	}
	builder := model.db.Model(&schema.PermissionMenu{}).Where("business_code = ?", headInfo.PLATFORM_BUSINESS_CODE)
	if in.ID > 0 {
		builder = builder.Where("id = ?", in.ID)
	}
	if in.PermissionName != "" {
		builder = builder.Where("permission_name LIKE ?", "%"+in.PermissionName+"%")
	}
	var total int64
	if err := builder.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	rows := make([]*schema.PermissionMenu, 0)
	err := builder.Order("id ASC").Offset(pageQuery.Offset()).Limit(pageQuery.GetPageSize()).Find(&rows).Error
	return total, rows, err
}
