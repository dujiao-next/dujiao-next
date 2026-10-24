package procurementhttp

import (
	"errors"
	"strconv"
	"strings"

	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type Service interface {
	List(procurementcontract.ListFilter) ([]procurementdomain.Order, int64, error)
	StatsByStatus(procurementcontract.ListFilter) (map[string]int64, error)
	GetByID(id uint) (*procurementdomain.Order, error)
	FillParentOrderNo(order *procurementdomain.Order)
	RetryManual(id uint) error
	CancelManual(id uint) error
}

type AdminHandler struct {
	service Service
}

func NewAdminHandler(service Service) *AdminHandler {
	if service == nil {
		panic("procurement admin handler: required dependency is nil")
	}
	return &AdminHandler{service: service}
}

// GetProcurementOrders 采购单列表
func (h *AdminHandler) GetProcurementOrders(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	filter := procurementcontract.ListFilter{Page: page, PageSize: pageSize}
	if connID := strings.TrimSpace(c.Query("connection_id")); connID != "" {
		if id, err := ginutil.ParseQueryUint(connID, false); err == nil {
			filter.ConnectionID = id
		}
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		filter.Status = status
	}
	if orderNo := strings.TrimSpace(c.Query("order_no")); orderNo != "" {
		filter.LocalOrderNo = orderNo
	}
	if upstreamOrderNo := strings.TrimSpace(c.Query("upstream_order_no")); upstreamOrderNo != "" {
		filter.UpstreamOrderNo = upstreamOrderNo
	}
	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	filter.CreatedFrom = createdFrom
	filter.CreatedTo = createdTo

	orders, total, err := h.service.List(filter)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.procurement_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, orders, pagination)
}

// GetProcurementOrderStats 采购单按状态聚合（基于全量数据，仅复用筛选条件）
func (h *AdminHandler) GetProcurementOrderStats(c *gin.Context) {
	if h.service == nil {
		ginutil.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}

	filter := procurementcontract.ListFilter{}
	if connID := strings.TrimSpace(c.Query("connection_id")); connID != "" {
		if id, err := ginutil.ParseQueryUint(connID, false); err == nil {
			filter.ConnectionID = id
		}
	}
	if orderNo := strings.TrimSpace(c.Query("order_no")); orderNo != "" {
		filter.LocalOrderNo = orderNo
	}
	if upstreamOrderNo := strings.TrimSpace(c.Query("upstream_order_no")); upstreamOrderNo != "" {
		filter.UpstreamOrderNo = upstreamOrderNo
	}
	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	filter.CreatedFrom = createdFrom
	filter.CreatedTo = createdTo

	stats, err := h.service.StatsByStatus(filter)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.procurement_fetch_failed", err)
		return
	}

	var total int64
	for _, v := range stats {
		total += v
	}
	response.Success(c, gin.H{
		"total":     total,
		"by_status": stats,
	})
}

// GetProcurementOrder 采购单详情
func (h *AdminHandler) GetProcurementOrder(c *gin.Context) {
	if h.service == nil {
		ginutil.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	order, err := h.service.GetByID(id)
	if err != nil {
		if errors.Is(err, procurementcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.procurement_fetch_failed", err)
		return
	}
	if order == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
		return
	}
	order.TruncateUpstreamPayload(procurementdomain.PayloadPreviewMaxLines)
	h.service.FillParentOrderNo(order)
	response.Success(c, order)
}

// DownloadProcurementUpstreamPayload 下载采购单上游交付内容
func (h *AdminHandler) DownloadProcurementUpstreamPayload(c *gin.Context) {
	if h.service == nil {
		ginutil.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	order, err := h.service.GetByID(id)
	if err != nil || order == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
		return
	}
	if order.UpstreamPayload == "" {
		ginutil.RespondError(c, response.CodeNotFound, "error.fulfillment_not_found", nil)
		return
	}
	filename := "upstream-payload-" + strconv.FormatUint(uint64(order.ID), 10) + ".txt"
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(200, "text/plain; charset=utf-8", []byte(order.UpstreamPayload))
}

// RetryProcurementOrder 手动重试采购单
func (h *AdminHandler) RetryProcurementOrder(c *gin.Context) {
	if h.service == nil {
		ginutil.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.service.RetryManual(id); err != nil {
		if errors.Is(err, procurementcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
			return
		}
		if errors.Is(err, procurementcontract.ErrStatusInvalid) {
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.procurement_retry_failed", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// CancelProcurementOrder 手动取消采购单
func (h *AdminHandler) CancelProcurementOrder(c *gin.Context) {
	if h.service == nil {
		ginutil.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.service.CancelManual(id); err != nil {
		if errors.Is(err, procurementcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
			return
		}
		if errors.Is(err, procurementcontract.ErrStatusInvalid) {
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		if errors.Is(err, procurementcontract.ErrUpstreamCancelRefused) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.procurement_upstream_cancel_refused", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.procurement_cancel_failed", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
