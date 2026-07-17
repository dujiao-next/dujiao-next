package admin

import (
	"errors"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

// GetProductMappings 获取商品映射列表
func (h *Handler) GetProductMappings(c *gin.Context) {
	page, pageSize := shared.ParsePagination(c)

	connectionID, _ := shared.ParseQueryUint(c.Query("connection_id"), false)
	upstreamStatus := c.Query("upstream_status")
	if upstreamStatus != "" {
		validStatuses := map[string]bool{
			models.UpstreamStatusActive:   true,
			models.UpstreamStatusInactive: true,
			models.UpstreamStatusDeleted:  true,
		}
		if !validStatuses[upstreamStatus] {
			shared.RespondError(c, response.CodeBadRequest, "error.invalid_upstream_status", nil)
			return
		}
	}
	productStatus := c.Query("product_status")
	if productStatus != "" && productStatus != "active" && productStatus != "inactive" {
		shared.RespondError(c, response.CodeBadRequest, "error.invalid_product_status", nil)
		return
	}
	search := c.Query("search")

	mappings, total, err := h.ProductMappingService.List(repository.ProductMappingListFilter{
		ConnectionID:   connectionID,
		UpstreamStatus: upstreamStatus,
		ProductStatus:  productStatus,
		Search:         search,
		Pagination: repository.Pagination{
			Page:     page,
			PageSize: pageSize,
		},
	})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.mapping_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, mappings, pagination)
}

// GetProductMapping 获取商品映射详情
func (h *Handler) GetProductMapping(c *gin.Context) {
	id, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	mapping, err := h.ProductMappingService.GetByID(id)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.mapping_fetch_failed", err)
		return
	}
	if mapping == nil {
		shared.RespondError(c, response.CodeNotFound, "error.mapping_not_found", nil)
		return
	}

	// 同时返回 SKU 映射
	skuMappings, err := h.ProductMappingService.GetSKUMappings(mapping.ID)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.mapping_fetch_failed", err)
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
func (h *Handler) ImportUpstreamProduct(c *gin.Context) {
	var req ImportUpstreamProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	mapping, err := h.ProductMappingService.ImportUpstreamProductWithAutoCategory(
		req.ConnectionID,
		req.UpstreamProductID,
		req.CategoryID,
		req.Slug,
		req.AutoCreateCategory,
	)
	if err != nil {
		if errors.Is(err, service.ErrMappingAlreadyExists) {
			shared.RespondError(c, response.CodeBadRequest, "error.mapping_already_exists", nil)
			return
		}
		if errors.Is(err, service.ErrConnectionNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrUpstreamProductNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.upstream_product_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrSlugExists) {
			shared.RespondError(c, response.CodeBadRequest, "error.slug_exists", nil)
			return
		}
		if errors.Is(err, service.ErrProductCategoryInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.product_category_invalid", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.mapping_import_failed", err)
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
func (h *Handler) BatchImportUpstreamProducts(c *gin.Context) {
	var req BatchImportUpstreamProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	outcomes, err := h.ProductMappingService.BatchImportUpstreamProducts(
		req.ConnectionID,
		req.UpstreamProductIDs,
		req.CategoryID,
		req.AutoCreateCategory,
	)
	if err != nil {
		if errors.Is(err, service.ErrConnectionNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.mapping_import_failed", err)
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
func (h *Handler) BatchSyncProductMappings(c *gin.Context) {
	var req BatchMappingActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	successCount := 0
	for _, id := range req.IDs {
		if err := h.ProductMappingService.SyncProduct(id); err == nil {
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
func (h *Handler) BatchUpdateProductMappingStatus(c *gin.Context) {
	var req BatchUpdateMappingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	successCount := 0
	for _, id := range req.IDs {
		if err := h.ProductMappingService.SetActive(id, req.IsActive); err == nil {
			successCount++
		}
	}

	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount})
}

// BatchDeleteProductMappings 批量删除
func (h *Handler) BatchDeleteProductMappings(c *gin.Context) {
	var req BatchMappingActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	successCount := 0
	for _, id := range req.IDs {
		if err := h.ProductMappingService.Delete(id); err == nil {
			successCount++
		}
	}

	response.Success(c, gin.H{"total": len(req.IDs), "success_count": successCount})
}

// SyncProductMapping 同步商品映射
func (h *Handler) SyncProductMapping(c *gin.Context) {
	id, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	if err := h.ProductMappingService.SyncProduct(id); err != nil {
		if errors.Is(err, service.ErrMappingNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.mapping_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.mapping_sync_failed", err)
		return
	}

	response.Success(c, gin.H{"synced": true})
}

// UpdateProductMappingStatusRequest 更新映射状态请求
type UpdateProductMappingStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// UpdateProductMappingStatus 启用/禁用映射
func (h *Handler) UpdateProductMappingStatus(c *gin.Context) {
	id, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	var req UpdateProductMappingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	if err := h.ProductMappingService.SetActive(id, req.IsActive); err != nil {
		if errors.Is(err, service.ErrMappingNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.mapping_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.mapping_update_failed", err)
		return
	}

	response.Success(c, gin.H{"updated": true})
}

// DeleteProductMapping 删除映射
func (h *Handler) DeleteProductMapping(c *gin.Context) {
	id, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	if err := h.ProductMappingService.Delete(id); err != nil {
		if errors.Is(err, service.ErrMappingNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.mapping_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.mapping_delete_failed", err)
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

// ListUpstreamProducts 代理拉取上游商品列表
func (h *Handler) ListUpstreamProducts(c *gin.Context) {
	connectionID, err := shared.ParseQueryUint(c.Query("connection_id"), true)
	if err != nil || connectionID == 0 {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	page, pageSize := shared.ParsePaginationWithKeys(c, "page", "page_size", 50)

	result, err := h.ProductMappingService.ListUpstreamProducts(connectionID, page, pageSize)
	if err != nil {
		if errors.Is(err, service.ErrConnectionNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.upstream_products_fetch_failed", err)
		return
	}

	// 查询已映射的上游商品 ID（仅首页时返回，避免重复查询）
	var mappedIDs []uint
	if page == 1 {
		mappedIDs, _ = h.ProductMappingService.GetMappedUpstreamIDs(connectionID)
	}

	response.Success(c, gin.H{
		"items":      result.Items,
		"total":      result.Total,
		"mapped_ids": mappedIDs,
	})
}

// ListUpstreamCategories 获取上游分类列表
func (h *Handler) ListUpstreamCategories(c *gin.Context) {
	connectionID, err := shared.ParseQueryUint(c.Query("connection_id"), true)
	if err != nil || connectionID == 0 {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	categories, supported, err := h.ProductMappingService.ListUpstreamCategories(connectionID)
	if err != nil {
		if errors.Is(err, service.ErrConnectionNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.upstream_categories_fetch_failed", err)
		return
	}

	response.Success(c, gin.H{
		"supported":  supported,
		"categories": categories,
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
func (h *Handler) BatchImportByCategory(c *gin.Context) {
	var req BatchImportByCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	result, err := h.ProductMappingService.BatchImportByCategory(
		req.ConnectionID,
		req.UpstreamCategoryID,
		req.AutoCreateCategory,
		req.LocalCategoryID,
	)
	if err != nil {
		if errors.Is(err, service.ErrConnectionNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.category_import_failed", err)
		return
	}

	response.Success(c, result)
}
