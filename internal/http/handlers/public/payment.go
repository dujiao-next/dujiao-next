package public

import (
	"errors"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/dto"
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

// CreatePaymentRequest 创建支付请求
type CreatePaymentRequest struct {
	OrderNo    string `json:"order_no" binding:"required"`
	ChannelID  uint   `json:"channel_id"`
	UseBalance bool   `json:"use_balance"`
}

// LatestPaymentQuery 查询最新待支付记录
type LatestPaymentQuery struct {
	OrderNo string `form:"order_no" binding:"required"`
}

// SelectPaymentMethodRequest 选择二维码支付的币种和网络。
type SelectPaymentMethodRequest struct {
	Currency string `json:"currency" binding:"required"`
	Network  string `json:"network" binding:"required"`
}

// PaypalWebhookQuery PayPal webhook 查询参数。
type PaypalWebhookQuery struct {
	ChannelID uint `form:"channel_id" binding:"required"`
}

// WechatCallbackQuery 微信支付回调查询参数。
type WechatCallbackQuery struct {
	ChannelID uint `form:"channel_id"`
}

// StripeWebhookQuery Stripe webhook 查询参数。
type StripeWebhookQuery struct {
	ChannelID uint `form:"channel_id"`
}

// DujiaoPayWebhookQuery DujiaoPay webhook 查询参数。
type DujiaoPayWebhookQuery struct {
	ChannelID uint `form:"channel_id"`
}

const callbackLogValueLimit = 4096

// CreatePayment 创建支付单
func (h *Handler) CreatePayment(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}

	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	tenant := tenantFromRequest(c)
	order, err := h.OrderService.GetOrderByUserOrderNoForTenant(tenant, req.OrderNo, uid)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}

	result, err := h.PaymentService.CreatePayment(service.CreatePaymentInput{
		OrderID:    order.ID,
		ChannelID:  req.ChannelID,
		UseBalance: req.UseBalance,
		ClientIP:   c.ClientIP(),
		Context:    c.Request.Context(),
	})
	if err != nil {
		respondPaymentCreateError(c, err)
		return
	}

	response.Success(c, dto.NewCreatePaymentResp(result))
}

// SelectPaymentMethod 为当前用户的订单或充值单选择链上付款方式。
func (h *Handler) SelectPaymentMethod(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	paymentID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		return
	}
	var req SelectPaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	payment, err := h.PaymentService.GetPayment(paymentID)
	if err != nil {
		respondPaymentCaptureError(c, err)
		return
	}

	businessNo := ""
	if payment.OrderID != 0 {
		order, orderErr := h.OrderService.GetOrderByUserForTenant(tenantFromRequest(c), payment.OrderID, uid)
		if orderErr != nil {
			respondPaymentCaptureError(c, orderErr)
			return
		}
		businessNo = order.OrderNo
	} else {
		recharge, rechargeErr := h.WalletService.GetRechargeOrderByPaymentIDAndUser(paymentID, uid)
		if rechargeErr != nil {
			if errors.Is(rechargeErr, service.ErrWalletRechargeNotFound) {
				shared.RespondError(c, response.CodeNotFound, "error.payment_not_found", nil)
				return
			}
			shared.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", rechargeErr)
			return
		}
		businessNo = recharge.RechargeNo
	}

	updated, err := h.PaymentService.SelectPaymentMethod(service.SelectPaymentMethodInput{
		PaymentID: paymentID,
		Currency:  req.Currency,
		Network:   req.Network,
		Context:   c.Request.Context(),
	})
	if err != nil {
		respondPaymentCaptureError(c, err)
		return
	}
	response.Success(c, dto.NewLatestPaymentResp(updated, businessNo))
}

// CapturePayment 用户捕获支付。
func (h *Handler) CapturePayment(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}
	paymentID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		return
	}
	payment, err := h.PaymentService.GetPayment(paymentID)
	if err != nil {
		if errors.Is(err, service.ErrPaymentNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.payment_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	if _, err := h.OrderService.GetOrderByUserForTenant(tenantFromRequest(c), payment.OrderID, uid); err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}
	updated, err := h.PaymentService.CapturePayment(service.CapturePaymentInput{
		PaymentID: paymentID,
		Context:   c.Request.Context(),
	})
	if err != nil {
		respondPaymentCaptureError(c, err)
		return
	}
	response.Success(c, gin.H{
		"payment_id": updated.ID,
		"status":     updated.Status,
	})
}

// GetLatestPayment 获取用户最新待支付记录
func (h *Handler) GetLatestPayment(c *gin.Context) {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return
	}

	var query LatestPaymentQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	order, err := h.OrderService.GetOrderByUserOrderNoForTenant(tenantFromRequest(c), query.OrderNo, uid)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		return
	}

	if order.ParentID != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		return
	}
	if order.Status != constants.OrderStatusPendingPayment {
		shared.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		return
	}
	if order.ExpiresAt != nil && !order.ExpiresAt.After(time.Now()) {
		shared.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		return
	}

	payment, err := h.PaymentRepo.GetLatestPendingByOrder(order.ID, time.Now())
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	if payment == nil {
		shared.RespondError(c, response.CodeNotFound, "error.payment_not_found", nil)
		return
	}

	response.Success(c, dto.NewLatestPaymentResp(payment, order.OrderNo))
}

func respondPaymentCreateError(c *gin.Context, err error) {
	respondWithMappedError(c, err, paymentCreateErrorRules, response.CodeInternal, "error.payment_create_failed")
}

func respondPaymentCaptureError(c *gin.Context, err error) {
	respondWithMappedError(c, err, paymentCaptureErrorRules, response.CodeInternal, "error.payment_callback_failed")
}
