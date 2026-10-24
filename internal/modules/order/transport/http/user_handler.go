package orderhttp

import (
	"errors"
	"strings"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	orderpresenter "github.com/dujiao-next/internal/modules/order/transport/presenter"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// UserOrderListFilter 用户订单列表过滤。
type UserOrderListFilter struct {
	Page     int
	PageSize int
	UserID   uint
	Status   string
	OrderNo  string
}

// UserOrderQuery 用户订单查询与取消端口。
type UserOrderQuery interface {
	ListOrdersByUserForTenant(tenant reseller.TenantContext, filter UserOrderListFilter) ([]orderdomain.Order, int64, error)
	StatsOrdersByUserForTenant(tenant reseller.TenantContext, filter UserOrderListFilter) (map[string]int64, error)
	GetOrderByUserOrderNoForTenant(tenant reseller.TenantContext, orderNo string, userID uint) (*orderdomain.Order, error)
	GetAnyOrderByUserOrderNoForTenant(tenant reseller.TenantContext, orderNo string, userID uint) (*orderdomain.Order, error)
	CancelOrder(orderID uint, userID uint) (*orderdomain.Order, error)
}

// AvailablePaymentChannelFilter 可用支付渠道过滤。
type AvailablePaymentChannelFilter struct {
	TargetAmount *money.Amount
	User         *userdomain.User
	PaymentType  string
}

// PaymentChannelPolicy 订单可用支付渠道端口。
type PaymentChannelPolicy interface {
	GetAllowedChannelIDsForOrder(items []orderdomain.OrderItem) []uint
	GetAvailableChannels(filter AvailablePaymentChannelFilter) ([]map[string]interface{}, error)
}

// RefundRecordDirectory 退款记录查询端口。
type RefundRecordDirectory interface {
	ListByOrderIDs(orderIDs []uint) ([]orderdomain.OrderRefundRecord, error)
}

// UserLookup 用户查询端口。
type UserLookup interface {
	GetByID(id uint) (*userdomain.User, error)
}

// OrderPaymentChannelsRequest 查询订单可用支付渠道请求
type OrderPaymentChannelsRequest struct {
	Amount  string             `json:"amount" binding:"required"`
	OrderNo string             `json:"order_no"`
	Items   []OrderItemRequest `json:"items"`
}

// UserHandler 处理前台用户订单 HTTP（只读 + 取消 + 支付渠道查询）。
type UserHandler struct {
	orders   UserOrderQuery
	payments PaymentChannelPolicy
	refunds  RefundRecordDirectory
	users    UserLookup
}

func NewUserHandler(orders UserOrderQuery, payments PaymentChannelPolicy, refunds RefundRecordDirectory, users UserLookup) *UserHandler {
	if orders == nil {
		panic("order user handler: orders is nil")
	}
	return &UserHandler{orders: orders, payments: payments, refunds: refunds, users: users}
}

func tenantFromRequest(c *gin.Context) reseller.TenantContext {
	return reseller.TenantOrMain(c.Request.Context())
}

// ListOrders 获取订单列表
func (h *UserHandler) ListOrders(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	page, pageSize := ginutil.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))
	orderNo := strings.TrimSpace(c.Query("order_no"))

	orders, total, err := h.orders.ListOrdersByUserForTenant(tenantFromRequest(c), UserOrderListFilter{
		Page:     page,
		PageSize: pageSize,
		UserID:   uid,
		Status:   status,
		OrderNo:  orderNo,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, orderpresenter.NewOrderSummaryList(orders), pagination)
}

// OrderStats 按状态聚合当前用户订单数量（基于全量数据，仅复用关键词筛选）
func (h *UserHandler) OrderStats(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	orderNo := strings.TrimSpace(c.Query("order_no"))
	stats, err := h.orders.StatsOrdersByUserForTenant(tenantFromRequest(c), UserOrderListFilter{
		UserID:  uid,
		OrderNo: orderNo,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
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

// GetOrderByOrderNo 按订单号获取订单详情
func (h *UserHandler) GetOrderByOrderNo(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}

	order, err := h.orders.GetOrderByUserOrderNoForTenant(tenantFromRequest(c), orderNo, uid)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}

	orderDetail := orderpresenter.NewOrderDetailTruncated(order)
	enrichOrderWithAllowedChannels(h.payments, order, &orderDetail)
	enrichOrderWithRefundRecords(h.refunds, order, &orderDetail)
	response.Success(c, orderDetail)
}

// DownloadFulfillment 下载订单交付内容（登录用户）
func (h *UserHandler) DownloadFulfillment(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	order, err := h.orders.GetAnyOrderByUserOrderNoForTenant(tenantFromRequest(c), orderNo, uid)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	if order == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		return
	}
	respondFulfillmentDownload(c, order)
}

// GetOrderPaymentChannels 获取当前用户订单可用支付渠道（按金额与商品范围过滤）
func (h *UserHandler) GetOrderPaymentChannels(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req OrderPaymentChannelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		response.Success(c, []map[string]interface{}{})
		return
	}

	var user *userdomain.User
	if h.users != nil {
		user, _ = h.users.GetByID(uid)
	}
	if h.payments == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", nil)
		return
	}
	channels, err := h.payments.GetAvailableChannels(AvailablePaymentChannelFilter{
		TargetAmount: &money.Amount{Decimal: amount},
		User:         user,
		PaymentType:  constants.PaymentTypeOrder,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}

	var allowedIDs []uint
	orderNo := strings.TrimSpace(req.OrderNo)
	switch {
	case orderNo != "":
		order, orderErr := h.orders.GetOrderByUserOrderNoForTenant(tenantFromRequest(c), orderNo, uid)
		if orderErr != nil {
			if errors.Is(orderErr, ErrOrderNotFound) {
				ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
				return
			}
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", orderErr)
			return
		}
		allItems := append([]orderdomain.OrderItem{}, order.Items...)
		for _, child := range order.Children {
			allItems = append(allItems, child.Items...)
		}
		allowedIDs = h.payments.GetAllowedChannelIDsForOrder(allItems)
	case len(req.Items) > 0:
		orderItems := make([]orderdomain.OrderItem, 0, len(req.Items))
		for _, item := range req.Items {
			if item.ProductID == 0 {
				continue
			}
			orderItems = append(orderItems, orderdomain.OrderItem{ProductID: item.ProductID})
		}
		allowedIDs = h.payments.GetAllowedChannelIDsForOrder(orderItems)
	}

	// nil 表示商品未限制渠道；空切片表示限制后无可用渠道。
	if allowedIDs != nil {
		allowedSet := make(map[uint]struct{}, len(allowedIDs))
		for _, id := range allowedIDs {
			allowedSet[id] = struct{}{}
		}
		filtered := make([]map[string]interface{}, 0, len(channels))
		for _, channel := range channels {
			channelID, ok := channel["id"].(uint)
			if !ok {
				continue
			}
			if _, matched := allowedSet[channelID]; matched {
				filtered = append(filtered, channel)
			}
		}
		channels = filtered
	}

	response.Success(c, channels)
}

// CancelOrder 用户取消订单
func (h *UserHandler) CancelOrder(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}

	tenant := tenantFromRequest(c)
	found, err := h.orders.GetOrderByUserOrderNoForTenant(tenant, orderNo, uid)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}

	order, err := h.orders.CancelOrder(found.ID, uid)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		case errors.Is(err, ErrOrderCancelNotAllowed):
			ginutil.RespondError(c, response.CodeBadRequest, "error.order_cancel_not_allowed", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", err)
		}
		return
	}

	response.Success(c, orderpresenter.NewOrderDetail(order))
}

func enrichOrderWithAllowedChannels(payments PaymentChannelPolicy, order *orderdomain.Order, detail *orderpresenter.OrderDetail) {
	if payments == nil || order == nil || detail == nil {
		return
	}
	allItems := order.Items
	for _, child := range order.Children {
		allItems = append(allItems, child.Items...)
	}
	allowed := payments.GetAllowedChannelIDsForOrder(allItems)
	if len(allowed) > 0 {
		detail.AllowedPaymentChannelIDs = allowed
	}
}

func collectRefundRelevantOrderIDs(order *orderdomain.Order) []uint {
	if order == nil || order.ID == 0 {
		return nil
	}
	seen := map[uint]struct{}{}
	ids := make([]uint, 0, 1+len(order.Children))
	appendID := func(id uint) {
		if id == 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	appendID(order.ID)
	for _, child := range order.Children {
		appendID(child.ID)
	}
	return ids
}

func enrichOrderWithRefundRecords(refunds RefundRecordDirectory, order *orderdomain.Order, detail *orderpresenter.OrderDetail) {
	if order == nil || detail == nil || refunds == nil {
		return
	}
	status := strings.ToLower(strings.TrimSpace(detail.Status))
	if status != constants.OrderStatusRefunded && status != constants.OrderStatusPartiallyRefunded {
		return
	}

	orderIDs := collectRefundRelevantOrderIDs(order)
	if len(orderIDs) == 0 {
		return
	}
	records, err := refunds.ListByOrderIDs(orderIDs)
	if err != nil {
		logger.Warnw("public_order_refund_records_fetch_failed",
			"order_id", order.ID,
			"order_no", order.OrderNo,
			"error", err,
		)
		return
	}
	if len(records) == 0 {
		return
	}

	detail.RefundRecords = make([]orderpresenter.OrderRefundResp, 0, len(records))
	for _, record := range records {
		detail.RefundRecords = append(detail.RefundRecords, orderpresenter.OrderRefundResp{
			Type:      strings.TrimSpace(record.Type),
			Amount:    record.Amount,
			Currency:  strings.TrimSpace(record.Currency),
			Remark:    strings.TrimSpace(record.Remark),
			CreatedAt: record.CreatedAt,
		})
	}
}

func respondFulfillmentDownload(c *gin.Context, order *orderdomain.Order) {
	payload := collectFulfillmentPayload(order)
	if payload == "" {
		ginutil.RespondError(c, response.CodeNotFound, "error.fulfillment_not_found", nil)
		return
	}
	filename := "fulfillment-" + order.OrderNo + ".txt"
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(200, "text/plain; charset=utf-8", []byte(payload))
}

func collectFulfillmentPayload(order *orderdomain.Order) string {
	if order.Fulfillment != nil && order.Fulfillment.Payload != "" {
		return order.Fulfillment.Payload
	}
	var parts []string
	for _, child := range order.Children {
		if child.Fulfillment != nil && child.Fulfillment.Payload != "" {
			parts = append(parts, child.Fulfillment.Payload)
		}
	}
	return strings.Join(parts, "\n")
}
