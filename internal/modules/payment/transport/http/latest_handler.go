package paymenthttp

import (
	"errors"
	"time"

	paymentpresenter "github.com/dujiao-next/internal/modules/payment/transport/presenter"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrGuestOrderNotFound = errors.New("guest order not found")
)

// GuestOrderLookup 游客订单查询端口。
type GuestOrderLookup interface {
	GetOrderByGuestOrderNoForTenant(tenant reseller.TenantContext, orderNo, email, password string) (*orderdomain.Order, error)
	GetOrderByGuestForTenant(tenant reseller.TenantContext, orderID uint, email, password string) (*orderdomain.Order, error)
}

// UserOrderLookup 用户订单查询端口。
type UserOrderLookup interface {
	GetOrderByUserOrderNoForTenant(tenant reseller.TenantContext, orderNo string, userID uint) (*orderdomain.Order, error)
	GetOrderByUserForTenant(tenant reseller.TenantContext, orderID, userID uint) (*orderdomain.Order, error)
}

// PendingPaymentLookup 待支付记录查询端口。
type PendingPaymentLookup interface {
	GetLatestPendingByOrder(orderID uint, now time.Time) (*paymentdomain.Payment, error)
}

// LatestGuestPaymentQuery 游客最新待支付查询参数。
type LatestGuestPaymentQuery struct {
	OrderNo string `form:"order_no" binding:"required"`
}

// LatestPaymentQuery 用户最新待支付查询参数。
type LatestPaymentQuery struct {
	OrderNo string `form:"order_no" binding:"required"`
}

// LatestHandler 处理前台最新待支付查询 HTTP。
type LatestHandler struct {
	guestOrders GuestOrderLookup
	userOrders  UserOrderLookup
	payments    PendingPaymentLookup
}

func NewLatestHandler(guestOrders GuestOrderLookup, userOrders UserOrderLookup, payments PendingPaymentLookup) *LatestHandler {
	if guestOrders == nil || userOrders == nil || payments == nil {
		panic("payment latest handler: required dependency is nil")
	}
	return &LatestHandler{guestOrders: guestOrders, userOrders: userOrders, payments: payments}
}

func tenantFromRequest(c *gin.Context) reseller.TenantContext {
	return reseller.TenantOrMain(c.Request.Context())
}

// GetGuestLatestPayment 获取游客最新待支付记录
func (h *LatestHandler) GetGuestLatestPayment(c *gin.Context) {
	var query LatestGuestPaymentQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	email, password, ok := ginutil.GetGuestCredentials(c)
	if !ok || email == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.guest_email_required", nil)
		return
	}
	if password == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.guest_password_required", nil)
		return
	}

	order, err := h.guestOrders.GetOrderByGuestOrderNoForTenant(tenantFromRequest(c), query.OrderNo, email, password)
	if err != nil {
		if errors.Is(err, ErrGuestOrderNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.guest_order_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	h.respondLatestPayment(c, order)
}

// GetLatestPayment 获取用户最新待支付记录
func (h *LatestHandler) GetLatestPayment(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var query LatestPaymentQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	order, err := h.userOrders.GetOrderByUserOrderNoForTenant(tenantFromRequest(c), query.OrderNo, uid)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	h.respondLatestPayment(c, order)
}

func (h *LatestHandler) respondLatestPayment(c *gin.Context, order *orderdomain.Order) {
	if order.ParentID != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		return
	}
	if order.Status != constants.OrderStatusPendingPayment {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		return
	}
	if order.ExpiresAt != nil && !order.ExpiresAt.After(time.Now()) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		return
	}

	payment, err := h.payments.GetLatestPendingByOrder(order.ID, time.Now())
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	if payment == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.payment_not_found", nil)
		return
	}

	response.Success(c, paymentpresenter.NewLatestPaymentResp(payment, order.OrderNo))
}
