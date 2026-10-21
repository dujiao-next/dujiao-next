package gormstore

import (
	"errors"
	"strings"
	"time"

	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	"github.com/dujiao-next/internal/persistence/gormutil"

	"gorm.io/gorm"
)

// MappingStore 是 Catalog 上游映射端口的 GORM 实现。
type MappingStore struct {
	db *gorm.DB
}

var _ mappingcontract.MappingRepository = (*MappingStore)(nil)

func NewMappingStore(db *gorm.DB) *MappingStore {
	return &MappingStore{db: db}
}

// WithTx 绑定事务
func (r *MappingStore) WithTx(tx *gorm.DB) *MappingStore {
	if tx == nil {
		return r
	}
	return NewMappingStore(tx)
}

// BindTx 将事务内 store 暴露为导入用例所需的窄写入端口。
func (r *MappingStore) BindTx(tx *gorm.DB) mappingcontract.MappingRepository {
	return r.WithTx(tx)
}

func (r *MappingStore) GetByID(id uint) (*mappingdomain.Mapping, error) {
	var m mappingdomain.Mapping
	if err := r.db.Where("product_mappings.deleted_at IS NULL").Preload("Connection", "deleted_at IS NULL").Preload("Product", "deleted_at IS NULL").First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *MappingStore) GetByLocalProductID(productID uint) (*mappingdomain.Mapping, error) {
	var m mappingdomain.Mapping
	if err := r.db.Where("deleted_at IS NULL AND local_product_id = ?", productID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *MappingStore) GetByConnectionAndUpstreamID(connectionID, upstreamProductID uint) (*mappingdomain.Mapping, error) {
	var m mappingdomain.Mapping
	if err := r.db.Where("deleted_at IS NULL AND connection_id = ? AND upstream_product_id = ?", connectionID, upstreamProductID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *MappingStore) GetByConnectionAndExternalID(connectionID uint, upstreamProductExternalID string) (*mappingdomain.Mapping, error) {
	var m mappingdomain.Mapping
	if err := r.db.Where("deleted_at IS NULL AND connection_id = ? AND upstream_product_external_id = ?", connectionID, strings.TrimSpace(upstreamProductExternalID)).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *MappingStore) Create(mapping *mappingdomain.Mapping) error {
	return r.db.Create(mapping).Error
}

func (r *MappingStore) Update(mapping *mappingdomain.Mapping) error {
	return r.db.Save(mapping).Error
}

func (r *MappingStore) Delete(id uint) error {
	return r.db.Model(&mappingdomain.Mapping{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

// DeleteByLocalProduct 删除指定本地商品的所有映射及其 SKU 映射
func (r *MappingStore) DeleteByLocalProduct(productID uint) error {
	// 先删除关联的 SKU 映射
	if err := r.db.Where("product_mapping_id IN (?)",
		r.db.Model(&mappingdomain.Mapping{}).Select("id").Where("deleted_at IS NULL AND local_product_id = ?", productID),
	).Where("deleted_at IS NULL").Model(&mappingdomain.SKUMapping{}).Update("deleted_at", time.Now()).Error; err != nil {
		return err
	}
	return r.db.Model(&mappingdomain.Mapping{}).
		Where("deleted_at IS NULL AND local_product_id = ?", productID).
		Update("deleted_at", time.Now()).Error
}

func (r *MappingStore) List(filter mappingcontract.ListFilter) ([]mappingdomain.Mapping, int64, error) {
	var mappings []mappingdomain.Mapping
	var total int64

	q := r.db.Model(&mappingdomain.Mapping{}).Where("product_mappings.deleted_at IS NULL")
	if filter.ConnectionID > 0 {
		q = q.Where("connection_id = ?", filter.ConnectionID)
	}
	if filter.UpstreamStatus != "" {
		q = q.Where("upstream_status = ?", filter.UpstreamStatus)
	}
	if filter.ProductStatus == "active" {
		q = q.Where("product_mappings.is_active = ?", true)
	} else if filter.ProductStatus == "inactive" {
		q = q.Where("product_mappings.is_active = ?", false)
	}
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		condition, argCount := gormutil.BuildLocalizedLikeCondition(r.db, nil, []string{"title_json"})
		q = q.Where(
			"product_mappings.local_product_id IN (SELECT id FROM products WHERE deleted_at IS NULL AND ("+condition+"))",
			gormutil.RepeatLikeArgs(like, argCount)...,
		)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	q = q.
		Preload("Connection", "deleted_at IS NULL").
		Preload("Product", "deleted_at IS NULL").
		Preload("Product.SKUs", "deleted_at IS NULL").
		Order("created_at DESC")
	if filter.Page > 0 && filter.PageSize > 0 {
		q = q.Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize)
	}

	if err := q.Find(&mappings).Error; err != nil {
		return nil, 0, err
	}

	return mappings, total, nil
}

func (r *MappingStore) ListActiveByConnection(connectionID uint) ([]mappingdomain.Mapping, error) {
	var mappings []mappingdomain.Mapping
	if err := r.db.Where("deleted_at IS NULL AND connection_id = ? AND is_active = ?", connectionID, true).Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

func (r *MappingStore) ListAllActive() ([]mappingdomain.Mapping, error) {
	var mappings []mappingdomain.Mapping
	if err := r.db.Where("deleted_at IS NULL AND is_active = ?", true).Preload("Connection", "deleted_at IS NULL").Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

func (r *MappingStore) ListByLocalProductIDs(productIDs []uint) ([]mappingdomain.Mapping, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}
	var mappings []mappingdomain.Mapping
	if err := r.db.Where("deleted_at IS NULL AND local_product_id IN ?", productIDs).Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

func (r *MappingStore) ListUpstreamIDsByConnection(connectionID uint) ([]uint, error) {
	var ids []uint
	if err := r.db.Model(&mappingdomain.Mapping{}).
		Where("deleted_at IS NULL AND connection_id = ?", connectionID).
		Pluck("upstream_product_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *MappingStore) ListUpstreamExternalIDsByConnection(connectionID uint) ([]string, error) {
	var ids []string
	if err := r.db.Model(&mappingdomain.Mapping{}).
		Where("deleted_at IS NULL AND connection_id = ? AND upstream_product_external_id <> ''", connectionID).
		Pluck("upstream_product_external_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
