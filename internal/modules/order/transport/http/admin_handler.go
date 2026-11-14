package orderhttp

import (
	"errors"
	"fmt"
	"strings"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"

	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

var (
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderStatusInvalid    = errors.New("order status invalid")
	ErrGuestOrderNotFound    = errors.New("guest order not found")
	ErrOrderCancelNotAllowed = errors.New("order cancel not allowed")
)

// OrderListFilter 管理端订单列表过滤。
type OrderListFilter struct {
	Page           int
	PageSize       int
	UserID         uint
	UserKeyword    string
	Status         string
	OrderNo        string
	GuestEmail     string
	ProductKeyword string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	SortBy         string
	SortOrder      string
}

// OrderQuery 管理端订单查询与状态更新端口。
type OrderQuery interface {
	ListOrdersForAdmin(filter OrderListFilter) ([]orderdomain.Order, int64, error)
	GetOrderForAdmin(orderID uint) (*orderdomain.Order, error)
	UpdateOrderStatus(orderID uint, status string) (*orderdomain.Order, error)
}

// UserDirectory 用户目录端口。
type UserDirectory interface {
	ListByIDs(ids []uint) ([]userdomain.User, error)
	GetByID(id uint) (*userdomain.User, error)
}

// CouponLookup 优惠券查询端口。
type CouponLookup interface {
	GetByID(id uint) (*coupondomain.Coupon, error)
}

// PromotionLookup 活动价查询端口。
type PromotionLookup interface {
	GetByID(id uint) (*promotiondomain.Promotion, error)
}

// PaymentDirectory 支付记录查询端口。
type PaymentDirectory interface {
	ListByOrderID(orderID uint) ([]paymentdomain.Payment, error)
}

// PaymentChannelDirectory 支付渠道查询端口。
type PaymentChannelDirectory interface {
	ListByIDs(ids []uint) ([]paymentdomain.PaymentChannel, error)
}

// AccountingPolicy 订单利润统计口径端口。
type AccountingPolicy interface {
	// RefundReversesCost 退款时是否按比例冲回成本，与仪表盘利润口径保持一致。
	RefundReversesCost() bool
}

// AdminHandler 处理后台订单 HTTP。
type AdminHandler struct {
	orders     OrderQuery
	users      UserDirectory
	coupons    CouponLookup
	promotions PromotionLookup
	payments   PaymentDirectory
	channels   PaymentChannelDirectory
	accounting AccountingPolicy
}

func NewAdminHandler(
	orders OrderQuery,
	users UserDirectory,
	coupons CouponLookup,
	promotions PromotionLookup,
	payments PaymentDirectory,
	channels PaymentChannelDirectory,
	accounting AccountingPolicy,
) *AdminHandler {
	if orders == nil || users == nil || coupons == nil || promotions == nil || payments == nil || channels == nil || accounting == nil {
		panic("order admin handler: required dependency is nil")
	}
	return &AdminHandler{
		orders:     orders,
		users:      users,
		coupons:    coupons,
		promotions: promotions,
		payments:   payments,
		channels:   channels,
		accounting: accounting,
	}
}

// AdminOrderListItem 管理端订单列表返回
type AdminOrderListItem struct {
	orderdomain.Order
	UserEmail       string `json:"user_email,omitempty"`
	UserDisplayName string `json:"user_display_name,omitempty"`
}

// AdminOrderPaymentItem 管理端订单详情中的支付项
type AdminOrderPaymentItem struct {
	paymentdomain.Payment
	ChannelName        string `json:"channel_name"`
	DisplayChannelType string `json:"display_channel_type,omitempty"`
}

// AdminOrderDetail 管理端订单详情返回
type AdminOrderDetail struct {
	orderdomain.Order
	UserEmail       string                  `json:"user_email,omitempty"`
	UserDisplayName string                  `json:"user_display_name,omitempty"`
	CouponCode      string                  `json:"coupon_code,omitempty"`
	PromotionName   string                  `json:"promotion_name,omitempty"`
	Payments        []AdminOrderPaymentItem `json:"payments,omitempty"`
	// RefundReversesCost 为 true 时，订单详情利润在退款时按比例冲回成本。
	RefundReversesCost bool `json:"refund_reverses_cost"`
}

// AdminListOrders 管理端订单列表
func (h *AdminHandler) AdminListOrders(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	status := strings.TrimSpace(c.Query("status"))
	userIDRaw := c.Query("user_id")
	userKeyword := strings.TrimSpace(c.Query("user_keyword"))
	orderNo := strings.TrimSpace(c.Query("order_no"))
	guestEmail := strings.TrimSpace(c.Query("guest_email"))

	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var userID uint
	userID, _ = ginutil.ParseQueryUint(userIDRaw, false)

	productKeyword := strings.TrimSpace(c.Query("product_keyword"))
	sortBy := strings.TrimSpace(c.Query("sort_by"))
	sortOrder := strings.TrimSpace(c.Query("sort_order"))

	orders, total, err := h.orders.ListOrdersForAdmin(OrderListFilter{
		Page:           page,
		PageSize:       pageSize,
		UserID:         userID,
		UserKeyword:    userKeyword,
		Status:         status,
		OrderNo:        orderNo,
		GuestEmail:     guestEmail,
		ProductKeyword: productKeyword,
		CreatedFrom:    createdFrom,
		CreatedTo:      createdTo,
		SortBy:         sortBy,
		SortOrder:      sortOrder,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	userMap := map[uint]userdomain.User{}
	userIDs := make([]uint, 0, len(orders))
	seen := map[uint]struct{}{}
	for _, order := range orders {
		if order.UserID == 0 {
			continue
		}
		if _, ok := seen[order.UserID]; ok {
			continue
		}
		seen[order.UserID] = struct{}{}
		userIDs = append(userIDs, order.UserID)
	}
	if len(userIDs) > 0 {
		users, err := h.users.ListByIDs(userIDs)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
			return
		}
		for _, user := range users {
			userMap[user.ID] = user
		}
	}

	items := make([]AdminOrderListItem, 0, len(orders))
	for _, order := range orders {
		var email, displayName string
		if user, ok := userMap[order.UserID]; ok {
			email = user.Email
			displayName = user.DisplayName
		}
		items = append(items, AdminOrderListItem{
			Order:           order,
			UserEmail:       email,
			UserDisplayName: displayName,
		})
	}

	response.SuccessWithPage(c, items, pagination)
}

// AdminGetOrder 管理端订单详情
func (h *AdminHandler) AdminGetOrder(c *gin.Context) {
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}

	order, err := h.orders.GetOrderForAdmin(orderID)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		}
		return
	}
	var email, displayName string
	if order.UserID != 0 {
		user, err := h.users.GetByID(order.UserID)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
			return
		}
		if user != nil {
			email = user.Email
			displayName = user.DisplayName
		}
	}

	var couponCode string
	if order.CouponID != nil && *order.CouponID > 0 {
		coupon, err := h.coupons.GetByID(*order.CouponID)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
			return
		}
		if coupon != nil {
			couponCode = coupon.Code
		}
	}

	var promotionName string
	if order.PromotionID != nil && *order.PromotionID > 0 {
		promotion, err := h.promotions.GetByID(*order.PromotionID)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
			return
		}
		if promotion != nil {
			promotionName = promotion.Name
		}
	}

	promotionNameMap := make(map[uint]string)
	for i := range order.Items {
		item := order.Items[i]
		if item.PromotionID == nil || *item.PromotionID == 0 {
			continue
		}
		promotionID := *item.PromotionID
		if _, ok := promotionNameMap[promotionID]; ok {
			continue
		}
		promotion, err := h.promotions.GetByID(promotionID)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
			return
		}
		if promotion != nil {
			promotionNameMap[promotionID] = promotion.Name
		} else {
			promotionNameMap[promotionID] = ""
		}
	}
	for i := range order.Children {
		for _, item := range order.Children[i].Items {
			if item.PromotionID == nil || *item.PromotionID == 0 {
				continue
			}
			promotionID := *item.PromotionID
			if _, ok := promotionNameMap[promotionID]; ok {
				continue
			}
			promotion, err := h.promotions.GetByID(promotionID)
			if err != nil {
				ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
				return
			}
			if promotion != nil {
				promotionNameMap[promotionID] = promotion.Name
			} else {
				promotionNameMap[promotionID] = ""
			}
		}
	}
	for i := range order.Items {
		item := &order.Items[i]
		if item.PromotionID == nil || *item.PromotionID == 0 {
			continue
		}
		item.PromotionName = promotionNameMap[*item.PromotionID]
	}
	for i := range order.Children {
		for j := range order.Children[i].Items {
			item := &order.Children[i].Items[j]
			if item.PromotionID == nil || *item.PromotionID == 0 {
				continue
			}
			item.PromotionName = promotionNameMap[*item.PromotionID]
		}
	}

	payments, err := h.payments.ListByOrderID(order.ID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	channelNameMap, err := h.resolvePaymentChannelNames(payments)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	paymentItems := make([]AdminOrderPaymentItem, 0, len(payments))
	for _, payment := range payments {
		paymentItems = append(paymentItems, AdminOrderPaymentItem{
			Payment:            payment,
			ChannelName:        channelNameMap[payment.ChannelID],
			DisplayChannelType: paymentDisplayChannelType(payment),
		})
	}

	order.TruncateFulfillmentPayload()
	response.Success(c, AdminOrderDetail{
		Order:           *order,
		UserEmail:       email,
		UserDisplayName: displayName,
		CouponCode:      couponCode,
		PromotionName:   promotionName,
		Payments:        paymentItems,

		RefundReversesCost: h.accounting.RefundReversesCost(),
	})
}

func (h *AdminHandler) resolvePaymentChannelNames(payments []paymentdomain.Payment) (map[uint]string, error) {
	channelIDs := make([]uint, 0, len(payments))
	seen := make(map[uint]struct{})
	for _, payment := range payments {
		if payment.ChannelID == 0 {
			continue
		}
		if _, ok := seen[payment.ChannelID]; ok {
			continue
		}
		seen[payment.ChannelID] = struct{}{}
		channelIDs = append(channelIDs, payment.ChannelID)
	}
	result := make(map[uint]string)
	if len(channelIDs) == 0 {
		return result, nil
	}
	channels, err := h.channels.ListByIDs(channelIDs)
	if err != nil {
		return nil, err
	}
	for _, channel := range channels {
		result[channel.ID] = channel.Name
	}
	return result, nil
}

func paymentDisplayChannelType(payment paymentdomain.Payment) string {
	if displayChannelType := strings.TrimSpace(payment.DisplayChannelType); displayChannelType != "" {
		return displayChannelType
	}
	value, ok := payment.ProviderPayload["display_channel_type"]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

// AdminUpdateOrderStatusRequest 管理端更新订单状态请求
type AdminUpdateOrderStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// AdminUpdateOrderStatus 管理端更新订单状态
func (h *AdminHandler) AdminUpdateOrderStatus(c *gin.Context) {
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}

	var req AdminUpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	order, err := h.orders.UpdateOrderStatus(orderID, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		case errors.Is(err, ErrOrderStatusInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", err)
		}
		return
	}

	response.Success(c, order)
}
