package mappinghttp

import (
	"context"
	"errors"

	mappingapp "github.com/dujiao-next/internal/modules/catalog/mapping/application"
	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/upstream"

	"github.com/gin-gonic/gin"
)

// ProductMappingService 是后台商品映射端点所需的最小用例接口。
type ProductMappingService interface {
	List(filter mappingcontract.ListFilter) ([]mappingdomain.Mapping, int64, error)
	GetByID(id uint) (*mappingdomain.Mapping, error)
	GetSKUMappings(mappingID uint) ([]mappingdomain.SKUMapping, error)
	ImportUpstreamProductWithAutoCategory(connectionID, upstreamProductID, categoryID uint, slug string, autoCreateCategory bool) (*mappingdomain.Mapping, error)
	BatchImportUpstreamProducts(connectionID uint, upstreamProductIDs []uint, categoryID uint, autoCreateCategory bool) ([]mappingapp.BatchUpstreamProductImportOutcome, error)
	SyncProduct(mappingID uint) error
	SetActive(id uint, active bool) error
	Delete(id uint) error
	ListUpstreamProducts(connectionID uint, page, pageSize int) (*upstream.ProductListResult, error)
	GetMappedUpstreamIDs(connectionID uint) ([]uint, error)
	ListUpstreamCategories(connectionID uint) ([]upstream.UpstreamCategory, bool, error)
	ListUpstreamCategoryCounts(ctx context.Context, connectionID uint) (map[uint]int, int, error)
	BatchImportByCategory(connectionID, upstreamCategoryID uint, autoCreateCategory bool, localCategoryID uint) (*mappingapp.BatchImportByCategoryResult, error)
}

// AdminHandler 处理后台商品映射管理请求。
type AdminHandler struct {
	service ProductMappingService
}

// NewAdminHandler 创建后台商品映射 Handler。
func NewAdminHandler(service ProductMappingService) *AdminHandler {
	if service == nil {
		panic("catalog admin product mapping handler: service is nil")
	}
	return &AdminHandler{service: service}
}

// GetProductMappings 获取商品映射列表
func (h *AdminHandler) GetProductMappings(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	connectionID, _ := ginutil.ParseQueryUint(c.Query("connection_id"), false)
	upstreamStatus := c.Query("upstream_status")
	if upstreamStatus != "" {
		validStatuses := map[string]bool{
			mappingdomain.UpstreamStatusActive:   true,
			mappingdomain.UpstreamStatusInactive: true,
			mappingdomain.UpstreamStatusDeleted:  true,
		}
		if !validStatuses[upstreamStatus] {
			ginutil.RespondError(c, response.CodeBadRequest, "error.invalid_upstream_status", nil)
			return
		}
	}
	productStatus := c.Query("product_status")
	if productStatus != "" && productStatus != "active" && productStatus != "inactive" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invalid_product_status", nil)
		return
	}
	search := c.Query("search")

	mappings, total, err := h.service.List(mappingcontract.ListFilter{
		ConnectionID:   connectionID,
		UpstreamStatus: upstreamStatus,
		ProductStatus:  productStatus,
		Search:         search,
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.mapping_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, mappings, pagination)
}

// GetProductMapping 获取商品映射详情
func (h *AdminHandler) GetProductMapping(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	mapping, err := h.service.GetByID(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.mapping_fetch_failed", err)
		return
	}
	if mapping == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.mapping_not_found", nil)
		return
	}

	// 同时返回 SKU 映射
	skuMappings, err := h.service.GetSKUMappings(mapping.ID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.mapping_fetch_failed", err)
		return
	}

	response.Success(c, gin.H{
		"mapping":      mapping,
		"sku_mappings": skuMappings,
	})
}

// ImportUpstreamProductRequest 导入上游商品请求
type ImportUpstreamProductRequest struct {
	ConnectionID       uint   `json:"connection_id" binding:"required"`
	UpstreamProductID  uint   `json:"upstream_product_id" binding:"required"`
	CategoryID         uint   `json:"category_id"`
	Slug               string `json:"slug"`
	AutoCreateCategory bool   `json:"auto_create_category"`
}

// ImportUpstreamProduct 导入上游商品
func (h *AdminHandler) ImportUpstreamProduct(c *gin.Context) {
	var req ImportUpstreamProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	mapping, err := h.service.ImportUpstreamProductWithAutoCategory(
		req.ConnectionID,
		req.UpstreamProductID,
		req.CategoryID,
		req.Slug,
		req.AutoCreateCategory,
	)
	if err != nil {
		if errors.Is(err, mappingcontract.ErrMappingAlreadyExists) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.mapping_already_exists", nil)
			return
		}
		if errors.Is(err, siteconnectioncontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		if errors.Is(err, mappingcontract.ErrUpstreamProductNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.upstream_product_not_found", nil)
			return
		}
		if errors.Is(err, productcontract.ErrSlugExists) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.slug_exists", nil)
			return
		}
		if errors.Is(err, productcontract.ErrProductCategoryInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_category_invalid", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.mapping_import_failed", err)
		return
	}

	response.Success(c, mapping)
}

// BatchImportUpstreamProductRequest 批量导入上游商品请求
type BatchImportUpstreamProductRequest struct {
	ConnectionID       uint   `json:"connection_id" binding:"required"`
	UpstreamProductIDs []uint `json:"upstream_product_ids" binding:"required,min=1"`
	CategoryID         uint   `json:"category_id"`
	AutoCreateCategory bool   `json:"auto_create_category"`
}

// BatchImportUpstreamProductResult 单个商品导入结果
type BatchImportUpstreamProductResult struct {
	UpstreamProductID uint   `json:"upstream_product_id"`
	Success           bool   `json:"success"`
	Error             string `json:"error,omitempty"`
}

// BatchImportUpstreamProducts 批量导入上游商品
func (h *AdminHandler) BatchImportUpstreamProducts(c *gin.Context) {
	var req BatchImportUpstreamProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	outcomes, err := h.service.BatchImportUpstreamProducts(
		req.ConnectionID,
		req.UpstreamProductIDs,
		req.CategoryID,
		req.AutoCreateCategory,
	)
	if err != nil {
		if errors.Is(err, siteconnectioncontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		if errors.Is(err, mappingcontract.ErrBatchImportInProgress) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.mapping_batch_import_in_progress", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.mapping_import_failed", err)
		return
	}

	results := make([]BatchImportUpstreamProductResult, len(outcomes))
	successCount := 0
	for i, o := range outcomes {
		item := BatchImportUpstreamProductResult{UpstreamProductID: o.UpstreamProductID}
		if o.Err != nil {
			item.Error = o.Err.Error()
		} else {
			item.Success = true
			successCount++
		}
		results[i] = item
	}

	response.Success(c, gin.H{
		"results":       results,
		"total":         len(req.UpstreamProductIDs),
		"success_count": successCount,
	})
}

// BatchMappingActionRequest 批量操作请求
type BatchMappingActionRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}

// BatchSyncProductMappings 批量同步
func (h *AdminHandler) BatchSyncProductMappings(c *gin.Context) {
	var req BatchMappingActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	successCount := 0
	for _, id := range req.IDs {
		if err := h.service.SyncProduct(id); err == nil {
			successCount++
		}
	}

	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount})
}

// BatchUpdateMappingStatusRequest 批量更新状态请求
type BatchUpdateMappingStatusRequest struct {
	IDs      []uint `json:"ids" binding:"required,min=1"`
	IsActive bool   `json:"is_active"`
}

// BatchUpdateProductMappingStatus 批量启用/禁用
func (h *AdminHandler) BatchUpdateProductMappingStatus(c *gin.Context) {
	var req BatchUpdateMappingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	successCount := 0
	for _, id := range req.IDs {
		if err := h.service.SetActive(id, req.IsActive); err == nil {
			successCount++
		}
	}

	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount})
}

// BatchDeleteProductMappings 批量删除
func (h *AdminHandler) BatchDeleteProductMappings(c *gin.Context) {
	var req BatchMappingActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	successCount := 0
	for _, id := range req.IDs {
		if err := h.service.Delete(id); err == nil {
			successCount++
		}
	}

	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount})
}

// SyncProductMapping 同步商品映射
func (h *AdminHandler) SyncProductMapping(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	if err := h.service.SyncProduct(id); err != nil {
		if errors.Is(err, mappingcontract.ErrMappingNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.mapping_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.mapping_sync_failed", err)
		return
	}

	response.Success(c, gin.H{"synced": true})
}

// UpdateProductMappingStatusRequest 更新映射状态请求
type UpdateProductMappingStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// UpdateProductMappingStatus 启用/禁用映射
func (h *AdminHandler) UpdateProductMappingStatus(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	var req UpdateProductMappingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.service.SetActive(id, req.IsActive); err != nil {
		if errors.Is(err, mappingcontract.ErrMappingNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.mapping_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.mapping_update_failed", err)
		return
	}

	response.Success(c, gin.H{"updated": true})
}

// DeleteProductMapping 删除映射
func (h *AdminHandler) DeleteProductMapping(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	if err := h.service.Delete(id); err != nil {
		if errors.Is(err, mappingcontract.ErrMappingNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.mapping_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.mapping_delete_failed", err)
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

// ListUpstreamProducts 代理拉取上游商品列表
func (h *AdminHandler) ListUpstreamProducts(c *gin.Context) {
	connectionID, err := ginutil.ParseQueryUint(c.Query("connection_id"), true)
	if err != nil || connectionID == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	page, pageSize := ginutil.ParsePaginationWithKeys(c, "page", "page_size", 50)

	result, err := h.service.ListUpstreamProducts(connectionID, page, pageSize)
	if err != nil {
		if errors.Is(err, siteconnectioncontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.upstream_products_fetch_failed", err)
		return
	}

	// 查询已映射的上游商品 ID（仅首页时返回，避免重复查询）
	var mappedIDs []uint
	if page == 1 {
		mappedIDs, _ = h.service.GetMappedUpstreamIDs(connectionID)
	}

	response.Success(c, gin.H{
		"items":      result.Items,
		"total":      result.Total,
		"mapped_ids": mappedIDs,
	})
}

// ListUpstreamCategories 获取上游分类列表
func (h *AdminHandler) ListUpstreamCategories(c *gin.Context) {
	connectionID, err := ginutil.ParseQueryUint(c.Query("connection_id"), true)
	if err != nil || connectionID == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	categories, supported, err := h.service.ListUpstreamCategories(connectionID)
	if err != nil {
		if errors.Is(err, siteconnectioncontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.upstream_categories_fetch_failed", err)
		return
	}

	response.Success(c, gin.H{
		"supported":  supported,
		"categories": categories,
	})
}

// ListUpstreamCategoryCounts 统计上游各分类下的商品数量（遍历全部分页，不受管理端已加载页数影响）
func (h *AdminHandler) ListUpstreamCategoryCounts(c *gin.Context) {
	connectionID, err := ginutil.ParseQueryUint(c.Query("connection_id"), true)
	if err != nil || connectionID == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	counts, total, err := h.service.ListUpstreamCategoryCounts(c.Request.Context(), connectionID)
	if err != nil {
		if errors.Is(err, siteconnectioncontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.upstream_products_fetch_failed", err)
		return
	}

	response.Success(c, gin.H{
		"counts": counts,
		"total":  total,
	})
}

// BatchImportByCategoryRequest 按分类批量导入请求
type BatchImportByCategoryRequest struct {
	ConnectionID       uint `json:"connection_id" binding:"required"`
	UpstreamCategoryID uint `json:"upstream_category_id" binding:"required"`
	AutoCreateCategory bool `json:"auto_create_category"`
	LocalCategoryID    uint `json:"local_category_id"`
}

// BatchImportByCategory 按上游分类批量导入
func (h *AdminHandler) BatchImportByCategory(c *gin.Context) {
	var req BatchImportByCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	result, err := h.service.BatchImportByCategory(
		req.ConnectionID,
		req.UpstreamCategoryID,
		req.AutoCreateCategory,
		req.LocalCategoryID,
	)
	if err != nil {
		if errors.Is(err, siteconnectioncontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		if errors.Is(err, mappingcontract.ErrBatchImportInProgress) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.mapping_batch_import_in_progress", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.category_import_failed", err)
		return
	}

	response.Success(c, result)
}
