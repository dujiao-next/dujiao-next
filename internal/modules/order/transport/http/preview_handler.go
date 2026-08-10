package orderhttp

import (
	"errors"
	"fmt"
	"strconv"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"

	"github.com/dujiao-next/internal/i18n"
	captchahttp "github.com/dujiao-next/internal/modules/captcha/transport/http"
	cardsecretapp "github.com/dujiao-next/internal/modules/cardsecret/application"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	resellermodule "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
)

var (
	ErrProductSKURequired        = errors.New("product sku required")
	ErrInvalidOrderAmount        = errors.New("invalid order amount")
	ErrGuestEmailRequired        = errors.New("guest email required")
	ErrGuestPasswordRequired     = errors.New("guest password required")
	ErrGuestPasswordTooWeak      = errors.New("guest password too weak")
	ErrInvalidEmail              = errors.New("invalid email")
	ErrProductPurchaseNotAllowed = errors.New("product purchase not allowed")
	ErrGuestCouponNotAllowed     = errors.New("guest coupon not allowed")
	ErrManualStockInsufficient   = errors.New("manual stock insufficient")
	ErrOrderCurrencyMismatch     = errors.New("order currency mismatch")
	ErrProductNotAvailable       = errors.New("product not available")
	ErrResellerCouponNotAllowed  = errors.New("reseller coupon not allowed")
	ErrQueueUnavailable          = errors.New("queue unavailable")
	ErrRiskIPBlacklisted         = errors.New("risk: ip blacklisted")
	ErrRiskClientIPUnavailable   = errors.New("risk: client ip unavailable")
	ErrRiskTooManyPendingOrders  = errors.New("risk: too many pending orders")
	ErrRiskProductQuantityLimit  = errors.New("risk: product quantity limit")
	ErrRiskPendingProductLimit   = errors.New("risk: pending product quantity limit")
	ErrRiskOrderRateLimited      = errors.New("risk: order rate limited")
)

type riskRateLimitedError struct {
	retryAfter int64
	err        error
}

func (e *riskRateLimitedError) Error() string {
	if e == nil || e.err == nil {
		return ErrRiskOrderRateLimited.Error()
	}
	return e.err.Error()
}

func (e *riskRateLimitedError) Unwrap() error { return e.err }

func (e *riskRateLimitedError) RetryAfterSeconds() int64 {
	if e == nil {
		return 0
	}
	return e.retryAfter
}

// WrapRiskRateLimited 将限流错误包装为可提取 Retry-After 的 transport 错误。
func WrapRiskRateLimited(retryAfter int64, err error) error {
	wrapped := err
	if !errors.Is(err, ErrRiskOrderRateLimited) {
		wrapped = fmt.Errorf("%w: %v", ErrRiskOrderRateLimited, err)
	}
	return &riskRateLimitedError{retryAfter: retryAfter, err: wrapped}
}

// OrderItemRequest 订单项请求
type OrderItemRequest struct {
	ProductID       uint   `json:"product_id" binding:"required"`
	SKUID           uint   `json:"sku_id"`
	Quantity        int    `json:"quantity" binding:"required"`
	FulfillmentType string `json:"fulfillment_type"`
}

// CreateOrderRequest 用户订单预览/创建请求体（preview 使用）。
type CreateOrderRequest struct {
	Items               []OrderItemRequest      `json:"items" binding:"required"`
	CouponCode          string                  `json:"coupon_code"`
	AffiliateCode       string                  `json:"affiliate_code"`
	AffiliateVisitorKey string                  `json:"affiliate_visitor_key"`
	ManualFormData      map[string]jsonmap.JSON `json:"manual_form_data"`
}

// CreateGuestOrderRequest 游客订单预览请求体。
type CreateGuestOrderRequest struct {
	Email               string                            `json:"email" binding:"required"`
	OrderPassword       string                            `json:"order_password" binding:"required"`
	Items               []OrderItemRequest                `json:"items" binding:"required"`
	CouponCode          string                            `json:"coupon_code"`
	AffiliateCode       string                            `json:"affiliate_code"`
	AffiliateVisitorKey string                            `json:"affiliate_visitor_key"`
	ManualFormData      map[string]jsonmap.JSON           `json:"manual_form_data"`
	CaptchaPayload      captchahttp.CaptchaPayloadRequest `json:"captcha_payload"`
}

// CreateOrderItem 创建/预览订单项。
type CreateOrderItem struct {
	ProductID       uint
	SKUID           uint
	Quantity        int
	FulfillmentType string
}

// CreateOrderInput 用户订单预览输入。
type CreateOrderInput struct {
	UserID              uint
	Tenant              resellermodule.TenantContext
	Items               []CreateOrderItem
	CouponCode          string
	AffiliateCode       string
	AffiliateVisitorKey string
	ClientIP            string
	ManualFormData      map[string]jsonmap.JSON
}

// CreateGuestOrderInput 游客订单预览输入。
type CreateGuestOrderInput struct {
	Email               string
	OrderPassword       string
	Locale              string
	Tenant              resellermodule.TenantContext
	Items               []CreateOrderItem
	CouponCode          string
	AffiliateCode       string
	AffiliateVisitorKey string
	ClientIP            string
	ManualFormData      map[string]jsonmap.JSON
}

// OrderPreview 订单金额预览。
type OrderPreview struct {
	Currency                string             `json:"currency"`
	OriginalAmount          money.Amount       `json:"original_amount"`
	MemberDiscountAmount    money.Amount       `json:"member_discount_amount"`
	DiscountAmount          money.Amount       `json:"discount_amount"`
	PromotionDiscountAmount money.Amount       `json:"promotion_discount_amount"`
	WholesaleDiscountAmount money.Amount       `json:"wholesale_discount_amount"`
	TotalAmount             money.Amount       `json:"total_amount"`
	Items                   []OrderPreviewItem `json:"items"`
}

// OrderPreviewItem 订单项金额预览。
type OrderPreviewItem struct {
	ProductID          uint              `json:"product_id"`
	SKUID              uint              `json:"sku_id"`
	TitleJSON          jsonmap.JSON      `json:"title"`
	SKUSnapshotJSON    jsonmap.JSON      `json:"sku_snapshot"`
	Tags               jsonslice.Strings `json:"tags"`
	OriginalUnitPrice  money.Amount      `json:"original_unit_price"`
	UnitPrice          money.Amount      `json:"unit_price"`
	Quantity           int               `json:"quantity"`
	OriginalTotalPrice money.Amount      `json:"original_total_price"`
	TotalPrice         money.Amount      `json:"total_price"`
	MemberDiscount     money.Amount      `json:"member_discount_amount"`
	CouponDiscount     money.Amount      `json:"coupon_discount_amount"`
	PromotionDiscount  money.Amount      `json:"promotion_discount_amount"`
	WholesaleDiscount  money.Amount      `json:"wholesale_discount_amount"`
	FulfillmentType    string            `json:"fulfillment_type"`
}

// OrderPreviewService 订单金额预览端口。
type OrderPreviewService interface {
	PreviewOrder(input CreateOrderInput) (*OrderPreview, error)
	PreviewGuestOrder(input CreateGuestOrderInput) (*OrderPreview, error)
}

// PreviewHandler 处理前台订单金额预览 HTTP。
type PreviewHandler struct {
	orders OrderPreviewService
}

func NewPreviewHandler(orders OrderPreviewService) *PreviewHandler {
	if orders == nil {
		panic("order preview handler: orders is nil")
	}
	return &PreviewHandler{orders: orders}
}

// PreviewOrder 用户订单金额预览
func (h *PreviewHandler) PreviewOrder(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	preview, err := h.orders.PreviewOrder(CreateOrderInput{
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
		respondUserOrderPreviewError(c, err)
		return
	}
	response.Success(c, preview)
}

// PreviewGuestOrder 游客订单金额预览
func (h *PreviewHandler) PreviewGuestOrder(c *gin.Context) {
	var req CreateGuestOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	preview, err := h.orders.PreviewGuestOrder(CreateGuestOrderInput{
		Email:               req.Email,
		OrderPassword:       req.OrderPassword,
		Locale:              i18n.ResolveLocale(c),
		Tenant:              tenantFromRequest(c),
		Items:               mapOrderItems(req.Items),
		CouponCode:          req.CouponCode,
		AffiliateCode:       req.AffiliateCode,
		AffiliateVisitorKey: req.AffiliateVisitorKey,
		ClientIP:            c.ClientIP(),
		ManualFormData:      req.ManualFormData,
	})
	if err != nil {
		respondGuestOrderPreviewError(c, err)
		return
	}
	response.Success(c, preview)
}

func mapOrderItems(items []OrderItemRequest) []CreateOrderItem {
	out := make([]CreateOrderItem, 0, len(items))
	for _, item := range items {
		out = append(out, CreateOrderItem(item))
	}
	return out
}

type mappedError struct {
	target error
	code   int
	key    string
	logErr bool
}

func respondWithMappedError(c *gin.Context, err error, rules []mappedError, fallbackCode int, fallbackKey string) {
	if retryAfter := retryAfterSeconds(err); retryAfter > 0 {
		c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
		locale := i18n.ResolveLocale(c)
		msg := fmt.Sprintf("%s (%ds)", i18n.T(locale, "error.risk_order_rate_limited"), retryAfter)
		response.Error(c, response.CodeTooManyRequests, msg)
		return
	}
	for _, rule := range rules {
		if errors.Is(err, rule.target) {
			var cause error
			if rule.logErr {
				cause = err
			}
			ginutil.RespondError(c, rule.code, rule.key, cause)
			return
		}
	}
	ginutil.RespondError(c, fallbackCode, fallbackKey, err)
}

type retryAfterCarrier interface {
	RetryAfterSeconds() int64
}

func retryAfterSeconds(err error) int64 {
	var carrier retryAfterCarrier
	if errors.As(err, &carrier) {
		return carrier.RetryAfterSeconds()
	}
	return 0
}

func respondUserOrderPreviewError(c *gin.Context, err error) {
	rules := append([]mappedError{}, orderRiskControlErrorRules...)
	rules = append(rules, userOrderCommonErrorRules...)
	rules = append(rules, userOrderPreviewExtraErrorRules...)
	respondWithMappedError(c, err, rules, response.CodeInternal, "error.order_create_failed")
}

func respondGuestOrderPreviewError(c *gin.Context, err error) {
	rules := append([]mappedError{}, orderRiskControlErrorRules...)
	rules = append(rules, guestOrderCommonErrorRules...)
	rules = append(rules, guestOrderPreviewExtraErrorRules...)
	respondWithMappedError(c, err, rules, response.CodeInternal, "error.order_create_failed")
}

func respondUserOrderCreateError(c *gin.Context, err error) {
	respondWithMappedError(c, err, append(orderRiskControlErrorRules, userOrderCommonErrorRules...), response.CodeInternal, "error.order_create_failed")
}

func respondGuestOrderCreateError(c *gin.Context, err error) {
	rules := append([]mappedError{}, orderRiskControlErrorRules...)
	rules = append(rules, guestOrderCommonErrorRules...)
	rules = append(rules, guestOrderCreateExtraErrorRules...)
	respondWithMappedError(c, err, rules, response.CodeInternal, "error.order_create_failed")
}

var orderRiskControlErrorRules = []mappedError{
	{target: ErrRiskIPBlacklisted, code: response.CodeForbidden, key: "error.risk_ip_blacklisted"},
	{target: ErrRiskClientIPUnavailable, code: response.CodeForbidden, key: "error.risk_client_ip_unavailable"},
	{target: ErrRiskTooManyPendingOrders, code: response.CodeTooManyRequests, key: "error.risk_too_many_pending_orders"},
	{target: ErrRiskProductQuantityLimit, code: response.CodeBadRequest, key: "error.risk_product_quantity_limit"},
	{target: ErrRiskPendingProductLimit, code: response.CodeTooManyRequests, key: "error.risk_pending_product_quantity_limit"},
	{target: ErrRiskOrderRateLimited, code: response.CodeTooManyRequests, key: "error.risk_order_rate_limited"},
}

var guestOrderCreateExtraErrorRules = []mappedError{
	{target: ErrQueueUnavailable, code: response.CodeInternal, key: "error.queue_unavailable"},
}

var userOrderCommonErrorRules = []mappedError{
	{target: ErrProductSKURequired, code: response.CodeBadRequest, key: "error.order_item_invalid"},
	{target: productcontract.ErrProductSKUInvalid, code: response.CodeBadRequest, key: "error.order_item_invalid"},
	{target: productdomain.ErrPurchaseQuantityInvalid, code: response.CodeBadRequest, key: "error.order_item_invalid"},
	{target: ErrInvalidOrderAmount, code: response.CodeBadRequest, key: "error.order_amount_invalid"},
	{target: ErrProductPurchaseNotAllowed, code: response.CodeBadRequest, key: "error.product_purchase_not_allowed"},
	{target: productdomain.ErrMaxPurchaseExceeded, code: response.CodeBadRequest, key: "error.product_max_purchase_exceeded"},
	{target: productdomain.ErrMinPurchaseNotMet, code: response.CodeBadRequest, key: "error.product_min_purchase_not_met"},
	{target: ErrManualStockInsufficient, code: response.CodeBadRequest, key: "error.manual_stock_insufficient"},
	{target: cardsecretapp.ErrInsufficient, code: response.CodeBadRequest, key: "error.card_secret_insufficient"},
	{target: ErrOrderCurrencyMismatch, code: response.CodeBadRequest, key: "error.order_currency_mismatch"},
	{target: productcontract.ErrProductPriceInvalid, code: response.CodeBadRequest, key: "error.product_price_invalid"},
	{target: ErrProductNotAvailable, code: response.CodeBadRequest, key: "error.product_not_available"},
	{target: productcontract.ErrResellerProductNotListed, code: response.CodeBadRequest, key: "error.reseller_product_not_listed"},
	{target: resellermodule.ErrPriceBelowBase, code: response.CodeBadRequest, key: "error.reseller_price_invalid"},
	{target: resellermodule.ErrMarkupExceeded, code: response.CodeBadRequest, key: "error.reseller_markup_exceeded"},
	{target: ErrResellerCouponNotAllowed, code: response.CodeBadRequest, key: "error.reseller_coupon_not_allowed"},
	{target: resellermodule.ErrPricingModeInvalid, code: response.CodeBadRequest, key: "error.reseller_price_invalid"},
	{target: couponcontract.ErrInvalid, code: response.CodeBadRequest, key: "error.coupon_invalid"},
	{target: couponcontract.ErrNotFound, code: response.CodeBadRequest, key: "error.coupon_not_found"},
	{target: couponcontract.ErrInactive, code: response.CodeBadRequest, key: "error.coupon_inactive"},
	{target: couponcontract.ErrNotStarted, code: response.CodeBadRequest, key: "error.coupon_not_started"},
	{target: couponcontract.ErrExpired, code: response.CodeBadRequest, key: "error.coupon_expired"},
	{target: couponcontract.ErrUsageLimit, code: response.CodeBadRequest, key: "error.coupon_usage_limit"},
	{target: couponcontract.ErrPerUserLimit, code: response.CodeBadRequest, key: "error.coupon_per_user_limit"},
	{target: couponcontract.ErrMinAmount, code: response.CodeBadRequest, key: "error.coupon_min_amount"},
	{target: couponcontract.ErrScopeInvalid, code: response.CodeBadRequest, key: "error.coupon_scope_invalid"},
	{target: couponcontract.ErrPaymentRoleNotAllowed, code: response.CodeBadRequest, key: "error.coupon_payment_role_not_allowed"},
	{target: couponcontract.ErrPaymentRoleGuestOnly, code: response.CodeBadRequest, key: "error.coupon_payment_role_guest_only"},
	{target: couponcontract.ErrPaymentRoleMemberOnly, code: response.CodeBadRequest, key: "error.coupon_payment_role_member_only"},
	{target: couponcontract.ErrMemberLevelNotAllowed, code: response.CodeBadRequest, key: "error.coupon_member_level_not_allowed"},
	{target: couponcontract.ErrWholesaleDisabled, code: response.CodeBadRequest, key: "error.coupon_wholesale_disabled"},
	{target: promotioncontract.ErrInvalid, code: response.CodeBadRequest, key: "error.promotion_invalid"},
	{target: manualform.ErrSchemaInvalid, code: response.CodeBadRequest, key: "error.manual_form_schema_invalid"},
	{target: manualform.ErrRequiredMissing, code: response.CodeBadRequest, key: "error.manual_form_required_missing"},
	{target: manualform.ErrFieldInvalid, code: response.CodeBadRequest, key: "error.manual_form_field_invalid"},
	{target: manualform.ErrTypeInvalid, code: response.CodeBadRequest, key: "error.manual_form_type_invalid"},
	{target: manualform.ErrOptionInvalid, code: response.CodeBadRequest, key: "error.manual_form_option_invalid"},
}

var userOrderPreviewExtraErrorRules = []mappedError{
	{target: ErrQueueUnavailable, code: response.CodeInternal, key: "error.queue_unavailable"},
}

var guestOrderCommonErrorRules = []mappedError{
	{target: ErrProductSKURequired, code: response.CodeBadRequest, key: "error.order_item_invalid"},
	{target: productcontract.ErrProductSKUInvalid, code: response.CodeBadRequest, key: "error.order_item_invalid"},
	{target: ErrGuestEmailRequired, code: response.CodeBadRequest, key: "error.guest_email_required"},
	{target: ErrGuestPasswordRequired, code: response.CodeBadRequest, key: "error.guest_password_required"},
	{target: ErrGuestPasswordTooWeak, code: response.CodeBadRequest, key: "error.guest_password_too_weak"},
	{target: ErrInvalidEmail, code: response.CodeBadRequest, key: "error.email_invalid"},
	{target: ErrProductPurchaseNotAllowed, code: response.CodeBadRequest, key: "error.product_purchase_not_allowed"},
	{target: productdomain.ErrMaxPurchaseExceeded, code: response.CodeBadRequest, key: "error.product_max_purchase_exceeded"},
	{target: productdomain.ErrMinPurchaseNotMet, code: response.CodeBadRequest, key: "error.product_min_purchase_not_met"},
	{target: ErrGuestCouponNotAllowed, code: response.CodeBadRequest, key: "error.guest_coupon_not_allowed"},
	{target: productdomain.ErrPurchaseQuantityInvalid, code: response.CodeBadRequest, key: "error.order_item_invalid"},
	{target: ErrInvalidOrderAmount, code: response.CodeBadRequest, key: "error.order_amount_invalid"},
	{target: ErrManualStockInsufficient, code: response.CodeBadRequest, key: "error.manual_stock_insufficient"},
	{target: cardsecretapp.ErrInsufficient, code: response.CodeBadRequest, key: "error.card_secret_insufficient"},
	{target: ErrOrderCurrencyMismatch, code: response.CodeBadRequest, key: "error.order_currency_mismatch"},
	{target: productcontract.ErrProductPriceInvalid, code: response.CodeBadRequest, key: "error.product_price_invalid"},
	{target: ErrProductNotAvailable, code: response.CodeBadRequest, key: "error.product_not_available"},
	{target: productcontract.ErrResellerProductNotListed, code: response.CodeBadRequest, key: "error.reseller_product_not_listed"},
	{target: resellermodule.ErrPriceBelowBase, code: response.CodeBadRequest, key: "error.reseller_price_invalid"},
	{target: resellermodule.ErrMarkupExceeded, code: response.CodeBadRequest, key: "error.reseller_markup_exceeded"},
	{target: ErrResellerCouponNotAllowed, code: response.CodeBadRequest, key: "error.reseller_coupon_not_allowed"},
	{target: resellermodule.ErrPricingModeInvalid, code: response.CodeBadRequest, key: "error.reseller_price_invalid"},
	{target: couponcontract.ErrWholesaleDisabled, code: response.CodeBadRequest, key: "error.coupon_wholesale_disabled"},
	{target: manualform.ErrSchemaInvalid, code: response.CodeBadRequest, key: "error.manual_form_schema_invalid"},
	{target: manualform.ErrRequiredMissing, code: response.CodeBadRequest, key: "error.manual_form_required_missing"},
	{target: manualform.ErrFieldInvalid, code: response.CodeBadRequest, key: "error.manual_form_field_invalid"},
	{target: manualform.ErrTypeInvalid, code: response.CodeBadRequest, key: "error.manual_form_type_invalid"},
	{target: manualform.ErrOptionInvalid, code: response.CodeBadRequest, key: "error.manual_form_option_invalid"},
}

var guestOrderPreviewExtraErrorRules = []mappedError{
	{target: couponcontract.ErrInvalid, code: response.CodeBadRequest, key: "error.coupon_invalid"},
	{target: couponcontract.ErrNotFound, code: response.CodeBadRequest, key: "error.coupon_not_found"},
	{target: couponcontract.ErrInactive, code: response.CodeBadRequest, key: "error.coupon_inactive"},
	{target: couponcontract.ErrNotStarted, code: response.CodeBadRequest, key: "error.coupon_not_started"},
	{target: couponcontract.ErrExpired, code: response.CodeBadRequest, key: "error.coupon_expired"},
	{target: couponcontract.ErrUsageLimit, code: response.CodeBadRequest, key: "error.coupon_usage_limit"},
	{target: couponcontract.ErrPerUserLimit, code: response.CodeBadRequest, key: "error.coupon_per_user_limit"},
	{target: couponcontract.ErrMinAmount, code: response.CodeBadRequest, key: "error.coupon_min_amount"},
	{target: couponcontract.ErrScopeInvalid, code: response.CodeBadRequest, key: "error.coupon_scope_invalid"},
	{target: couponcontract.ErrPaymentRoleNotAllowed, code: response.CodeBadRequest, key: "error.coupon_payment_role_not_allowed"},
	{target: couponcontract.ErrPaymentRoleGuestOnly, code: response.CodeBadRequest, key: "error.coupon_payment_role_guest_only"},
	{target: couponcontract.ErrPaymentRoleMemberOnly, code: response.CodeBadRequest, key: "error.coupon_payment_role_member_only"},
	{target: couponcontract.ErrMemberLevelNotAllowed, code: response.CodeBadRequest, key: "error.coupon_member_level_not_allowed"},
	{target: couponcontract.ErrWholesaleDisabled, code: response.CodeBadRequest, key: "error.coupon_wholesale_disabled"},
	{target: promotioncontract.ErrInvalid, code: response.CodeBadRequest, key: "error.promotion_invalid"},
}
