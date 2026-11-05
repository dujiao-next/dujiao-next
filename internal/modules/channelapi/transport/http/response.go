package channelhttp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/i18n"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

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

func channelErrorRule(target error, httpCode, code int, errorCode, key string) mappedChannelError {
	return mappedChannelError{target: target, httpCode: httpCode, code: code, errorCode: errorCode, key: key}
}

var channelOrderCreateErrorRules = []mappedChannelError{
	channelErrorRule(ErrRiskIPBlacklisted, http.StatusForbidden, response.CodeForbidden, "risk_blocked", "error.risk_ip_blacklisted"),
	channelErrorRule(ErrRiskClientIPUnavailable, http.StatusForbidden, response.CodeForbidden, "risk_blocked", "error.risk_client_ip_unavailable"),
	channelErrorRule(ErrRiskTooManyPendingOrders, http.StatusTooManyRequests, response.CodeTooManyRequests, "risk_blocked", "error.risk_too_many_pending_orders"),
	channelErrorRule(ErrRiskProductQuantityLimit, http.StatusBadRequest, response.CodeBadRequest, "quantity_limit_exceeded", "error.risk_product_quantity_limit"),
	channelErrorRule(ErrRiskPendingProductLimit, http.StatusTooManyRequests, response.CodeTooManyRequests, "risk_blocked", "error.risk_pending_product_quantity_limit"),
	channelErrorRule(ErrRiskOrderRateLimited, http.StatusTooManyRequests, response.CodeTooManyRequests, "risk_blocked", "error.risk_order_rate_limited"),
	channelErrorRule(ErrProductSKURequired, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.order_item_invalid"),
	channelErrorRule(ErrProductSKUInvalid, http.StatusBadRequest, response.CodeBadRequest, "sku_not_found", "error.order_item_invalid"),
	channelErrorRule(ErrInvalidOrderItem, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.order_item_invalid"),
	channelErrorRule(ErrInvalidOrderAmount, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.order_amount_invalid"),
	channelErrorRule(ErrProductPurchaseNotAllowed, http.StatusBadRequest, response.CodeBadRequest, "product_unavailable", "error.product_purchase_not_allowed"),
	channelErrorRule(ErrProductMaxPurchaseExceeded, http.StatusBadRequest, response.CodeBadRequest, "quantity_limit_exceeded", "error.product_max_purchase_exceeded"),
	channelErrorRule(ErrProductMinPurchaseNotMet, http.StatusBadRequest, response.CodeBadRequest, "quantity_below_minimum", "error.product_min_purchase_not_met"),
	channelErrorRule(ErrProductNotAvailable, http.StatusBadRequest, response.CodeBadRequest, "product_unavailable", "error.product_not_available"),
	channelErrorRule(ErrManualStockInsufficient, http.StatusBadRequest, response.CodeBadRequest, "sku_out_of_stock", "error.manual_stock_insufficient"),
	channelErrorRule(ErrCardSecretInsufficient, http.StatusBadRequest, response.CodeBadRequest, "sku_out_of_stock", "error.card_secret_insufficient"),
	channelErrorRule(ErrOrderCurrencyMismatch, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.order_currency_mismatch"),
	channelErrorRule(ErrProductPriceInvalid, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.product_price_invalid"),
	channelErrorRule(couponcontract.ErrInvalid, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_invalid"),
	channelErrorRule(couponcontract.ErrNotFound, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_not_found"),
	channelErrorRule(couponcontract.ErrInactive, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_inactive"),
	channelErrorRule(couponcontract.ErrNotStarted, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_not_started"),
	channelErrorRule(couponcontract.ErrExpired, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_expired"),
	channelErrorRule(couponcontract.ErrUsageLimit, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_usage_limit"),
	channelErrorRule(couponcontract.ErrPerUserLimit, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_per_user_limit"),
	channelErrorRule(couponcontract.ErrMinAmount, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_min_amount"),
	channelErrorRule(couponcontract.ErrScopeInvalid, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_scope_invalid"),
	channelErrorRule(couponcontract.ErrPaymentRoleNotAllowed, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_payment_role_not_allowed"),
	channelErrorRule(couponcontract.ErrPaymentRoleGuestOnly, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_payment_role_guest_only"),
	channelErrorRule(couponcontract.ErrPaymentRoleMemberOnly, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_payment_role_member_only"),
	channelErrorRule(couponcontract.ErrMemberLevelNotAllowed, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_member_level_not_allowed"),
	channelErrorRule(couponcontract.ErrWholesaleDisabled, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.coupon_wholesale_disabled"),
	channelErrorRule(promotioncontract.ErrInvalid, http.StatusBadRequest, response.CodeBadRequest, "coupon_invalid", "error.promotion_invalid"),
	channelErrorRule(ErrManualFormSchemaInvalid, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.manual_form_schema_invalid"),
	channelErrorRule(ErrManualFormRequiredMissing, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.manual_form_required_missing"),
	channelErrorRule(ErrManualFormFieldInvalid, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.manual_form_field_invalid"),
	channelErrorRule(ErrManualFormTypeInvalid, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.manual_form_type_invalid"),
	channelErrorRule(ErrManualFormOptionInvalid, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.manual_form_option_invalid"),
}

var channelPaymentCreateErrorRules = []mappedChannelError{
	channelErrorRule(ErrPaymentInvalid, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.payment_invalid"),
	channelErrorRule(ErrOrderNotFound, http.StatusNotFound, response.CodeNotFound, "order_not_found", "error.order_not_found"),
	channelErrorRule(ErrOrderStatusInvalid, http.StatusBadRequest, response.CodeBadRequest, "order_status_invalid", "error.order_status_invalid"),
	channelErrorRule(ErrPaymentChannelNotFound, http.StatusNotFound, response.CodeNotFound, "payment_method_unavailable", "error.payment_channel_not_found"),
	channelErrorRule(ErrPaymentChannelInactive, http.StatusBadRequest, response.CodeBadRequest, "payment_method_unavailable", "error.payment_channel_inactive"),
	channelErrorRule(ErrPaymentProviderUnsupported, http.StatusBadRequest, response.CodeBadRequest, "payment_method_unavailable", "error.payment_provider_not_supported"),
	channelErrorRule(ErrPaymentChannelConfigInvalid, http.StatusBadRequest, response.CodeBadRequest, "payment_method_unavailable", "error.payment_channel_config_invalid"),
	channelErrorRule(ErrPaymentGatewayRequestFailed, http.StatusBadRequest, response.CodeBadRequest, "payment_create_failed", "error.payment_gateway_request_failed"),
	channelErrorRule(ErrPaymentGatewayResponseInvalid, http.StatusBadRequest, response.CodeBadRequest, "payment_create_failed", "error.payment_gateway_response_invalid"),
	channelErrorRule(ErrPaymentCurrencyMismatch, http.StatusBadRequest, response.CodeBadRequest, "payment_create_failed", "error.payment_currency_mismatch"),
	channelErrorRule(ErrWalletOnlyPaymentRequired, http.StatusBadRequest, response.CodeBadRequest, "wallet_only_payment_required", "error.wallet_only_payment_required"),
}

func respondChannelSuccess(c *gin.Context, data interface{}) {
	response.ChannelSuccess(c, data)
}

func respondChannelError(c *gin.Context, httpCode, code int, errorCode, key string, err error) {
	locale := i18n.ResolveLocale(c)
	message := i18n.T(locale, key)
	if err != nil {
		ginutil.RequestLog(c).Errorw(
			"channel_handler_error",
			"http_code", httpCode,
			"code", code,
			"error_code", errorCode,
			"message", message,
			"error", err,
		)
	}
	response.ChannelError(c, httpCode, code, message, errorCode)
}

func respondChannelBindError(c *gin.Context, err error) {
	locale := i18n.ResolveLocale(c)
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		details := make([]string, 0, len(validationErrors))
		for _, fieldError := range validationErrors {
			details = append(details, formatChannelFieldError(locale, fieldError))
		}
		message := strings.Join(details, "; ")
		ginutil.RequestLog(c).Warnw("channel_bind_validation_error", "details", message, "error", err)
		response.ChannelError(c, http.StatusBadRequest, response.CodeBadRequest, message, "validation_error")
		return
	}

	message := i18n.T(locale, "error.bad_request")
	ginutil.RequestLog(c).Warnw("channel_bind_error", "message", message, "error", err)
	response.ChannelError(c, http.StatusBadRequest, response.CodeBadRequest, message, "validation_error")
}

func respondChannelMappedError(c *gin.Context, err error, rules []mappedChannelError, fallbackHTTPCode, fallbackCode int, fallbackErrorCode, fallbackKey string) {
	if seconds := retryAfter(err); seconds > 0 {
		c.Header("Retry-After", strconv.FormatInt(seconds, 10))
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
	case errors.Is(err, telegramauthapp.ErrTelegramAuthPayloadInvalid):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
	case errors.Is(err, userauthapp.ErrInvalidEmail):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.email_invalid", nil)
	case errors.Is(err, userauthapp.ErrNotFound):
		respondChannelError(c, http.StatusNotFound, response.CodeNotFound, "user_not_found", "error.user_not_found", nil)
	case errors.Is(err, userauthapp.ErrVerifyCodeInvalid):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "verify_code_invalid", "error.verify_code_invalid", nil)
	case errors.Is(err, userauthapp.ErrVerifyCodeExpired):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "verify_code_expired", "error.verify_code_expired", nil)
	case errors.Is(err, userauthapp.ErrVerifyCodeAttemptsExceeded):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "verify_code_invalid", "error.verify_code_attempts_exceeded", nil)
	case errors.Is(err, userauthapp.ErrUserDisabled):
		respondChannelError(c, http.StatusUnauthorized, response.CodeUnauthorized, "user_disabled", "error.user_disabled", nil)
	// Bot 新用户在邀请注册模式下需先提交邀请码（/channel/identities/telegram/provision 的 invite_code）。
	case errors.Is(err, userauthapp.ErrInviteCodeRequired):
		respondChannelError(c, http.StatusForbidden, response.CodeForbidden, "invite_code_required", "error.invite_code_required", nil)
	case errors.Is(err, userauthapp.ErrInviteCodeInvalid):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "invite_code_invalid", "error.invite_code_invalid", nil)
	case errors.Is(err, userauthapp.ErrRegistrationDisabled):
		respondChannelError(c, http.StatusForbidden, response.CodeForbidden, "registration_disabled", "error.registration_disabled", nil)
	case errors.Is(err, userauthapp.ErrUserOAuthIdentityExists):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "channel_identity_conflict", "error.telegram_bind_conflict", nil)
	case errors.Is(err, userauthapp.ErrUserOAuthAlreadyBound):
		respondChannelError(c, http.StatusBadRequest, response.CodeBadRequest, "channel_identity_conflict", "error.telegram_already_bound", nil)
	default:
		respondChannelError(c, http.StatusInternalServerError, response.CodeInternal, "internal_error", "error.internal_error", err)
	}
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

func formatChannelFieldError(locale string, fieldError validator.FieldError) string {
	field := fieldError.Field()
	tag := fieldError.Tag()
	param := fieldError.Param()

	customKey := "validation." + field + "." + tag
	if message := i18n.T(locale, customKey); message != customKey {
		return message
	}

	ruleKey := "validation.rule." + tag
	if ruleMessage := i18n.T(locale, ruleKey); ruleMessage != ruleKey {
		if param != "" {
			return field + ": " + i18n.Sprintf(locale, ruleKey, param)
		}
		return field + ": " + ruleMessage
	}

	if param != "" {
		return field + ": " + tag + "=" + param
	}
	return field + ": " + tag
}
