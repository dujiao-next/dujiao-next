package service

import (
	"errors"
	"strconv"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
)

// 文件组织约定:
//   product_mapping_service.go        — 核心:struct/ctor/setters + 基础查询/CRUD + 错误定义
//   product_mapping_import.go         — 单品导入 + 上游元数据列表 + 图片下载
//   product_mapping_sync.go           — 同步流程(单品 / 全量库存)
//   product_mapping_markup.go         — 加价重算
//   product_mapping_batch_import.go   — 按上游分类批量导入

var (
	ErrMappingNotFound               = errors.New("product mapping not found")
	ErrMappingAlreadyExists          = errors.New("product mapping already exists for this upstream product")
	ErrUpstreamProductNotFound       = errors.New("upstream product not found")
	ErrMappingInactive               = errors.New("product mapping is inactive")
	ErrProtocolCapabilityUnsupported = errors.New("connection protocol does not support this capability")
	ErrProductNoActiveSKU            = errors.New("product has no active sku")
)

// ProductMappingService 商品映射业务服务
type ProductMappingService struct {
	mappingRepo     repository.ProductMappingRepository
	skuMappingRepo  repository.SKUMappingRepository
	productRepo     repository.ProductRepository
	productSKURepo  repository.ProductSKURepository
	categoryRepo    repository.CategoryRepository
	connService     *SiteConnectionService
	categoryService *CategoryService
	mediaService    *MediaService
	settingService  *SettingService
}

// NewProductMappingService 创建商品映射服务
func NewProductMappingService(
	mappingRepo repository.ProductMappingRepository,
	skuMappingRepo repository.SKUMappingRepository,
	productRepo repository.ProductRepository,
	productSKURepo repository.ProductSKURepository,
	categoryRepo repository.CategoryRepository,
	connService *SiteConnectionService,
) *ProductMappingService {
	return &ProductMappingService{
		mappingRepo:    mappingRepo,
		skuMappingRepo: skuMappingRepo,
		productRepo:    productRepo,
		productSKURepo: productSKURepo,
		categoryRepo:   categoryRepo,
		connService:    connService,
	}
}

// SetCategoryService 设置分类服务（避免循环依赖）
func (s *ProductMappingService) SetCategoryService(cs *CategoryService) {
	s.categoryService = cs
}

// SetMediaService 设置素材服务（避免循环依赖）
func (s *ProductMappingService) SetMediaService(ms *MediaService) {
	s.mediaService = ms
}

// SetSettingService 注入设置服务（用于读取上游同步动态配置）
func (s *ProductMappingService) SetSettingService(ss *SettingService) {
	s.settingService = ss
}

// GetByID 获取映射详情
func (s *ProductMappingService) GetByID(id uint) (*models.ProductMapping, error) {
	return s.mappingRepo.GetByID(id)
}

// List 列表查询映射
func (s *ProductMappingService) List(filter repository.ProductMappingListFilter) ([]models.ProductMapping, int64, error) {
	return s.mappingRepo.List(filter)
}

// SetActive 启用/禁用映射
func (s *ProductMappingService) SetActive(id uint, active bool) error {
	mapping, err := s.mappingRepo.GetByID(id)
	if err != nil {
		return err
	}
	if mapping == nil {
		return ErrMappingNotFound
	}
	mapping.IsActive = active
	return s.mappingRepo.Update(mapping)
}

// Delete 删除映射（不删除本地商品）
func (s *ProductMappingService) Delete(id uint) error {
	mapping, err := s.mappingRepo.GetByID(id)
	if err != nil {
		return err
	}
	if mapping == nil {
		return ErrMappingNotFound
	}

	// 删除 SKU 映射
	if err := s.skuMappingRepo.DeleteByProductMapping(id); err != nil {
		return err
	}

	// 还原本地商品状态：取消映射标记、交付类型改回 manual、自动下架
	if mapping.LocalProductID > 0 {
		localProduct, err := s.productRepo.GetByID(strconv.FormatUint(uint64(mapping.LocalProductID), 10))
		if err == nil && localProduct != nil {
			localProduct.IsMapped = false
			if localProduct.FulfillmentType == constants.FulfillmentTypeUpstream {
				localProduct.FulfillmentType = constants.FulfillmentTypeManual
				localProduct.IsActive = false // 下架，防止用户下单后无法交付
			}
			_ = s.productRepo.Update(localProduct)
		}
	}

	return s.mappingRepo.Delete(id)
}

// GetSKUMappings 获取映射的 SKU 映射列表
func (s *ProductMappingService) GetSKUMappings(mappingID uint) ([]models.SKUMapping, error) {
	return s.skuMappingRepo.ListByProductMapping(mappingID)
}

// GetMappedUpstreamIDs 获取指定连接下所有已映射的上游商品 ID
func (s *ProductMappingService) GetMappedUpstreamIDs(connectionID uint) ([]uint, error) {
	return s.mappingRepo.ListUpstreamIDsByConnection(connectionID)
}
