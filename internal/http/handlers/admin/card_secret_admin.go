package admin

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

// CreateCardSecretBatchRequest 批量录入卡密请求
type CreateCardSecretBatchRequest struct {
	ProductID           uint     `json:"product_id" binding:"required"`
	SKUID               uint     `json:"sku_id"`
	Secrets             []string `json:"secrets" binding:"required"`
	BatchNo             string   `json:"batch_no"`
	Note                string   `json:"note"`
	OverwriteDuplicates bool     `json:"overwrite_duplicates"`
}

// UpdateCardSecretRequest 更新卡密请求
type UpdateCardSecretRequest struct {
	Secret *string `json:"secret"`
	Status *string `json:"status"`
}

// CardSecretQueryRequest 卡密查询条件
type CardSecretQueryRequest struct {
	ProductID uint   `json:"product_id"`
	SKUID     uint   `json:"sku_id"`
	BatchID   uint   `json:"batch_id"`
	Status    string `json:"status"`
	Secret    string `json:"secret"`
	BatchNo   string `json:"batch_no"`
}

// BatchUpdateCardSecretStatusRequest 批量更新卡密状态请求
type BatchUpdateCardSecretStatusRequest struct {
	IDs     []uint                  `json:"ids"`
	BatchID uint                    `json:"batch_id"`
	Filter  *CardSecretQueryRequest `json:"filter"`
	Status  string                  `json:"status" binding:"required"`
}

// BatchDeleteCardSecretRequest 批量删除卡密请求
type BatchDeleteCardSecretRequest struct {
	IDs     []uint                  `json:"ids"`
	BatchID uint                    `json:"batch_id"`
	Filter  *CardSecretQueryRequest `json:"filter"`
}

// ExportCardSecretRequest 批量导出卡密请求
type ExportCardSecretRequest struct {
	IDs     []uint                  `json:"ids"`
	BatchID uint                    `json:"batch_id"`
	Filter  *CardSecretQueryRequest `json:"filter"`
	Format  string                  `json:"format" binding:"required"`
}

// ExportAvailableCardSecretRequest 可用卡密出库导出请求
type ExportAvailableCardSecretRequest struct {
	ProductID         uint   `json:"product_id" binding:"required"`
	SKUID             uint   `json:"sku_id"`
	BatchID           uint   `json:"batch_id"`
	Limit             int    `json:"limit" binding:"required"`
	Format            string `json:"format" binding:"required"`
	DeleteAfterExport bool   `json:"delete_after_export"`
}

func buildCardSecretListInput(filter *CardSecretQueryRequest) service.ListCardSecretInput {
	if filter == nil {
		return service.ListCardSecretInput{}
	}
	return service.ListCardSecretInput{
		ProductID: filter.ProductID,
		SKUID:     filter.SKUID,
		BatchID:   filter.BatchID,
		Status:    strings.TrimSpace(filter.Status),
		Secret:    strings.TrimSpace(filter.Secret),
		BatchNo:   strings.TrimSpace(filter.BatchNo),
	}
}

func respondCardSecretDuplicateConfirmation(c *gin.Context, err error) bool {
	var duplicateErr *service.CardSecretDuplicatesError
	if !errors.As(err, &duplicateErr) {
		return false
	}
	response.Success(c, gin.H{
		"requires_confirmation": true,
		"duplicate_count":       len(duplicateErr.Secrets),
		"duplicates":            duplicateErr.Secrets,
		"created":               0,
		"imported":              0,
	})
	return true
}

func respondCardSecretImportSuccess(c *gin.Context, batchID uint, batchNo string, imported int) {
	response.Success(c, gin.H{
		"requires_confirmation": false,
		"created":               imported,
		"imported":              imported,
		"batch_id":              batchID,
		"batch_no":              batchNo,
	})
}

// CreateCardSecretBatch 批量录入卡密
func (h *Handler) CreateCardSecretBatch(c *gin.Context) {
	adminID, ok := shared.GetAdminID(c)
	if !ok {
		return
	}
	var req CreateCardSecretBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	batch, imported, err := h.CardSecretService.CreateCardSecretBatch(service.CreateCardSecretBatchInput{
		ProductID:           req.ProductID,
		SKUID:               req.SKUID,
		Secrets:             req.Secrets,
		BatchNo:             req.BatchNo,
		Note:                req.Note,
		Source:              constants.CardSecretSourceManual,
		AdminID:             adminID,
		OverwriteDuplicates: req.OverwriteDuplicates,
	})
	if err != nil {
		if respondCardSecretDuplicateConfirmation(c, err) {
			return
		}
		switch {
		case errors.Is(err, service.ErrProductSKURequired):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrProductSKUInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrCardSecretInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrProductNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
		case errors.Is(err, service.ErrProductFetchFailed):
			shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		case errors.Is(err, service.ErrCardSecretFetchFailed):
			shared.RespondError(c, response.CodeInternal, "error.card_secret_fetch_failed", err)
		case errors.Is(err, service.ErrCardSecretBatchCreateFailed):
			shared.RespondError(c, response.CodeInternal, "error.card_secret_batch_create_failed", err)
		default:
			shared.RespondError(c, response.CodeInternal, "error.card_secret_create_failed", err)
		}
		return
	}

	respondCardSecretImportSuccess(c, batch.ID, batch.BatchNo, imported)
}

// ImportCardSecretCSV 导入 CSV 卡密
func (h *Handler) ImportCardSecretCSV(c *gin.Context) {
	adminID, ok := shared.GetAdminID(c)
	if !ok {
		return
	}
	productID, err := shared.ParseQueryUint(c.PostForm("product_id"), true)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	skuID, err := shared.ParseQueryUint(c.DefaultPostForm("sku_id", "0"), false)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	batchNo := strings.TrimSpace(c.PostForm("batch_no"))
	note := strings.TrimSpace(c.PostForm("note"))
	overwriteDuplicates, err := shared.ParseOptionalBoolValue(c.PostForm("overwrite_duplicates"))
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}

	batch, imported, err := h.CardSecretService.ImportCardSecretCSV(service.ImportCardSecretCSVInput{
		ProductID:           productID,
		SKUID:               skuID,
		File:                file,
		BatchNo:             batchNo,
		Note:                note,
		AdminID:             adminID,
		OverwriteDuplicates: overwriteDuplicates != nil && *overwriteDuplicates,
	})
	if err != nil {
		if respondCardSecretDuplicateConfirmation(c, err) {
			return
		}
		switch {
		case errors.Is(err, service.ErrProductSKURequired):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrProductSKUInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrCardSecretInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrProductNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
		case errors.Is(err, service.ErrProductFetchFailed):
			shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		case errors.Is(err, service.ErrCardSecretFetchFailed):
			shared.RespondError(c, response.CodeInternal, "error.card_secret_fetch_failed", err)
		case errors.Is(err, service.ErrCardSecretBatchCreateFailed):
			shared.RespondError(c, response.CodeInternal, "error.card_secret_batch_create_failed", err)
		default:
			shared.RespondError(c, response.CodeInternal, "error.card_secret_import_failed", err)
		}
		return
	}

	respondCardSecretImportSuccess(c, batch.ID, batch.BatchNo, imported)
}

// GetCardSecrets 获取卡密列表
func (h *Handler) GetCardSecrets(c *gin.Context) {
	var productID uint
	rawProductID := strings.TrimSpace(c.Query("product_id"))
	if rawProductID != "" {
		parsed, err := shared.ParseQueryUint(rawProductID, false)
		if err != nil {
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
			return
		}
		productID = parsed
	}
	var skuID uint
	rawSKUID := strings.TrimSpace(c.Query("sku_id"))
	if rawSKUID != "" {
		parsed, err := shared.ParseQueryUint(rawSKUID, false)
		if err != nil {
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
			return
		}
		skuID = parsed
	}
	var batchID uint
	rawBatchID := strings.TrimSpace(c.Query("batch_id"))
	if rawBatchID != "" {
		parsed, err := shared.ParseQueryUint(rawBatchID, false)
		if err != nil {
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
			return
		}
		batchID = parsed
	}
	page, pageSize := shared.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))
	secret := strings.TrimSpace(c.Query("secret"))
	batchNo := strings.TrimSpace(c.Query("batch_no"))

	items, total, err := h.CardSecretService.ListCardSecrets(service.ListCardSecretInput{
		ProductID: productID,
		SKUID:     skuID,
		BatchID:   batchID,
		Status:    status,
		Secret:    secret,
		BatchNo:   batchNo,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProductSKURequired):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrProductSKUInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrCardSecretInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.card_secret_fetch_failed", err)
		}
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, items, pagination)
}

// UpdateCardSecret 更新卡密
func (h *Handler) UpdateCardSecret(c *gin.Context) {
	rawID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}

	var req UpdateCardSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	secret := ""
	if req.Secret != nil {
		secret = *req.Secret
	}
	status := ""
	if req.Status != nil {
		status = *req.Status
	}
	if strings.TrimSpace(secret) == "" && strings.TrimSpace(status) == "" {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	item, err := h.CardSecretService.UpdateCardSecret(rawID, secret, status)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.card_secret_not_found", nil)
		case errors.Is(err, service.ErrCardSecretInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrCardSecretUpdateFailed):
			shared.RespondError(c, response.CodeInternal, "error.card_secret_update_failed", err)
		default:
			shared.RespondError(c, response.CodeInternal, "error.card_secret_update_failed", err)
		}
		return
	}

	response.Success(c, item)
}

// BatchUpdateCardSecretStatus 批量更新卡密状态
func (h *Handler) BatchUpdateCardSecretStatus(c *gin.Context) {
	var req BatchUpdateCardSecretStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	rows, err := h.CardSecretService.BatchUpdateCardSecretStatus(req.IDs, req.BatchID, buildCardSecretListInput(req.Filter), req.Status)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCardSecretInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.card_secret_not_found", nil)
		case errors.Is(err, service.ErrCardSecretUpdateFailed):
			shared.RespondError(c, response.CodeInternal, "error.card_secret_update_failed", err)
		default:
			shared.RespondError(c, response.CodeInternal, "error.card_secret_update_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"affected": rows,
	})
}

// BatchDeleteCardSecrets 批量删除卡密
func (h *Handler) BatchDeleteCardSecrets(c *gin.Context) {
	var req BatchDeleteCardSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	rows, err := h.CardSecretService.BatchDeleteCardSecrets(req.IDs, req.BatchID, buildCardSecretListInput(req.Filter))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCardSecretInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.card_secret_not_found", nil)
		case errors.Is(err, service.ErrCardSecretDeleteFailed):
			shared.RespondError(c, response.CodeInternal, "error.card_secret_delete_failed", err)
		default:
			shared.RespondError(c, response.CodeInternal, "error.card_secret_delete_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"affected": rows,
	})
}

// ExportCardSecrets 批量导出卡密
func (h *Handler) ExportCardSecrets(c *gin.Context) {
	var req ExportCardSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	content, contentType, err := h.CardSecretService.ExportCardSecrets(req.IDs, req.BatchID, buildCardSecretListInput(req.Filter), req.Format)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCardSecretInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.card_secret_not_found", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.card_secret_fetch_failed", err)
		}
		return
	}

	normalizedFormat := strings.ToLower(strings.TrimSpace(req.Format))
	filename := "card-secrets-" + time.Now().Format("20060102-150405") + "." + normalizedFormat
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(200, contentType, content)
}

// ExportAvailableCardSecrets 从可用库存中导出卡密并出库
func (h *Handler) ExportAvailableCardSecrets(c *gin.Context) {
	adminID, ok := shared.GetAdminID(c)
	if !ok {
		return
	}
	var req ExportAvailableCardSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	result, err := h.CardSecretService.ExportAvailableCardSecrets(service.ExportAvailableCardSecretInput{
		ProductID:         req.ProductID,
		SKUID:             req.SKUID,
		BatchID:           req.BatchID,
		AdminID:           adminID,
		Limit:             req.Limit,
		Format:            req.Format,
		DeleteAfterExport: req.DeleteAfterExport,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCardSecretInvalid),
			errors.Is(err, service.ErrProductSKUInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrProductNotFound),
			errors.Is(err, service.ErrNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.card_secret_not_found", nil)
		case errors.Is(err, service.ErrCardSecretInsufficient):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_insufficient", nil)
		case errors.Is(err, service.ErrCardSecretUpdateFailed):
			shared.RespondError(c, response.CodeInternal, "error.card_secret_update_failed", err)
		case errors.Is(err, service.ErrCardSecretDeleteFailed):
			shared.RespondError(c, response.CodeInternal, "error.card_secret_delete_failed", err)
		default:
			shared.RespondError(c, response.CodeInternal, "error.card_secret_fetch_failed", err)
		}
		return
	}

	normalizedFormat := strings.ToLower(strings.TrimSpace(req.Format))
	filename := "card-secrets-available-" + time.Now().Format("20060102-150405") + "." + normalizedFormat
	c.Header("Content-Type", result.ContentType)
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Header("X-Exported-Count", strconv.Itoa(result.Count))
	if result.Record != nil {
		c.Header("X-Export-Record-ID", strconv.FormatUint(uint64(result.Record.ID), 10))
	}
	c.Data(200, result.ContentType, result.Content)
}

// GetCardSecretExports 获取卡密出库导出记录。
func (h *Handler) GetCardSecretExports(c *gin.Context) {
	page, pageSize := shared.ParsePagination(c)
	parseOptional := func(key string) (uint, bool) {
		raw := strings.TrimSpace(c.Query(key))
		if raw == "" {
			return 0, true
		}
		value, err := shared.ParseQueryUint(raw, false)
		if err != nil {
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
			return 0, false
		}
		return value, true
	}
	id, ok := parseOptional("id")
	if !ok {
		return
	}
	productID, ok := parseOptional("product_id")
	if !ok {
		return
	}
	skuID, ok := parseOptional("sku_id")
	if !ok {
		return
	}
	batchID, ok := parseOptional("batch_id")
	if !ok {
		return
	}
	rows, total, err := h.CardSecretService.ListCardSecretExports(service.ListCardSecretExportInput{
		ID:        id,
		ProductID: productID,
		SKUID:     skuID,
		BatchID:   batchID,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.card_secret_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}

// GetCardSecretStats 获取库存统计
func (h *Handler) GetCardSecretStats(c *gin.Context) {
	productID, err := shared.ParseQueryUint(c.Query("product_id"), true)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	skuID, err := shared.ParseQueryUint(c.DefaultQuery("sku_id", "0"), false)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	stats, err := h.CardSecretService.GetStats(productID, skuID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProductSKURequired):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrProductSKUInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrCardSecretInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.card_secret_stats_failed", err)
		}
		return
	}
	response.Success(c, stats)
}

// GetCardSecretBatches 获取卡密批次列表
func (h *Handler) GetCardSecretBatches(c *gin.Context) {
	productID, err := shared.ParseQueryUint(c.Query("product_id"), true)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	page, pageSize := shared.ParsePagination(c)
	skuID, err := shared.ParseQueryUint(c.DefaultQuery("sku_id", "0"), false)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}

	items, total, err := h.CardSecretService.ListBatches(productID, skuID, page, pageSize)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProductSKURequired):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrProductSKUInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, service.ErrCardSecretInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.card_secret_batch_fetch_failed", err)
		}
		return
	}
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, items, pagination)
}

// GetCardSecretTemplate 下载导入模板
func (h *Handler) GetCardSecretTemplate(c *gin.Context) {
	content := "secret\nCARD-AAA-0001\nCARD-BBB-0002\n"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\"card-secrets-template.csv\"")
	c.String(200, content)
}
