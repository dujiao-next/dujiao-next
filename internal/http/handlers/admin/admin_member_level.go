package admin

import (
	"errors"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// CreateMemberLevelRequest 创建/更新会员等级请求
type CreateMemberLevelRequest struct {
	NameJSON          models.JSON `json:"name" binding:"required"`
	Slug              string      `json:"slug" binding:"required"`
	Icon              string      `json:"icon"`
	DiscountRate      float64     `json:"discount_rate"`
	RechargeThreshold float64     `json:"recharge_threshold"`
	SpendThreshold    float64     `json:"spend_threshold"`
	IsDefault         bool        `json:"is_default"`
	SortOrder         int         `json:"sort_order"`
	IsActive          *bool       `json:"is_active"`
}

// GetAdminMemberLevels 获取会员等级列表
func (h *Handler) GetAdminMemberLevels(c *gin.Context) {
	page, pageSize := shared.ParsePaginationWithKeys(c, "page", "page_size", 50)

	isActive, err := shared.ParseQueryBoolPtr(c, "is_active")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	levels, total, err := h.MemberLevelService.ListLevels(repository.MemberLevelListFilter{
		IsActive: isActive,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.member_level_fetch_failed", err)
		return
	}

	// 批量统计每个等级的会员人数，查询失败不报错（member_count 保持默认值 0），仅记录日志
	if len(levels) > 0 {
		ids := make([]uint, len(levels))
		for i, lv := range levels {
			ids[i] = lv.ID
		}
		if counts, err := h.UserRepo.CountByMemberLevelIDs(ids); err == nil {
			for i := range levels {
				levels[i].MemberCount = counts[levels[i].ID]
			}
		} else {
			logger.Warnw("member_level_count_failed", "error", err, "level_ids", ids)
		}
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, levels, pagination)
}

// CreateMemberLevel 创建会员等级
func (h *Handler) CreateMemberLevel(c *gin.Context) {
	var req CreateMemberLevelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	level := &models.MemberLevel{
		NameJSON:          req.NameJSON,
		Slug:              req.Slug,
		Icon:              req.Icon,
		DiscountRate:      models.NewMoneyFromDecimal(decimal.NewFromFloat(req.DiscountRate)),
		RechargeThreshold: models.NewMoneyFromDecimal(decimal.NewFromFloat(req.RechargeThreshold)),
		SpendThreshold:    models.NewMoneyFromDecimal(decimal.NewFromFloat(req.SpendThreshold)),
		IsDefault:         req.IsDefault,
		SortOrder:         req.SortOrder,
		IsActive:          isActive,
	}

	if err := h.MemberLevelService.CreateLevel(level); err != nil {
		switch {
		case errors.Is(err, service.ErrMemberLevelSlugExists):
			shared.RespondError(c, response.CodeBadRequest, "error.member_level_slug_exists", nil)
		case errors.Is(err, service.ErrMemberLevelSortOrderUsed):
			shared.RespondError(c, response.CodeBadRequest, "error.member_level_sort_order_used", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.member_level_create_failed", err)
		}
		return
	}

	response.Success(c, level)
}

// UpdateMemberLevel 更新会员等级
func (h *Handler) UpdateMemberLevel(c *gin.Context) {
	levelID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	existing, err := h.MemberLevelService.GetByID(levelID)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.member_level_fetch_failed", err)
		return
	}
	if existing == nil {
		shared.RespondError(c, response.CodeNotFound, "error.member_level_not_found", nil)
		return
	}

	var req CreateMemberLevelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	existing.NameJSON = req.NameJSON
	existing.Slug = req.Slug
	existing.Icon = req.Icon
	existing.DiscountRate = models.NewMoneyFromDecimal(decimal.NewFromFloat(req.DiscountRate))
	existing.RechargeThreshold = models.NewMoneyFromDecimal(decimal.NewFromFloat(req.RechargeThreshold))
	existing.SpendThreshold = models.NewMoneyFromDecimal(decimal.NewFromFloat(req.SpendThreshold))
	existing.IsDefault = req.IsDefault
	existing.SortOrder = req.SortOrder
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	if err := h.MemberLevelService.UpdateLevel(existing); err != nil {
		switch {
		case errors.Is(err, service.ErrMemberLevelSlugExists):
			shared.RespondError(c, response.CodeBadRequest, "error.member_level_slug_exists", nil)
		case errors.Is(err, service.ErrMemberLevelSortOrderUsed):
			shared.RespondError(c, response.CodeBadRequest, "error.member_level_sort_order_used", nil)
		case errors.Is(err, service.ErrMemberLevelNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.member_level_not_found", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.member_level_update_failed", err)
		}
		return
	}

	response.Success(c, existing)
}

// DeleteMemberLevel 删除会员等级
func (h *Handler) DeleteMemberLevel(c *gin.Context) {
	levelID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	if err := h.MemberLevelService.DeleteLevel(levelID); err != nil {
		switch {
		case errors.Is(err, service.ErrMemberLevelNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.member_level_not_found", nil)
		case errors.Is(err, service.ErrMemberLevelDeleteDefault):
			shared.RespondError(c, response.CodeBadRequest, "error.member_level_cannot_delete_default", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.member_level_delete_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

// GetMemberLevelPrices 获取商品的等级价列表
func (h *Handler) GetMemberLevelPrices(c *gin.Context) {
	productID, err := shared.ParseQueryUint(c.Query("product_id"), false)
	if err != nil || productID == 0 {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	prices, err := h.MemberLevelService.GetLevelPricesByProduct(productID)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.member_level_price_fetch_failed", err)
		return
	}

	response.Success(c, prices)
}

// BatchUpsertMemberLevelPricesRequest 批量设置等级价请求
type BatchUpsertMemberLevelPricesRequest struct {
	Prices []MemberLevelPriceInput `json:"prices" binding:"required"`
}

// MemberLevelPriceInput 等级价输入
type MemberLevelPriceInput struct {
	MemberLevelID uint    `json:"member_level_id" binding:"required"`
	ProductID     uint    `json:"product_id" binding:"required"`
	SKUID         uint    `json:"sku_id"`
	PriceAmount   float64 `json:"price_amount" binding:"required"`
}

// BatchUpsertMemberLevelPrices 批量设置等级价
func (h *Handler) BatchUpsertMemberLevelPrices(c *gin.Context) {
	var req BatchUpsertMemberLevelPricesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	prices := make([]models.MemberLevelPrice, 0, len(req.Prices))
	for _, p := range req.Prices {
		prices = append(prices, models.MemberLevelPrice{
			MemberLevelID: p.MemberLevelID,
			ProductID:     p.ProductID,
			SKUID:         p.SKUID,
			PriceAmount:   models.NewMoneyFromDecimal(decimal.NewFromFloat(p.PriceAmount)),
		})
	}

	if err := h.MemberLevelService.BatchUpsertLevelPrices(prices); err != nil {
		shared.RespondError(c, response.CodeInternal, "error.member_level_price_save_failed", err)
		return
	}

	response.Success(c, gin.H{"saved": true})
}

// DeleteMemberLevelPrice 删除等级价
func (h *Handler) DeleteMemberLevelPrice(c *gin.Context) {
	priceID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	if err := h.MemberLevelService.DeleteLevelPrice(priceID); err != nil {
		shared.RespondError(c, response.CodeInternal, "error.member_level_price_delete_failed", err)
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

// SetUserMemberLevelRequest 手动设置用户等级请求
type SetUserMemberLevelRequest struct {
	MemberLevelID uint `json:"member_level_id"`
}

// SetUserMemberLevel 手动设置用户等级
func (h *Handler) SetUserMemberLevel(c *gin.Context) {
	userID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	var req SetUserMemberLevelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	if err := h.MemberLevelService.SetUserLevel(userID, req.MemberLevelID); err != nil {
		switch {
		case errors.Is(err, service.ErrMemberLevelNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.member_level_not_found", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.user_member_level_update_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"updated": true})
}

// BackfillMemberLevels POST /admin/member-levels/backfill — 为所有未分配等级的老用户批量分配默认等级
func (h *Handler) BackfillMemberLevels(c *gin.Context) {
	affected, err := h.MemberLevelService.BackfillDefaultLevel()
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMemberLevelNotFound):
			shared.RespondError(c, response.CodeBadRequest, "error.member_level_no_default", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.member_level_backfill_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"affected": affected})
}
