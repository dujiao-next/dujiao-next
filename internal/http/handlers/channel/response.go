package channel

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type mappedChannelError struct {
	target    error
	httpCode  int
	code      int
	errorCode string
	key       string
}

var channelOrderCreateErrorRules = []mappedChannelError{
	{target: service.ErrRiskIPBlacklisted, httpCode: http.StatusForbidden, code: response.CodeForbidden, errorCode: "risk_blocked", key: "error.risk_ip_blacklisted"},
	{target: service.ErrRiskEmailBlacklisted, httpCode: http.StatusForbidden, code: response.CodeForbidden, errorCode: "risk_blocked", key: "error.risk_email_blacklisted"},
	{target: service.ErrRiskTooManyPendingOrders, httpCode: http.StatusTooManyRequests, code: response.CodeTooManyRequests, errorCode: "risk_blocked", key: "error.risk_too_many_pending_orders"},
	{target: service.ErrRiskOrderRateLimited, httpCode: http.StatusTooManyRequests, code: response.CodeTooManyRequests, errorCode: "risk_blocked", key: "error.risk_order_rate_limited"},
	{target: service.ErrProductSKURequired, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.order_item_invalid"},
	{target: service.ErrProductSKUInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "sku_not_found", key: "error.order_item_invalid"},
	{target: service.ErrInvalidOrderItem, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.order_item_invalid"},
	{target: service.ErrInvalidOrderAmount, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.order_amount_invalid"},
	{target: service.ErrProductPurchaseNotAllowed, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "product_unavailable", key: "error.product_purchase_not_allowed"},
	{target: service.ErrProductMaxPurchaseExceeded, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "quantity_limit_exceeded", key: "error.product_max_purchase_exceeded"},
	{target: service.ErrProductMinPurchaseNotMet, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "quantity_below_minimum", key: "error.product_min_purchase_not_met"},
	{target: service.ErrProductNotAvailable, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "product_unavailable", key: "error.product_not_available"},
	{target: service.ErrManualStockInsufficient, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "sku_out_of_stock", key: "error.manual_stock_insufficient"},
	{target: service.ErrCardSecretInsufficient, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "sku_out_of_stock", key: "error.card_secret_insufficient"},
	{target: service.ErrOrderCurrencyMismatch, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.order_currency_mismatch"},
	{target: service.ErrProductPriceInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.product_price_invalid"},
	{target: service.ErrCouponInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_invalid"},
	{target: service.ErrCouponNotFound, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_not_found"},
	{target: service.ErrCouponInactive, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_inactive"},
	{target: service.ErrCouponNotStarted, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_not_started"},
	{target: service.ErrCouponExpired, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_expired"},
	{target: service.ErrCouponUsageLimit, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_usage_limit"},
	{target: service.ErrCouponPerUserLimit, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_per_user_limit"},
	{target: service.ErrCouponMinAmount, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_min_amount"},
	{target: service.ErrCouponScopeInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_scope_invalid"},
	{target: service.ErrCouponPaymentRoleNotAllowed, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_payment_role_not_allowed"},
	{target: service.ErrCouponPaymentRoleGuestOnly, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_payment_role_guest_only"},
	{target: service.ErrCouponPaymentRoleMemberOnly, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_payment_role_member_only"},
	{target: service.ErrCouponMemberLevelNotAllowed, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_member_level_not_allowed"},
	{target: service.ErrCouponWholesaleDisabled, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.coupon_wholesale_disabled"},
	{target: service.ErrPromotionInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "coupon_invalid", key: "error.promotion_invalid"},
	{target: service.ErrManualFormSchemaInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.manual_form_schema_invalid"},
	{target: service.ErrManualFormRequiredMissing, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.manual_form_required_missing"},
	{target: service.ErrManualFormFieldInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.manual_form_field_invalid"},
	{target: service.ErrManualFormTypeInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.manual_form_type_invalid"},
	{target: service.ErrManualFormOptionInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.manual_form_option_invalid"},
}

var channelPaymentCreateErrorRules = []mappedChannelError{
	{target: service.ErrPaymentInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "validation_error", key: "error.payment_invalid"},
	{target: service.ErrOrderNotFound, httpCode: http.StatusNotFound, code: response.CodeNotFound, errorCode: "order_not_found", key: "error.order_not_found"},
	{target: service.ErrOrderStatusInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "order_status_invalid", key: "error.order_status_invalid"},
	{target: service.ErrPaymentChannelNotFound, httpCode: http.StatusNotFound, code: response.CodeNotFound, errorCode: "payment_method_unavailable", key: "error.payment_channel_not_found"},
	{target: service.ErrPaymentChannelInactive, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "payment_method_unavailable", key: "error.payment_channel_inactive"},
	{target: service.ErrPaymentProviderNotSupported, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "payment_method_unavailable", key: "error.payment_provider_not_supported"},
	{target: service.ErrPaymentChannelConfigInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "payment_method_unavailable", key: "error.payment_channel_config_invalid"},
	{target: service.ErrPaymentGatewayRequestFailed, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "payment_create_failed", key: "error.payment_gateway_request_failed"},
	{target: service.ErrPaymentGatewayResponseInvalid, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "payment_create_failed", key: "error.payment_gateway_response_invalid"},
	{target: service.ErrPaymentCurrencyMismatch, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "payment_create_failed", key: "error.payment_currency_mismatch"},
	{target: service.ErrWalletOnlyPaymentRequired, httpCode: http.StatusBadRequest, code: response.CodeBadRequest, errorCode: "wallet_only_payment_required", key: "error.wallet_only_payment_required"},
}

func respondChannelSuccess(c *gin.Context, data interface{}) {
	response.ChannelSuccess(c, data)
}

func respondChannelError(c *gin.Context, httpCode, code int, errorCode, key string, err error) {
	locale := i18n.ResolveLocale(c)
	msg := i18n.T(locale, key)
	if err != nil {
		shared.RequestLog(c).Errorw("channel_handler_error",
			"http_code", httpCode,
			"code", code,
			"error_code", errorCode,
			"message", msg,
			"error", err,
		)
	}
	response.ChannelError(c, httpCode, code, msg, errorCode)
}

func respondChannelBindError(c *gin.Context, err error) {
	locale := i18n.ResolveLocale(c)

	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		details := make([]string, 0, len(ve))
		for _, fe := range ve {
			details = append(details, formatChannelFieldError(locale, fe))
		}
		msg := strings.Join(details, "; ")
		shared.RequestLog(c).Warnw("channel_bind_validation_error", "details", msg, "error", err)
		response.ChannelError(c, http.StatusBadRequest, response.CodeBadRequest, msg, "validation_error")
		return
	}

	msg := i18n.T(locale, "error.bad_request")
	shared.RequestLog(c).Warnw("channel_bind_error", "message", msg, "error", err)
	response.ChannelError(c, http.StatusBadRequest, response.CodeBadRequest, msg, "validation_error")
}

func respondChannelMappedError(c *gin.Context, err error, rules []mappedChannelError, fallbackHTTPCode, fallbackCode int, fallbackErrorCode, fallbackKey string) {
	// 频率限制特殊处理：添加 Retry-After header
	if retryAfter := service.GetRetryAfter(err); retryAfter > 0 {
		c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
	}

	for _, rule := range rules {
		if errors.Is(err, rule.target) {
			respondChannelError(c, rule.httpCode, rule.code, rule.errorCode, rule.key, nil)
			return
		}
	}
	respondChannelError(c, fallbackHTTPCode, fallbackCode, fallbackErrorCode, fallbackKey, err)
}

func respondChannelOrderCreateError(c *gin.Context, err error) {
	respondChannelMappedError(c, err, channelOrderCreateErrorRules, http.StatusBadRequest, response.CodeBadRequest, "order_create_failed", "error.order_create_failed")
}

func respondChannelPaymentCreateError(c *gin.Context, err error) {
	respondChannelMappedError(c, err, channelPaymentCreateErrorRules, http.StatusBadRequest, response.CodeBadRequest, "payment_create_failed", "error.payment_create_failed")
}

func respondChannelOrderPreviewError(c *gin.Context, err error) {
	respondChannelMappedError(c, err, channelOrderCreateErrorRules, http.StatusBadRequest, response.CodeBadRequest, "order_preview_failed", "error.order_create_failed")
}

func respondChannelIdentityServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTelegramAuthPayloadInvalid):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
	case errors.Is(err, service.ErrInvalidEmail):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.email_invalid", nil)
	case errors.Is(err, service.ErrNotFound):
		respondChannelError(c, http.StatusNotFound, response.CodeNotFound, "user_not_found", "error.user_not_found", nil)
	case errors.Is(err, service.ErrVerifyCodeInvalid):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "verify_code_invalid", "error.verify_code_invalid", nil)
	case errors.Is(err, service.ErrVerifyCodeExpired):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "verify_code_expired", "error.verify_code_expired", nil)
	case errors.Is(err, service.ErrVerifyCodeAttemptsExceeded):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "verify_code_invalid", "error.verify_code_attempts_exceeded", nil)
	case errors.Is(err, service.ErrUserDisabled):
		respondChannelError(c, http.StatusUnauthorized, response.CodeUnauthorized, "user_disabled", "error.user_disabled", nil)
	case errors.Is(err, service.ErrUserOAuthIdentityExists):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "channel_identity_conflict", "error.telegram_bind_conflict", nil)
	case errors.Is(err, service.ErrUserOAuthAlreadyBound):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "channel_identity_conflict", "error.telegram_already_bound", nil)
	default:
		respondChannelError(c, http.StatusInternalServerError, response.CodeInternal, "internal_error", "error.internal_error", err)
	}
}

func formatChannelFieldError(locale string, fe validator.FieldError) string {
	field := fe.Field()
	tag := fe.Tag()
	param := fe.Param()

	customKey := "validation." + field + "." + tag
	if msg := i18n.T(locale, customKey); msg != customKey {
		return msg
	}

	ruleKey := "validation.rule." + tag
	if ruleMsg := i18n.T(locale, ruleKey); ruleMsg != ruleKey {
		if param != "" {
			return field + ": " + i18n.Sprintf(locale, ruleKey, param)
		}
		return field + ": " + ruleMsg
	}

	if param != "" {
		return field + ": " + tag + "=" + param
	}
	return field + ": " + tag
}

func channelUserIDValue(primary, legacy string) string {
	if value := strings.TrimSpace(primary); value != "" {
		return value
	}
	return strings.TrimSpace(legacy)
}

func channelUserIDFromQuery(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return channelUserIDValue(c.Query("channel_user_id"), c.Query("telegram_user_id"))
}
