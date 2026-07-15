package admin

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
)

// UpdateAdminUserRequest 管理员更新用户请求
type UpdateAdminUserRequest struct {
	Nickname      *string `json:"nickname"`
	Locale        *string `json:"locale"`
	Status        *string `json:"status"`
	Email         *string `json:"email"`
	Password      *string `json:"password"`
	AdminNote     *string `json:"admin_note"`
	EmailVerified *bool   `json:"email_verified"`
	MemberLevelID *uint   `json:"member_level_id"` // 会员等级 ID，nil=不修改，0=清除等级
}

// BatchUpdateUserStatusRequest 批量更新用户状态请求
type BatchUpdateUserStatusRequest struct {
	UserIDs []uint `json:"user_ids" binding:"required"`
	Status  string `json:"status" binding:"required"`
}

// UserCouponUsageProduct 优惠券适用商品
type UserCouponUsageProduct struct {
	ID    uint        `json:"id"`
	Title models.JSON `json:"title"`
}

// UserCouponUsageItem 用户优惠券使用记录返回
type UserCouponUsageItem struct {
	ID             uint                     `json:"id"`
	CouponID       uint                     `json:"coupon_id"`
	CouponCode     string                   `json:"coupon_code"`
	CouponType     string                   `json:"coupon_type"`
	OrderID        uint                     `json:"order_id"`
	DiscountAmount models.Money             `json:"discount_amount"`
	CreatedAt      time.Time                `json:"created_at"`
	ScopeRefIDs    []uint                   `json:"scope_ref_ids"`
	ScopeProducts  []UserCouponUsageProduct `json:"scope_products"`
}

// AdminUserListItem 管理端用户列表项
type AdminUserListItem struct {
	models.User
	WalletBalance models.Money `json:"wallet_balance"`
}

// AdminUserOAuthIdentityItem 管理端用户第三方身份项
type AdminUserOAuthIdentityItem struct {
	ID             uint       `json:"id"`
	Provider       string     `json:"provider"`
	ProviderUserID string     `json:"provider_user_id"`
	Username       string     `json:"username"`
	AvatarURL      string     `json:"avatar_url"`
	AuthAt         *time.Time `json:"auth_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// AdminUserDetail 管理端用户详情
type AdminUserDetail struct {
	models.User
	WalletBalance   models.Money                 `json:"wallet_balance"`
	OAuthIdentities []AdminUserOAuthIdentityItem `json:"oauth_identities"`
}

// GetAdminUsers 获取用户列表
func (h *Handler) GetAdminUsers(c *gin.Context) {
	page, pageSize := shared.ParsePagination(c)

	userID, err := shared.ParseQueryUint(c.Query("user_id"), true)
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", err)
		return
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))

	var memberLevelID *uint
	if raw := strings.TrimSpace(c.Query("member_level_id")); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		v := uint(parsed)
		memberLevelID = &v
	}

	createdFrom, createdTo, err := shared.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	lastLoginFrom, lastLoginTo, err := shared.ParseQueryTimeRange(c, "last_login_from", "last_login_to")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	users, total, err := h.UserRepo.List(repository.UserListFilter{
		Page:          page,
		PageSize:      pageSize,
		UserID:        userID,
		Keyword:       keyword,
		Status:        status,
		MemberLevelID: memberLevelID,
		CreatedFrom:   createdFrom,
		CreatedTo:     createdTo,
		LastLoginFrom: lastLoginFrom,
		LastLoginTo:   lastLoginTo,
		SortBy:        strings.TrimSpace(c.Query("sort_by")),
		SortOrder:     strings.TrimSpace(c.Query("sort_order")),
	})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}

	userIDs := make([]uint, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	balanceMap, err := h.WalletService.GetBalancesByUserIDs(userIDs)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	items := make([]AdminUserListItem, 0, len(users))
	for _, user := range users {
		balance, ok := balanceMap[user.ID]
		if !ok {
			balance = models.NewMoneyFromDecimal(decimal.Zero)
		}
		items = append(items, AdminUserListItem{
			User:          user,
			WalletBalance: balance,
		})
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, items, pagination)
}

// GetAdminUser 获取用户详情
func (h *Handler) GetAdminUser(c *gin.Context) {
	userID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}

	user, err := h.UserRepo.GetByID(userID)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if user == nil {
		shared.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		return
	}
	account, err := h.WalletService.GetAccount(user.ID)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	identities, err := h.UserOAuthIdentityRepo.ListByUserID(user.ID)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	oauthItems := make([]AdminUserOAuthIdentityItem, 0, len(identities))
	for _, identity := range identities {
		oauthItems = append(oauthItems, AdminUserOAuthIdentityItem{
			ID:             identity.ID,
			Provider:       identity.Provider,
			ProviderUserID: identity.ProviderUserID,
			Username:       identity.Username,
			AvatarURL:      identity.AvatarURL,
			AuthAt:         identity.AuthAt,
			CreatedAt:      identity.CreatedAt,
		})
	}
	response.Success(c, AdminUserDetail{
		User:            *user,
		WalletBalance:   account.Balance,
		OAuthIdentities: oauthItems,
	})
}

// UpdateAdminUser 更新用户信息
func (h *Handler) UpdateAdminUser(c *gin.Context) {
	userID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}

	var req UpdateAdminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	user, err := h.UserRepo.GetByID(userID)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if user == nil {
		shared.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		return
	}

	updated := false
	revokeToken := false
	if req.Email != nil {
		normalized, err := service.NormalizeEmail(*req.Email)
		if err != nil {
			shared.RespondError(c, response.CodeBadRequest, "error.email_invalid", nil)
			return
		}
		existing, err := h.UserRepo.GetByEmail(normalized)
		if err != nil {
			shared.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
			return
		}
		if existing != nil && existing.ID != user.ID {
			shared.RespondError(c, response.CodeBadRequest, "error.email_exists", nil)
			return
		}
		if normalized != user.Email {
			user.Email = normalized
			updated = true
		}
	}
	if req.Nickname != nil {
		trimmed := strings.TrimSpace(*req.Nickname)
		if trimmed != "" {
			user.DisplayName = trimmed
			updated = true
		}
	}
	if req.Password != nil {
		trimmed := strings.TrimSpace(*req.Password)
		if trimmed != "" {
			hashed, err := bcrypt.GenerateFromPassword([]byte(trimmed), bcrypt.DefaultCost)
			if err != nil {
				shared.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
				return
			}
			user.PasswordHash = string(hashed)
			updated = true
			revokeToken = true
		}
	}
	if req.Locale != nil {
		trimmed := strings.TrimSpace(*req.Locale)
		if trimmed != "" {
			user.Locale = trimmed
			updated = true
		}
	}
	if req.Status != nil {
		trimmed := strings.ToLower(strings.TrimSpace(*req.Status))
		if trimmed == constants.UserStatusActive || trimmed == constants.UserStatusDisabled {
			if user.Status != trimmed {
				user.Status = trimmed
				updated = true
			}
			if trimmed == constants.UserStatusDisabled {
				revokeToken = true
			}
		}
	}

	if req.AdminNote != nil {
		user.AdminNote = *req.AdminNote
		updated = true
	}

	if req.MemberLevelID != nil {
		if *req.MemberLevelID != 0 {
			lv, err := h.MemberLevelService.GetByID(*req.MemberLevelID)
			if err != nil {
				shared.RespondError(c, response.CodeInternal, "error.member_level_fetch_failed", err)
				return
			}
			if lv == nil {
				shared.RespondError(c, response.CodeBadRequest, "error.member_level_not_found", nil)
				return
			}
		}
		user.MemberLevelID = *req.MemberLevelID
		updated = true
	}
	if req.EmailVerified != nil {
		if *req.EmailVerified {
			if user.EmailVerifiedAt == nil {
				now := time.Now()
				user.EmailVerifiedAt = &now
				updated = true
			}
		} else if user.EmailVerifiedAt != nil {
			user.EmailVerifiedAt = nil
			updated = true
		}
	}

	if !updated {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	now := time.Now()
	user.UpdatedAt = now
	if revokeToken {
		user.TokenVersion++
		user.TokenInvalidBefore = &now
	}
	if err := h.UserRepo.Update(user); err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		return
	}
	_ = cache.SetUserAuthState(c.Request.Context(), cache.BuildUserAuthState(user))

	response.Success(c, user)
}

// UnbindAdminUserTelegram 管理员解除目标用户的 Telegram 绑定。
// DELETE /admin/users/:id/oauth/telegram
func (h *Handler) UnbindAdminUserTelegram(c *gin.Context) {
	userID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}

	if err := h.UserAuthService.UnbindTelegram(userID); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			shared.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		case errors.Is(err, service.ErrUserDisabled):
			shared.RespondError(c, response.CodeBadRequest, "error.user_disabled", nil)
		case errors.Is(err, service.ErrUserOAuthNotBound):
			shared.RespondError(c, response.CodeBadRequest, "error.telegram_not_bound", nil)
		case errors.Is(err, service.ErrTelegramUnbindRequiresEmail):
			shared.RespondError(c, response.CodeBadRequest, "error.telegram_unbind_requires_email", nil)
		default:
			shared.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"unbound": true})
}

// GetAdminUserCouponUsages 获取用户优惠券使用记录
func (h *Handler) GetAdminUserCouponUsages(c *gin.Context) {
	userID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}

	page, pageSize := shared.ParsePagination(c)

	usages, total, err := h.CouponUsageRepo.ListByUser(repository.CouponUsageListFilter{
		Page:     page,
		PageSize: pageSize,
		UserID:   userID,
	})
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}

	couponIDs := make([]uint, 0, len(usages))
	couponIDSet := make(map[uint]struct{})
	for _, usage := range usages {
		if usage.CouponID == 0 {
			continue
		}
		if _, ok := couponIDSet[usage.CouponID]; !ok {
			couponIDSet[usage.CouponID] = struct{}{}
			couponIDs = append(couponIDs, usage.CouponID)
		}
	}

	coupons := make(map[uint]*models.Coupon)
	if len(couponIDs) > 0 {
		items, err := h.CouponRepo.ListByIDs(couponIDs)
		if err != nil {
			shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
			return
		}
		for i := range items {
			coupon := items[i]
			coupons[coupon.ID] = &coupon
		}
	}

	productIDs := make(map[uint]struct{})
	couponScopeIDs := make(map[uint][]uint)
	for _, coupon := range coupons {
		scopeIDs := decodeScopeRefIDs(coupon.ScopeRefIDs)
		couponScopeIDs[coupon.ID] = scopeIDs
		for _, pid := range scopeIDs {
			productIDs[pid] = struct{}{}
		}
	}

	scopeProducts := make(map[uint]UserCouponUsageProduct)
	if len(productIDs) > 0 {
		ids := make([]uint, 0, len(productIDs))
		for id := range productIDs {
			ids = append(ids, id)
		}
		products, err := h.ProductRepo.ListByIDs(ids)
		if err != nil {
			shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
			return
		}
		for i := range products {
			product := products[i]
			scopeProducts[product.ID] = UserCouponUsageProduct{
				ID:    product.ID,
				Title: product.TitleJSON,
			}
		}
	}

	result := make([]UserCouponUsageItem, 0, len(usages))
	for _, usage := range usages {
		item := UserCouponUsageItem{
			ID:             usage.ID,
			CouponID:       usage.CouponID,
			OrderID:        usage.OrderID,
			DiscountAmount: usage.DiscountAmount,
			CreatedAt:      usage.CreatedAt,
		}
		if coupon, ok := coupons[usage.CouponID]; ok {
			item.CouponCode = coupon.Code
			item.CouponType = coupon.Type
			scopeIDs := couponScopeIDs[coupon.ID]
			item.ScopeRefIDs = scopeIDs
			if len(scopeIDs) > 0 {
				products := make([]UserCouponUsageProduct, 0, len(scopeIDs))
				for _, pid := range scopeIDs {
					if prod, ok := scopeProducts[pid]; ok {
						products = append(products, prod)
					}
				}
				item.ScopeProducts = products
			}
		}
		result = append(result, item)
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, result, pagination)
}

func decodeScopeRefIDs(raw string) []uint {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}

// BatchUpdateUserStatus 批量更新用户状态
func (h *Handler) BatchUpdateUserStatus(c *gin.Context) {
	var req BatchUpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	if len(req.UserIDs) == 0 {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	normalizedStatus := strings.ToLower(strings.TrimSpace(req.Status))
	if normalizedStatus != constants.UserStatusActive && normalizedStatus != constants.UserStatusDisabled {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	if err := h.UserRepo.BatchUpdateStatus(req.UserIDs, normalizedStatus); err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		return
	}
	for _, userID := range req.UserIDs {
		_ = cache.DelUserAuthState(c.Request.Context(), userID)
	}

	response.Success(c, gin.H{"updated": len(req.UserIDs)})
}
