package adminhttp

import (
	"errors"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

func respondAdminManagementError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, productcontract.ErrNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.bad_request", nil)
	case errors.Is(err, resellercontract.ErrProfileStatusInvalid),
		errors.Is(err, resellercontract.ErrDomainStatusInvalid),
		errors.Is(err, resellercontract.ErrDomainInvalid),
		errors.Is(err, resellercontract.ErrSiteConfigInvalid),
		errors.Is(err, resellercontract.ErrDomainMainHostNotAllowed),
		errors.Is(err, resellercontract.ErrDomainConflict):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
	case errors.Is(err, resellercontract.ErrSubdomainBaseMissing):
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_subdomain_base_missing", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
	}
}

func respondAdminProductSettingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, productcontract.ErrNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
	case errors.Is(err, resellercontract.ErrProfileInactive):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
	case errors.Is(err, productcontract.ErrProductSKUInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
	case errors.Is(err, resellercontract.ErrPriceBelowBase),
		errors.Is(err, resellercontract.ErrPricingModeInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_price_invalid", nil)
	case errors.Is(err, resellercontract.ErrPriceBelowMasterPrice):
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_price_below_master", nil)
	case errors.Is(err, resellercontract.ErrMarkupExceeded):
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_markup_exceeded", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
	}
}
