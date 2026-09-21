package orderhttp

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	orderpresenter "github.com/dujiao-next/internal/modules/order/transport/presenter"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// GuestOrderQuery 游客订单只读端口。
type GuestOrderQuery interface {
	ListOrdersByGuestForTenant(tenant reseller.TenantContext, email, password string, page, pageSize int) ([]orderdomain.Order, int64, error)
	ListOrdersByBrowserTokenForTenant(tenant reseller.TenantContext, tokenHash string, page, pageSize int) ([]orderdomain.Order, int64, error)
	GetOrderByGuestOrderNoForTenant(tenant reseller.TenantContext, orderNo, email, password string) (*orderdomain.Order, error)
	GetAnyOrderByGuestOrderNoForTenant(tenant reseller.TenantContext, orderNo, email, password string) (*orderdomain.Order, error)
	CancelGuestOrder(order *orderdomain.Order) (*orderdomain.Order, error)
}

const browserOrderCookieName = "__Host-dujiao_browser_orders"

func browserTokenHash(raw string) string {
	raw = strings.TrimSpace(raw)
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != raw {
		return ""
	}
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

// GuestHandler 处理前台游客订单只读 HTTP。
type GuestHandler struct {
	orders   GuestOrderQuery
	payments PaymentChannelPolicy
	refunds  RefundRecordDirectory
}

func NewGuestHandler(orders GuestOrderQuery, payments PaymentChannelPolicy, refunds RefundRecordDirectory) *GuestHandler {
	if orders == nil {
		panic("order guest handler: orders is nil")
	}
	return &GuestHandler{orders: orders, payments: payments, refunds: refunds}
}

// ListGuestOrders 获取游客订单列表
func (h *GuestHandler) ListGuestOrders(c *gin.Context) {
	email, password, ok := ginutil.GetGuestCredentials(c)
	orderNo := strings.TrimSpace(c.Query("order_no"))
	if !ok || email == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}

	if orderNo != "" {
		order, err := h.orders.GetOrderByGuestOrderNoForTenant(tenantFromRequest(c), orderNo, email, password)
		if err != nil {
			if errors.Is(err, ErrGuestOrderNotFound) {
				pagination := response.Pagination{
					Page:      1,
					PageSize:  1,
					Total:     0,
					TotalPage: 1,
				}
				response.SuccessWithPage(c, []orderdomain.Order{}, pagination)
				return
			}
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
			return
		}
		pagination := response.Pagination{
			Page:      1,
			PageSize:  1,
			Total:     1,
			TotalPage: 1,
		}
		response.SuccessWithPage(c, orderpresenter.NewOrderSummaryList([]orderdomain.Order{*order}), pagination)
		return
	}

	page, pageSize := ginutil.ParsePagination(c)

	orders, total, err := h.orders.ListOrdersByGuestForTenant(tenantFromRequest(c), email, password, page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, orderpresenter.NewOrderSummaryList(orders), pagination)
}

// ListBrowserOrders returns summaries only for guest orders created in this browser.
func (h *GuestHandler) ListBrowserOrders(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	raw, err := c.Cookie(browserOrderCookieName)
	if err != nil {
		response.SuccessWithPage(c, []orderdomain.Order{}, response.BuildPagination(page, pageSize, 0))
		return
	}
	hash := browserTokenHash(raw)
	if hash == "" {
		response.SuccessWithPage(c, []orderdomain.Order{}, response.BuildPagination(page, pageSize, 0))
		return
	}
	orders, total, queryErr := h.orders.ListOrdersByBrowserTokenForTenant(tenantFromRequest(c), hash, page, pageSize)
	if queryErr != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", queryErr)
		return
	}
	response.SuccessWithPage(c, orderpresenter.NewOrderSummaryList(orders), response.BuildPagination(page, pageSize, total))
}

// GetGuestOrderByOrderNo 按订单号获取游客订单详情
func (h *GuestHandler) GetGuestOrderByOrderNo(c *gin.Context) {
	email, password, ok := ginutil.GetGuestCredentials(c)
	if !ok || email == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	order, err := h.orders.GetOrderByGuestOrderNoForTenant(tenantFromRequest(c), orderNo, email, password)
	if err != nil {
		if errors.Is(err, ErrGuestOrderNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
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

// DownloadGuestFulfillment 下载订单交付内容（游客）
// 支持父订单或子订单的 order_no
func (h *GuestHandler) DownloadGuestFulfillment(c *gin.Context) {
	email, password, ok := ginutil.GetGuestCredentials(c)
	if !ok || email == "" || password == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	order, err := h.orders.GetAnyOrderByGuestOrderNoForTenant(tenantFromRequest(c), orderNo, email, password)
	if err != nil {
		if errors.Is(err, ErrGuestOrderNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	if order == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
		return
	}
	respondFulfillmentDownload(c, order)
}

// CancelGuestOrder 取消凭邮箱和查询密码验证的待支付游客订单。
func (h *GuestHandler) CancelGuestOrder(c *gin.Context) {
	email, password, ok := ginutil.GetGuestCredentials(c)
	if !ok || strings.TrimSpace(email) == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if strings.TrimSpace(password) == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	found, err := h.orders.GetOrderByGuestOrderNoForTenant(tenantFromRequest(c), orderNo, email, password)
	if err != nil {
		if errors.Is(err, ErrGuestOrderNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	order, err := h.orders.CancelGuestOrder(found)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderCancelNotAllowed):
			ginutil.RespondError(c, response.CodeBadRequest, "error.order_cancel_not_allowed", nil)
		case errors.Is(err, ErrOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", err)
		}
		return
	}
	response.Success(c, orderpresenter.NewOrderDetailTruncated(order))
}
