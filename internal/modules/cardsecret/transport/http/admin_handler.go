package cardsecrethttp

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	cardsecretapp "github.com/dujiao-next/internal/modules/cardsecret/application"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type Service interface {
	CreateCardSecretBatch(cardsecretapp.CreateCardSecretBatchInput) (*cardsecretdomain.Batch, int, error)
	ImportCardSecretCSV(cardsecretapp.ImportCardSecretCSVInput) (*cardsecretdomain.Batch, int, error)
	ListCardSecrets(cardsecretapp.ListCardSecretInput) ([]cardsecretdomain.Secret, int64, error)
	UpdateCardSecret(id uint, secret, status string) (*cardsecretdomain.Secret, error)
	BatchUpdateCardSecretStatus(ids []uint, batchID uint, filter cardsecretapp.ListCardSecretInput, status string) (int64, error)
	BatchDeleteCardSecrets(ids []uint, batchID uint, filter cardsecretapp.ListCardSecretInput) (int64, error)
	ExportCardSecrets(ids []uint, batchID uint, filter cardsecretapp.ListCardSecretInput, format string) ([]byte, string, error)
	ExportAvailableCardSecrets(cardsecretapp.ExportAvailableCardSecretInput) (*cardsecretapp.ExportAvailableCardSecretResult, error)
	GetStats(productID, skuID uint) (*cardsecretapp.CardSecretStats, error)
	ListBatches(productID, skuID uint, page, pageSize int) ([]cardsecretapp.CardSecretBatchSummary, int64, error)
}

type AdminHandler struct {
	service Service
}

func NewAdminHandler(service Service) *AdminHandler {
	if service == nil {
		panic("card secret admin handler: required dependency is nil")
	}
	return &AdminHandler{service: service}
}

// CreateCardSecretBatchRequest 批量录入卡密请求
type CreateCardSecretBatchRequest struct {
	ProductID   uint     `json:"product_id" binding:"required"`
	SKUID       uint     `json:"sku_id"`
	Secrets     []string `json:"secrets" binding:"required"`
	BatchNo     string   `json:"batch_no"`
	Note        string   `json:"note"`
	Deduplicate *bool    `json:"deduplicate"`
	IsLoop      bool     `json:"is_loop"`
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

func buildCardSecretListInput(filter *CardSecretQueryRequest) cardsecretapp.ListCardSecretInput {
	if filter == nil {
		return cardsecretapp.ListCardSecretInput{}
	}
	return cardsecretapp.ListCardSecretInput{
		ProductID: filter.ProductID,
		SKUID:     filter.SKUID,
		BatchID:   filter.BatchID,
		Status:    strings.TrimSpace(filter.Status),
		Secret:    strings.TrimSpace(filter.Secret),
		BatchNo:   strings.TrimSpace(filter.BatchNo),
	}
}

// CreateCardSecretBatch 批量录入卡密
func (h *AdminHandler) CreateCardSecretBatch(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	var req CreateCardSecretBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	batch, created, err := h.service.CreateCardSecretBatch(cardsecretapp.CreateCardSecretBatchInput{
		ProductID:   req.ProductID,
		SKUID:       req.SKUID,
		Secrets:     req.Secrets,
		BatchNo:     req.BatchNo,
		Note:        req.Note,
		Source:      constants.CardSecretSourceManual,
		AdminID:     adminID,
		Deduplicate: req.Deduplicate,
		IsLoop:      req.IsLoop,
	})
	if err != nil {
		switch {
		case errors.Is(err, cardsecretapp.ErrProductSKURequired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrProductSKUInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrProductNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
		case errors.Is(err, cardsecretapp.ErrProductFetchFailed):
			ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		case errors.Is(err, cardsecretapp.ErrBatchCreateFailed):
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_batch_create_failed", err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_create_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"created":  created,
		"batch_id": batch.ID,
		"batch_no": batch.BatchNo,
	})
}

// ImportCardSecretCSV 导入 CSV 卡密
func (h *AdminHandler) ImportCardSecretCSV(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	productID, err := ginutil.ParseQueryUint(c.PostForm("product_id"), true)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	skuID, err := ginutil.ParseQueryUint(c.DefaultPostForm("sku_id", "0"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	batchNo := strings.TrimSpace(c.PostForm("batch_no"))
	note := strings.TrimSpace(c.PostForm("note"))
	deduplicate, err := ginutil.ParseOptionalBoolValue(c.PostForm("deduplicate"))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	isLoop, err := ginutil.ParseOptionalBoolValue(c.PostForm("is_loop"))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}

	batch, created, err := h.service.ImportCardSecretCSV(cardsecretapp.ImportCardSecretCSVInput{
		ProductID:   productID,
		SKUID:       skuID,
		File:        file,
		BatchNo:     batchNo,
		Note:        note,
		AdminID:     adminID,
		Deduplicate: deduplicate,
		IsLoop:      isLoop != nil && *isLoop,
	})
	if err != nil {
		switch {
		case errors.Is(err, cardsecretapp.ErrProductSKURequired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrProductSKUInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrProductNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
		case errors.Is(err, cardsecretapp.ErrProductFetchFailed):
			ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		case errors.Is(err, cardsecretapp.ErrBatchCreateFailed):
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_batch_create_failed", err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_import_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"created":  created,
		"batch_id": batch.ID,
		"batch_no": batch.BatchNo,
	})
}

// GetCardSecrets 获取卡密列表
func (h *AdminHandler) GetCardSecrets(c *gin.Context) {
	var productID uint
	rawProductID := strings.TrimSpace(c.Query("product_id"))
	if rawProductID != "" {
		parsed, err := ginutil.ParseQueryUint(rawProductID, false)
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
			return
		}
		productID = parsed
	}
	var skuID uint
	rawSKUID := strings.TrimSpace(c.Query("sku_id"))
	if rawSKUID != "" {
		parsed, err := ginutil.ParseQueryUint(rawSKUID, false)
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
			return
		}
		skuID = parsed
	}
	var batchID uint
	rawBatchID := strings.TrimSpace(c.Query("batch_id"))
	if rawBatchID != "" {
		parsed, err := ginutil.ParseQueryUint(rawBatchID, false)
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
			return
		}
		batchID = parsed
	}
	page, pageSize := ginutil.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))
	secret := strings.TrimSpace(c.Query("secret"))
	batchNo := strings.TrimSpace(c.Query("batch_no"))

	items, total, err := h.service.ListCardSecrets(cardsecretapp.ListCardSecretInput{
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
		case errors.Is(err, cardsecretapp.ErrProductSKURequired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrProductSKUInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_fetch_failed", err)
		}
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, items, pagination)
}

// UpdateCardSecret 更新卡密
func (h *AdminHandler) UpdateCardSecret(c *gin.Context) {
	rawID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}

	var req UpdateCardSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
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
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	item, err := h.service.UpdateCardSecret(rawID, secret, status)
	if err != nil {
		switch {
		case errors.Is(err, cardsecretapp.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.card_secret_not_found", nil)
		case errors.Is(err, cardsecretapp.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrUpdateFailed):
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_update_failed", err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_update_failed", err)
		}
		return
	}

	response.Success(c, item)
}

// BatchUpdateCardSecretStatus 批量更新卡密状态
func (h *AdminHandler) BatchUpdateCardSecretStatus(c *gin.Context) {
	var req BatchUpdateCardSecretStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	rows, err := h.service.BatchUpdateCardSecretStatus(req.IDs, req.BatchID, buildCardSecretListInput(req.Filter), req.Status)
	if err != nil {
		switch {
		case errors.Is(err, cardsecretapp.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.card_secret_not_found", nil)
		case errors.Is(err, cardsecretapp.ErrUpdateFailed):
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_update_failed", err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_update_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"affected": rows,
	})
}

// BatchDeleteCardSecrets 批量删除卡密
func (h *AdminHandler) BatchDeleteCardSecrets(c *gin.Context) {
	var req BatchDeleteCardSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	rows, err := h.service.BatchDeleteCardSecrets(req.IDs, req.BatchID, buildCardSecretListInput(req.Filter))
	if err != nil {
		switch {
		case errors.Is(err, cardsecretapp.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.card_secret_not_found", nil)
		case errors.Is(err, cardsecretapp.ErrDeleteFailed):
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_delete_failed", err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_delete_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"affected": rows,
	})
}

// ExportCardSecrets 批量导出卡密
func (h *AdminHandler) ExportCardSecrets(c *gin.Context) {
	var req ExportCardSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	content, contentType, err := h.service.ExportCardSecrets(req.IDs, req.BatchID, buildCardSecretListInput(req.Filter), req.Format)
	if err != nil {
		switch {
		case errors.Is(err, cardsecretapp.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.card_secret_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_fetch_failed", err)
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
func (h *AdminHandler) ExportAvailableCardSecrets(c *gin.Context) {
	var req ExportAvailableCardSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	result, err := h.service.ExportAvailableCardSecrets(cardsecretapp.ExportAvailableCardSecretInput{
		ProductID:         req.ProductID,
		SKUID:             req.SKUID,
		BatchID:           req.BatchID,
		Limit:             req.Limit,
		Format:            req.Format,
		DeleteAfterExport: req.DeleteAfterExport,
	})
	if err != nil {
		switch {
		case errors.Is(err, cardsecretapp.ErrInvalid),
			errors.Is(err, cardsecretapp.ErrProductSKUInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrProductNotFound),
			errors.Is(err, cardsecretapp.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.card_secret_not_found", nil)
		case errors.Is(err, cardsecretapp.ErrInsufficient):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_insufficient", nil)
		case errors.Is(err, cardsecretapp.ErrUpdateFailed):
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_update_failed", err)
		case errors.Is(err, cardsecretapp.ErrDeleteFailed):
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_delete_failed", err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_fetch_failed", err)
		}
		return
	}

	normalizedFormat := strings.ToLower(strings.TrimSpace(req.Format))
	filename := "card-secrets-available-" + time.Now().Format("20060102-150405") + "." + normalizedFormat
	c.Header("Content-Type", result.ContentType)
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Header("X-Exported-Count", strconv.Itoa(result.Count))
	c.Data(200, result.ContentType, result.Content)
}

// GetCardSecretStats 获取库存统计
func (h *AdminHandler) GetCardSecretStats(c *gin.Context) {
	productID, err := ginutil.ParseQueryUint(c.Query("product_id"), true)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	skuID, err := ginutil.ParseQueryUint(c.DefaultQuery("sku_id", "0"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	stats, err := h.service.GetStats(productID, skuID)
	if err != nil {
		switch {
		case errors.Is(err, cardsecretapp.ErrProductSKURequired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrProductSKUInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_stats_failed", err)
		}
		return
	}
	response.Success(c, stats)
}

// GetCardSecretBatches 获取卡密批次列表
func (h *AdminHandler) GetCardSecretBatches(c *gin.Context) {
	productID, err := ginutil.ParseQueryUint(c.Query("product_id"), true)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	skuID, err := ginutil.ParseQueryUint(c.DefaultQuery("sku_id", "0"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		return
	}

	items, total, err := h.service.ListBatches(productID, skuID, page, pageSize)
	if err != nil {
		switch {
		case errors.Is(err, cardsecretapp.ErrProductSKURequired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrProductSKUInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		case errors.Is(err, cardsecretapp.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.card_secret_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.card_secret_batch_fetch_failed", err)
		}
		return
	}
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, items, pagination)
}

// GetCardSecretTemplate 下载导入模板
func (h *AdminHandler) GetCardSecretTemplate(c *gin.Context) {
	content := "secret\nCARD-AAA-0001\nCARD-BBB-0002\n"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\"card-secrets-template.csv\"")
	c.String(200, content)
}
