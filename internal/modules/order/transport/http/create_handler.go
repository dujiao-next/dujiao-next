package orderhttp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	paymentpresenter "github.com/dujiao-next/internal/modules/payment/transport/presenter"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/i18n"
	captcha "github.com/dujiao-next/internal/modules/captcha/contract"
	captchahttp "github.com/dujiao-next/internal/modules/captcha/transport/http"
	orderpresenter "github.com/dujiao-next/internal/modules/order/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
)

func ensureBrowserOrderToken(c *gin.Context) (string, error) {
	if raw, err := c.Cookie(browserOrderCookieName); err == nil && browserTokenHash(raw) != "" {
		return raw, nil
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func setBrowserOrderCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: browserOrderCookieName, Value: token, Path: "/", MaxAge: 365 * 24 * 60 * 60,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

// OrderCreateService 订单创建端口。
type OrderCreateService interface {
	CreateOrder(input CreateOrderInput) (*orderdomain.Order, error)
	CreateGuestOrder(input CreateGuestOrderInput) (*orderdomain.Order, error)
}

// GuestCreateCaptcha 游客下单验证码端口。
type GuestCreateCaptcha interface {
	VerifyGuestCreateOrder(payload captchahttp.CaptchaPayloadRequest, clientIP string) error
}

// CreatePaymentInput 创建支付输入。
type CreatePaymentInput struct {
	OrderID       uint
	ChannelID     uint
	UseBalance    bool
	ClientIP      string
	Context       context.Context
	RequestScheme string
}

// CreatePaymentResult 创建支付结果。
type CreatePaymentResult struct {
	Payment          *paymentdomain.Payment
	OrderPaid        bool
	WalletPaidAmount money.Amount
	OnlinePayAmount  money.Amount
}

// OrderPaymentCreator 订单支付创建端口。
type OrderPaymentCreator interface {
	CreatePayment(input CreatePaymentInput) (*CreatePaymentResult, error)
}

// CreateOrderAndPayRequest 创建订单并发起支付请求
type CreateOrderAndPayRequest struct {
	Items               []OrderItemRequest      `json:"items" binding:"required"`
	CouponCode          string                  `json:"coupon_code"`
	AffiliateCode       string                  `json:"affiliate_code"`
	AffiliateVisitorKey string                  `json:"affiliate_visitor_key"`
	ManualFormData      map[string]jsonmap.JSON `json:"manual_form_data"`
	ChannelID           uint                    `json:"channel_id"`
	UseBalance          bool                    `json:"use_balance"`
}

// CreateGuestOrderAndPayRequest 游客创建订单并发起支付请求
type CreateGuestOrderAndPayRequest struct {
	Email               string                            `json:"email" binding:"required"`
	OrderPassword       string                            `json:"order_password" binding:"required"`
	Items               []OrderItemRequest                `json:"items" binding:"required"`
	CouponCode          string                            `json:"coupon_code"`
	AffiliateCode       string                            `json:"affiliate_code"`
	AffiliateVisitorKey string                            `json:"affiliate_visitor_key"`
	ManualFormData      map[string]jsonmap.JSON           `json:"manual_form_data"`
	CaptchaPayload      captchahttp.CaptchaPayloadRequest `json:"captcha_payload"`
	ChannelID           uint                              `json:"channel_id"`
}

// CreateHandler 处理前台订单创建与 create-and-pay HTTP。
type CreateHandler struct {
	orders   OrderCreateService
	payments PaymentChannelPolicy
	captcha  GuestCreateCaptcha
	pay      OrderPaymentCreator
}

func NewCreateHandler(orders OrderCreateService, payments PaymentChannelPolicy, captcha GuestCreateCaptcha, pay OrderPaymentCreator) *CreateHandler {
	if orders == nil {
		panic("order create handler: orders is nil")
	}
	return &CreateHandler{orders: orders, payments: payments, captcha: captcha, pay: pay}
}

// CreateOrder 创建订单
func (h *CreateHandler) CreateOrder(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	order, err := h.orders.CreateOrder(CreateOrderInput{
		UserID:              uid,
		Tenant:              tenantFromRequest(c),
		Items:               mapOrderItems(req.Items),
		CouponCode:          req.CouponCode,
		AffiliateCode:       req.AffiliateCode,
		AffiliateVisitorKey: req.AffiliateVisitorKey,
		ClientIP:            c.ClientIP(),
		ManualFormData:      req.ManualFormData,
	})
	if err != nil {
		respondUserOrderCreateError(c, err)
		return
	}

	orderDetail := orderpresenter.NewOrderDetail(order)
	enrichOrderWithAllowedChannels(h.payments, order, &orderDetail)
	response.Success(c, orderDetail)
}

// CreateGuestOrder 游客创建订单
func (h *CreateHandler) CreateGuestOrder(c *gin.Context) {
	var req CreateGuestOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if !h.verifyGuestCreateCaptcha(c, req.CaptchaPayload) {
		return
	}

	browserToken, err := ensureBrowserOrderToken(c)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_create_failed", err)
		return
	}
	order, err := h.orders.CreateGuestOrder(CreateGuestOrderInput{
		Email:               req.Email,
		OrderPassword:       req.OrderPassword,
		Locale:              i18n.ResolveLocale(c),
		BrowserTokenHash:    browserTokenHash(browserToken),
		Tenant:              tenantFromRequest(c),
		Items:               mapOrderItems(req.Items),
		CouponCode:          req.CouponCode,
		AffiliateCode:       req.AffiliateCode,
		AffiliateVisitorKey: req.AffiliateVisitorKey,
		ClientIP:            c.ClientIP(),
		ManualFormData:      req.ManualFormData,
	})
	if err != nil {
		respondGuestOrderCreateError(c, err)
		return
	}
	setBrowserOrderCookie(c, browserToken)
	orderDetail := orderpresenter.NewOrderDetail(order)
	enrichOrderWithAllowedChannels(h.payments, order, &orderDetail)
	response.Success(c, orderDetail)
}

// CreateOrderAndPay 创建订单并发起支付（合并接口）
func (h *CreateHandler) CreateOrderAndPay(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req CreateOrderAndPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	order, err := h.orders.CreateOrder(CreateOrderInput{
		UserID:              uid,
		Tenant:              tenantFromRequest(c),
		Items:               mapOrderItems(req.Items),
		CouponCode:          req.CouponCode,
		AffiliateCode:       req.AffiliateCode,
		AffiliateVisitorKey: req.AffiliateVisitorKey,
		ClientIP:            c.ClientIP(),
		ManualFormData:      req.ManualFormData,
	})
	if err != nil {
		respondUserOrderCreateError(c, err)
		return
	}

	orderResp := orderpresenter.NewOrderDetail(order)
	enrichOrderWithAllowedChannels(h.payments, order, &orderResp)

	if req.ChannelID == 0 && !req.UseBalance {
		response.Success(c, gin.H{
			"order":    orderResp,
			"order_no": order.OrderNo,
		})
		return
	}

	h.respondCreateAndPay(c, order, orderResp, req.ChannelID, req.UseBalance)
}

// CreateGuestOrderAndPay 游客创建订单并发起支付（合并接口）
func (h *CreateHandler) CreateGuestOrderAndPay(c *gin.Context) {
	var req CreateGuestOrderAndPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if !h.verifyGuestCreateCaptcha(c, req.CaptchaPayload) {
		return
	}

	browserToken, err := ensureBrowserOrderToken(c)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_create_failed", err)
		return
	}
	order, err := h.orders.CreateGuestOrder(CreateGuestOrderInput{
		Email:               req.Email,
		OrderPassword:       req.OrderPassword,
		Locale:              i18n.ResolveLocale(c),
		BrowserTokenHash:    browserTokenHash(browserToken),
		Tenant:              tenantFromRequest(c),
		Items:               mapOrderItems(req.Items),
		CouponCode:          req.CouponCode,
		AffiliateCode:       req.AffiliateCode,
		AffiliateVisitorKey: req.AffiliateVisitorKey,
		ClientIP:            c.ClientIP(),
		ManualFormData:      req.ManualFormData,
	})
	if err != nil {
		respondGuestOrderCreateError(c, err)
		return
	}
	setBrowserOrderCookie(c, browserToken)
	orderResp := orderpresenter.NewOrderDetail(order)
	enrichOrderWithAllowedChannels(h.payments, order, &orderResp)

	if req.ChannelID == 0 {
		response.Success(c, gin.H{
			"order":    orderResp,
			"order_no": order.OrderNo,
		})
		return
	}

	h.respondCreateAndPay(c, order, orderResp, req.ChannelID, false)
}

func (h *CreateHandler) verifyGuestCreateCaptcha(c *gin.Context, payload captchahttp.CaptchaPayloadRequest) bool {
	if h.captcha == nil {
		return true
	}
	if captchaErr := h.captcha.VerifyGuestCreateOrder(payload, c.ClientIP()); captchaErr != nil {
		switch {
		case errors.Is(captchaErr, captcha.ErrRequired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.captcha_required", nil)
		case errors.Is(captchaErr, captcha.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.captcha_invalid", nil)
		case errors.Is(captchaErr, captcha.ErrConfigInvalid):
			ginutil.RespondError(c, response.CodeInternal, "error.captcha_config_invalid", captchaErr)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.captcha_verify_failed", captchaErr)
		}
		return false
	}
	return true
}

func (h *CreateHandler) respondCreateAndPay(c *gin.Context, order *orderdomain.Order, orderResp orderpresenter.OrderDetail, channelID uint, useBalance bool) {
	if h.pay == nil {
		response.Success(c, gin.H{
			"order":         orderResp,
			"order_no":      order.OrderNo,
			"payment_error": "payment creator unavailable",
		})
		return
	}
	result, err := h.pay.CreatePayment(CreatePaymentInput{
		OrderID:       order.ID,
		ChannelID:     channelID,
		UseBalance:    useBalance,
		ClientIP:      c.ClientIP(),
		Context:       c.Request.Context(),
		RequestScheme: requestSchemeFromContext(c),
	})
	if err != nil {
		response.Success(c, gin.H{
			"order":         orderResp,
			"order_no":      order.OrderNo,
			"payment_error": err.Error(),
		})
		return
	}

	resp := gin.H{
		"order":              orderResp,
		"order_no":           order.OrderNo,
		"order_paid":         result.OrderPaid,
		"wallet_paid_amount": result.WalletPaidAmount,
		"online_pay_amount":  result.OnlinePayAmount,
	}
	if result.Payment != nil {
		resp["payment_id"] = result.Payment.ID
		resp["provider_type"] = result.Payment.ProviderType
		resp["channel_type"] = result.Payment.ChannelType
		resp["interaction_mode"] = result.Payment.InteractionMode
		resp["pay_url"] = result.Payment.PayURL
		resp["qr_code"] = result.Payment.QRCode
		if info := paymentpresenter.ExtractCryptoWalletInfo(result.Payment.ProviderType, result.Payment.InteractionMode, result.Payment.ProviderPayload); info.HasAny() {
			if info.Address != "" {
				resp["wallet_address"] = info.Address
			}
			if info.ChainAmount != "" {
				resp["chain_amount"] = info.ChainAmount
			}
			if info.Chain != "" {
				resp["chain"] = info.Chain
			}
			if info.TokenID != "" {
				resp["token_id"] = info.TokenID
			}
		}
		resp["expires_at"] = result.Payment.ExpiredAt
	}
	response.Success(c, resp)
}

// requestSchemeFromContext 返回当前请求的 scheme（优先 X-Forwarded-Proto）。
func requestSchemeFromContext(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		proto = strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
		if proto == "http" || proto == "https" {
			return proto
		}
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}
