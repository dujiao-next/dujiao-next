package memberlevelhttp

import (
	"errors"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type AdminService interface {
	ListLevels(filter memberlevelcontract.ListFilter) ([]memberleveldomain.MemberLevel, int64, error)
	CreateLevel(level *memberleveldomain.MemberLevel) error
	GetByID(id uint) (*memberleveldomain.MemberLevel, error)
	UpdateLevel(level *memberleveldomain.MemberLevel) error
	DeleteLevel(id uint) error
	GetLevelPricesByProduct(productID uint) ([]memberleveldomain.MemberLevelPrice, error)
	BatchUpsertLevelPrices(prices []memberleveldomain.MemberLevelPrice) error
	DeleteLevelPrice(id uint) error
	ListUserPrices(userID uint, page, pageSize int) ([]memberleveldomain.UserProductPrice, int64, error)
	BatchUpsertUserPrices(prices []memberleveldomain.UserProductPrice) error
	DeleteUserPrice(id uint) error
	SetUserLevel(userID, levelID uint) error
	BackfillDefaultLevel() (int64, error)
}

type AdminHandler struct {
	service AdminService
}

func NewAdminHandler(service AdminService) *AdminHandler {
	return &AdminHandler{service: service}
}

// CreateMemberLevelRequest 创建/更新会员等级请求
type CreateMemberLevelRequest struct {
	NameJSON          jsonmap.JSON `json:"name" binding:"required"`
	Slug              string       `json:"slug" binding:"required"`
	Icon              string       `json:"icon"`
	DiscountRate      float64      `json:"discount_rate"`
	RechargeThreshold float64      `json:"recharge_threshold"`
	SpendThreshold    float64      `json:"spend_threshold"`
	IsDefault         bool         `json:"is_default"`
	SortOrder         int          `json:"sort_order"`
	IsActive          *bool        `json:"is_active"`
}

// GetAdminMemberLevels 获取会员等级列表
func (h *AdminHandler) GetAdminMemberLevels(c *gin.Context) {
	page, pageSize := ginutil.ParsePaginationWithKeys(c, "page", "page_size", 50)

	isActive, err := ginutil.ParseQueryBoolPtr(c, "is_active")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	levels, total, err := h.service.ListLevels(memberlevelcontract.ListFilter{
		IsActive: isActive,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.member_level_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, levels, pagination)
}

// CreateMemberLevel 创建会员等级
func (h *AdminHandler) CreateMemberLevel(c *gin.Context) {
	var req CreateMemberLevelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	level := &memberleveldomain.MemberLevel{
		NameJSON:          req.NameJSON,
		Slug:              req.Slug,
		Icon:              req.Icon,
		DiscountRate:      money.FromDecimal(decimal.NewFromFloat(req.DiscountRate)),
		RechargeThreshold: money.FromDecimal(decimal.NewFromFloat(req.RechargeThreshold)),
		SpendThreshold:    money.FromDecimal(decimal.NewFromFloat(req.SpendThreshold)),
		IsDefault:         req.IsDefault,
		SortOrder:         req.SortOrder,
		IsActive:          isActive,
	}

	if err := h.service.CreateLevel(level); err != nil {
		switch {
		case errors.Is(err, memberlevelcontract.ErrSlugExists):
			ginutil.RespondError(c, response.CodeBadRequest, "error.member_level_slug_exists", nil)
		case errors.Is(err, memberlevelcontract.ErrSortOrderUsed):
			ginutil.RespondError(c, response.CodeBadRequest, "error.member_level_sort_order_used", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.member_level_create_failed", err)
		}
		return
	}

	response.Success(c, level)
}

// UpdateMemberLevel 更新会员等级
func (h *AdminHandler) UpdateMemberLevel(c *gin.Context) {
	levelID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	existing, err := h.service.GetByID(levelID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.member_level_fetch_failed", err)
		return
	}
	if existing == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.member_level_not_found", nil)
		return
	}

	var req CreateMemberLevelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	existing.NameJSON = req.NameJSON
	existing.Slug = req.Slug
	existing.Icon = req.Icon
	existing.DiscountRate = money.FromDecimal(decimal.NewFromFloat(req.DiscountRate))
	existing.RechargeThreshold = money.FromDecimal(decimal.NewFromFloat(req.RechargeThreshold))
	existing.SpendThreshold = money.FromDecimal(decimal.NewFromFloat(req.SpendThreshold))
	existing.IsDefault = req.IsDefault
	existing.SortOrder = req.SortOrder
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	if err := h.service.UpdateLevel(existing); err != nil {
		switch {
		case errors.Is(err, memberlevelcontract.ErrSlugExists):
			ginutil.RespondError(c, response.CodeBadRequest, "error.member_level_slug_exists", nil)
		case errors.Is(err, memberlevelcontract.ErrSortOrderUsed):
			ginutil.RespondError(c, response.CodeBadRequest, "error.member_level_sort_order_used", nil)
		case errors.Is(err, memberlevelcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.member_level_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.member_level_update_failed", err)
		}
		return
	}

	response.Success(c, existing)
}

// DeleteMemberLevel 删除会员等级
func (h *AdminHandler) DeleteMemberLevel(c *gin.Context) {
	levelID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	if err := h.service.DeleteLevel(levelID); err != nil {
		switch {
		case errors.Is(err, memberlevelcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.member_level_not_found", nil)
		case errors.Is(err, memberlevelcontract.ErrDeleteDefault):
			ginutil.RespondError(c, response.CodeBadRequest, "error.member_level_cannot_delete_default", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.member_level_delete_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

// GetMemberLevelPrices 获取商品的等级价列表
func (h *AdminHandler) GetMemberLevelPrices(c *gin.Context) {
	productID, err := ginutil.ParseQueryUint(c.Query("product_id"), false)
	if err != nil || productID == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	prices, err := h.service.GetLevelPricesByProduct(productID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.member_level_price_fetch_failed", err)
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
func (h *AdminHandler) BatchUpsertMemberLevelPrices(c *gin.Context) {
	var req BatchUpsertMemberLevelPricesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	prices := make([]memberleveldomain.MemberLevelPrice, 0, len(req.Prices))
	for _, p := range req.Prices {
		prices = append(prices, memberleveldomain.MemberLevelPrice{
			MemberLevelID: p.MemberLevelID,
			ProductID:     p.ProductID,
			SKUID:         p.SKUID,
			PriceAmount:   money.FromDecimal(decimal.NewFromFloat(p.PriceAmount)),
		})
	}

	if err := h.service.BatchUpsertLevelPrices(prices); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.member_level_price_save_failed", err)
		return
	}

	response.Success(c, gin.H{"saved": true})
}

// DeleteMemberLevelPrice 删除等级价
func (h *AdminHandler) DeleteMemberLevelPrice(c *gin.Context) {
	priceID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	if err := h.service.DeleteLevelPrice(priceID); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.member_level_price_delete_failed", err)
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

// SetUserMemberLevelRequest 手动设置用户等级请求
type SetUserMemberLevelRequest struct {
	MemberLevelID uint `json:"member_level_id"`
}

// SetUserMemberLevel 手动设置用户等级
func (h *AdminHandler) SetUserMemberLevel(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	var req SetUserMemberLevelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.service.SetUserLevel(userID, req.MemberLevelID); err != nil {
		switch {
		case errors.Is(err, memberlevelcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.member_level_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_member_level_update_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"updated": true})
}

// BackfillMemberLevels POST /admin/member-levels/backfill — 为所有未分配等级的老用户批量分配默认等级
func (h *AdminHandler) BackfillMemberLevels(c *gin.Context) {
	affected, err := h.service.BackfillDefaultLevel()
	if err != nil {
		switch {
		case errors.Is(err, memberlevelcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeBadRequest, "error.member_level_no_default", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.member_level_backfill_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"affected": affected})
}
