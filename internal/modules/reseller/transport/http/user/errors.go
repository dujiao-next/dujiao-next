package userhttp

import (
	"errors"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type mappedError struct {
	target error
	code   int
	key    string
}

var userManagementErrorRules = []mappedError{
	{target: resellermodule.ErrNotOpened, code: response.CodeBadRequest, key: "error.bad_request"},
	{target: resellermodule.ErrApplyDisabled, code: response.CodeForbidden, key: "error.forbidden"},
	{target: resellermodule.ErrProfileInactive, code: response.CodeBadRequest, key: "error.forbidden"},
	{target: resellermodule.ErrDomainInvalid, code: response.CodeBadRequest, key: "error.reseller_domain_invalid"},
	{target: resellermodule.ErrDomainMainHostNotAllowed, code: response.CodeBadRequest, key: "error.reseller_domain_main_host_not_allowed"},
	{target: resellermodule.ErrDomainConflict, code: response.CodeBadRequest, key: "error.reseller_domain_conflict"},
	{target: resellermodule.ErrSiteConfigInvalid, code: response.CodeBadRequest, key: "error.reseller_site_config_invalid"},
}

func respondUserManagementError(c *gin.Context, err error, fallbackKey string) {
	for _, rule := range userManagementErrorRules {
		if errors.Is(err, rule.target) {
			ginutil.RespondError(c, rule.code, rule.key, nil)
			return
		}
	}
	ginutil.RespondError(c, response.CodeInternal, fallbackKey, err)
}

func siteConfigFieldErrorKey(field string) string {
	switch field {
	case "support_telegram":
		return "error.reseller_support_telegram_invalid"
	case "support_whatsapp":
		return "error.reseller_support_whatsapp_invalid"
	case "support_email":
		return "error.reseller_support_email_invalid"
	case "support_url":
		return "error.reseller_support_url_invalid"
	case "image":
		return "error.reseller_image_invalid"
	case "link":
		return "error.reseller_link_invalid"
	default:
		return "error.bad_request"
	}
}

type uploadValidationMarker interface {
	UploadValidationError()
}

func isUploadValidationError(err error) bool {
	var marker uploadValidationMarker
	return errors.As(err, &marker)
}

var userProductSettingErrorRules = []mappedError{
	{target: resellermodule.ErrNotOpened, code: response.CodeBadRequest, key: "error.bad_request"},
	{target: resellermodule.ErrProfileInactive, code: response.CodeBadRequest, key: "error.forbidden"},
	{target: productcontract.ErrProductSKUInvalid, code: response.CodeBadRequest, key: "error.order_item_invalid"},
	{target: resellermodule.ErrPriceBelowBase, code: response.CodeBadRequest, key: "error.reseller_price_invalid"},
	{target: resellermodule.ErrPriceBelowMasterPrice, code: response.CodeBadRequest, key: "error.reseller_price_below_master"},
	{target: resellermodule.ErrMarkupExceeded, code: response.CodeBadRequest, key: "error.reseller_markup_exceeded"},
	{target: resellermodule.ErrPricingModeInvalid, code: response.CodeBadRequest, key: "error.reseller_price_invalid"},
}

func respondUserProductSettingError(c *gin.Context, err error, fallbackKey string) {
	for _, rule := range userProductSettingErrorRules {
		if errors.Is(err, rule.target) {
			ginutil.RespondError(c, rule.code, rule.key, nil)
			return
		}
	}
	if errors.Is(err, productcontract.ErrNotFound) {
		ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
		return
	}
	ginutil.RespondError(c, response.CodeInternal, fallbackKey, err)
}

var userFinanceErrorRules = []mappedError{
	{target: resellermodule.ErrNotOpened, code: response.CodeBadRequest, key: "error.bad_request"},
	{target: resellermodule.ErrProfileInactive, code: response.CodeBadRequest, key: "error.reseller_profile_inactive"},
	{target: resellermodule.ErrSettlementUnavailable, code: response.CodeBadRequest, key: "error.reseller_settlement_unavailable"},
	{target: resellermodule.ErrWithdrawAmountInvalid, code: response.CodeBadRequest, key: "error.reseller_withdraw_amount_invalid"},
	{target: resellermodule.ErrWithdrawCurrencyUnavailable, code: response.CodeBadRequest, key: "error.reseller_withdraw_currency_unavailable"},
	{target: resellermodule.ErrWithdrawInsufficient, code: response.CodeBadRequest, key: "error.reseller_withdraw_insufficient"},
	{target: resellermodule.ErrBalanceAccountFrozen, code: response.CodeBadRequest, key: "error.reseller_balance_frozen"},
}

func respondUserFinanceError(c *gin.Context, err error, fallbackKey string) {
	for _, rule := range userFinanceErrorRules {
		if errors.Is(err, rule.target) {
			ginutil.RespondError(c, rule.code, rule.key, nil)
			return
		}
	}
	ginutil.RespondError(c, response.CodeInternal, fallbackKey, err)
}

var userOrderErrorRules = []mappedError{
	{target: resellermodule.ErrNotOpened, code: response.CodeBadRequest, key: "error.bad_request"},
	{target: resellermodule.ErrProfileInactive, code: response.CodeBadRequest, key: "error.forbidden"},
	{target: resellermodule.ErrOrderNotFound, code: response.CodeNotFound, key: "error.order_not_found"},
}

func respondUserOrderError(c *gin.Context, err error, fallbackKey string) {
	for _, rule := range userOrderErrorRules {
		if errors.Is(err, rule.target) {
			ginutil.RespondError(c, rule.code, rule.key, nil)
			return
		}
	}
	ginutil.RespondError(c, response.CodeInternal, fallbackKey, err)
}
