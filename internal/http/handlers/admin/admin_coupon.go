package admin

import (
	"errors"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// CreateCouponRequest 创建优惠券请求
type CreateCouponRequest struct {
	Code                   string   `json:"code" binding:"required"`
	Type                   string   `json:"type" binding:"required"`
	Value                  float64  `json:"value" binding:"required"`
	MinAmount              float64  `json:"min_amount"`
	MaxDiscount            float64  `json:"max_discount"`
	UsageLimit             int      `json:"usage_limit"`
	PerUserLimit           int      `json:"per_user_limit"`
	DisabledWholesalePrice *bool    `json:"disabled_wholesale_price"`
	PerItemDiscount        *bool    `json:"per_item_discount"`
	PaymentRoles           []string `json:"payment_roles"`
	MemberLevels           []uint   `json:"member_levels"`
	ScopeRefIDs            []uint   `json:"scope_ref_ids" binding:"required"`
	StartsAt               string   `json:"starts_at"`
	EndsAt                 string   `json:"ends_at"`
	IsActive               *bool    `json:"is_active"`
}

func buildCreateCouponInputFromRequest(req CreateCouponRequest) (service.CreateCouponInput, error) {
	startsAt, err := shared.ParseTimeNullable(req.StartsAt)
	if err != nil {
		return service.CreateCouponInput{}, err
	}
	endsAt, err := shared.ParseTimeNullable(req.EndsAt)
	if err != nil {
		return service.CreateCouponInput{}, err
	}
	return service.CreateCouponInput{
		Code:                   req.Code,
		Type:                   req.Type,
		Value:                  models.NewMoneyFromDecimal(decimal.NewFromFloat(req.Value)),
		MinAmount:              models.NewMoneyFromDecimal(decimal.NewFromFloat(req.MinAmount)),
		MaxDiscount:            models.NewMoneyFromDecimal(decimal.NewFromFloat(req.MaxDiscount)),
		UsageLimit:             req.UsageLimit,
		PerUserLimit:           req.PerUserLimit,
		DisabledWholesalePrice: req.DisabledWholesalePrice,
		PerItemDiscount:        req.PerItemDiscount,
		PaymentRoles:           req.PaymentRoles,
		MemberLevels:           req.MemberLevels,
		ScopeRefIDs:            req.ScopeRefIDs,
		StartsAt:               startsAt,
		EndsAt:                 endsAt,
		IsActive:               req.IsActive,
	}, nil
}

func buildUpdateCouponInputFromRequest(req CreateCouponRequest) (service.UpdateCouponInput, error) {
	input, err := buildCreateCouponInputFromRequest(req)
	if err != nil {
		return service.UpdateCouponInput{}, err
	}
	return service.UpdateCouponInput(input), nil
}

// CreateCoupon 创建优惠券
func (h *Handler) CreateCoupon(c *gin.Context) {
	var req CreateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	input, err := buildCreateCouponInputFromRequest(req)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	coupon, err := h.CouponAdminService.Create(input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCouponInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.coupon_invalid", nil)
		case errors.Is(err, service.ErrCouponScopeInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.coupon_scope_invalid", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.coupon_create_failed", err)
		}
		return
	}

	response.Success(c, coupon)
}

// UpdateCoupon 更新优惠券
func (h *Handler) UpdateCoupon(c *gin.Context) {
	couponID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var req CreateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	input, err := buildUpdateCouponInputFromRequest(req)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	coupon, err := h.CouponAdminService.Update(couponID, input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCouponNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.coupon_not_found", nil)
		case errors.Is(err, service.ErrCouponInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.coupon_invalid", nil)
		case errors.Is(err, service.ErrCouponScopeInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.coupon_scope_invalid", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.coupon_update_failed", err)
		}
		return
	}

	response.Success(c, coupon)
}

// DeleteCoupon 删除优惠券
func (h *Handler) DeleteCoupon(c *gin.Context) {
	couponID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.CouponAdminService.Delete(couponID); err != nil {
		switch {
		case errors.Is(err, service.ErrCouponNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.coupon_not_found", nil)
		case errors.Is(err, service.ErrCouponInvalid):
			shared.RespondError(c, response.CodeBadRequest, "error.coupon_invalid", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.coupon_delete_failed", err)
		}
		return
	}
	response.Success(c, gin.H{
		"deleted": true,
	})
}

// GetAdminCoupons 获取优惠券列表
func (h *Handler) GetAdminCoupons(c *gin.Context) {
	page, pageSize := shared.ParsePagination(c)

	code := c.Query("code")
	id, err := shared.ParseQueryUint(c.Query("id"), true)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	scopeRefID, err := shared.ParseQueryUint(c.Query("scope_ref_id"), true)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	isActive, err := shared.ParseQueryBoolPtr(c, "is_active")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	coupons, total, err := h.CouponAdminService.List(repository.CouponListFilter{
		ID:         id,
		Code:       code,
		ScopeRefID: scopeRefID,
		IsActive:   isActive,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.coupon_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, coupons, pagination)
}
