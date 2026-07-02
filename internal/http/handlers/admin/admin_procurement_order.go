package admin

import (
	"errors"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

// GetProcurementOrders 采购单列表
func (h *Handler) GetProcurementOrders(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	page, pageSize := shared.ParsePagination(c)

	filter := repository.ProcurementOrderListFilter{
		Pagination: repository.Pagination{Page: page, PageSize: pageSize},
	}
	if connID := strings.TrimSpace(c.Query("connection_id")); connID != "" {
		if id, err := shared.ParseQueryUint(connID, false); err == nil {
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
	createdFrom, createdTo, err := shared.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	filter.CreatedFrom = createdFrom
	filter.CreatedTo = createdTo

	orders, total, err := h.ProcurementOrderService.List(filter)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.procurement_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, orders, pagination)
}

// GetProcurementOrderStats 采购单按状态聚合（基于全量数据，仅复用筛选条件）
func (h *Handler) GetProcurementOrderStats(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}

	filter := repository.ProcurementOrderListFilter{}
	if connID := strings.TrimSpace(c.Query("connection_id")); connID != "" {
		if id, err := shared.ParseQueryUint(connID, false); err == nil {
			filter.ConnectionID = id
		}
	}
	if orderNo := strings.TrimSpace(c.Query("order_no")); orderNo != "" {
		filter.LocalOrderNo = orderNo
	}
	if upstreamOrderNo := strings.TrimSpace(c.Query("upstream_order_no")); upstreamOrderNo != "" {
		filter.UpstreamOrderNo = upstreamOrderNo
	}
	createdFrom, createdTo, err := shared.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	filter.CreatedFrom = createdFrom
	filter.CreatedTo = createdTo

	stats, err := h.ProcurementOrderService.StatsByStatus(filter)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.procurement_fetch_failed", err)
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
func (h *Handler) GetProcurementOrder(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	order, err := h.ProcurementOrderService.GetByID(id)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.procurement_fetch_failed", err)
		return
	}
	if order == nil {
		shared.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
		return
	}
	order.TruncateUpstreamPayload(models.FulfillmentPayloadMaxPreviewLines)
	h.ProcurementOrderService.FillParentOrderNo(order)
	response.Success(c, order)
}

// DownloadProcurementUpstreamPayload 下载采购单上游交付内容
func (h *Handler) DownloadProcurementUpstreamPayload(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	order, err := h.ProcurementOrderService.GetByID(id)
	if err != nil || order == nil {
		shared.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
		return
	}
	if order.UpstreamPayload == "" {
		shared.RespondError(c, response.CodeNotFound, "error.fulfillment_not_found", nil)
		return
	}
	filename := "upstream-payload-" + strconv.FormatUint(uint64(order.ID), 10) + ".txt"
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(200, "text/plain; charset=utf-8", []byte(order.UpstreamPayload))
}

// RetryProcurementOrder 手动重试采购单
func (h *Handler) RetryProcurementOrder(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.ProcurementOrderService.RetryManual(id); err != nil {
		if errors.Is(err, service.ErrProcurementNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrProcurementStatusInvalid) {
			shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.procurement_retry_failed", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// CancelProcurementOrder 手动取消采购单
func (h *Handler) CancelProcurementOrder(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.ProcurementOrderService.CancelManual(id); err != nil {
		if errors.Is(err, service.ErrProcurementNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.procurement_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrProcurementStatusInvalid) {
			shared.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.procurement_cancel_failed", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// DirectSubmit 直接调用 SubmitToUpstream（绕过队列，同步执行）
func (h *Handler) DirectSubmit(c *gin.Context) {
	if h.ProcurementOrderService == nil {
		shared.RespondErrorWithMsg(c, response.CodeInternal, "service not available", nil)
		return
	}
	id, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.ProcurementOrderService.SubmitToUpstream(uint(id)); err != nil {
		shared.RespondError(c, response.CodeInternal, "error.procurement_submit_failed", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
