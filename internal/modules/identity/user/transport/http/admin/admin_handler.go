package adminuserhttp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNotFound                    = errors.New("not found")
	ErrUserDisabled                = errors.New("user disabled")
	ErrUserOAuthNotBound           = errors.New("user oauth not bound")
	ErrTelegramUnbindRequiresEmail = errors.New("telegram unbind requires real email")
	ErrGoogleUnbindLocked          = errors.New("google unbind would lock account")
	ErrInvalidEmail                = errors.New("invalid email")
)

// UserListFilter 管理端用户列表过滤条件。
type UserListFilter struct {
	Page          int
	PageSize      int
	UserID        uint
	Keyword       string
	Status        string
	CreatedFrom   *time.Time
	CreatedTo     *time.Time
	LastLoginFrom *time.Time
	LastLoginTo   *time.Time
	SortBy        string
	SortOrder     string
}

// UserDirectory 用户读写端口。
type UserDirectory interface {
	List(filter UserListFilter) ([]userdomain.User, int64, error)
	GetByID(id uint) (*userdomain.User, error)
	GetByEmail(email string) (*userdomain.User, error)
	Update(user *userdomain.User) error
	BatchUpdateStatus(ids []uint, status string) error
}

// EmailNormalizer 邮箱规范化端口。
type EmailNormalizer interface {
	NormalizeEmail(email string) (string, error)
}

// WalletBalances 钱包余额查询端口。
type WalletBalances interface {
	GetBalancesByUserIDs(userIDs []uint) (map[uint]money.Amount, error)
	GetAccount(userID uint) (*walletdomain.Account, error)
}

// OAuthIdentityDirectory 第三方身份查询端口。
type OAuthIdentityDirectory interface {
	ListByUserID(userID uint) ([]externalidentitydomain.Identity, error)
}

// OAuthIdentityUnbinder 第三方身份解绑端口。
type OAuthIdentityUnbinder interface {
	UnbindTelegram(userID uint) error
	UnbindGoogle(userID uint) error
}

// CouponUsageDirectory 优惠券使用记录端口。
type CouponUsageDirectory interface {
	ListByUser(filter couponcontract.UsageListFilter) ([]coupondomain.CouponUsage, int64, error)
}

// CouponDirectory 优惠券查询端口。
type CouponDirectory interface {
	ListByIDs(ids []uint) ([]coupondomain.Coupon, error)
}

// ProductDirectory 商品查询端口。
type ProductDirectory interface {
	ListByIDs(ids []uint) ([]productdomain.Product, error)
}

// AuthStateCache 用户鉴权状态缓存端口。
type AuthStateCache interface {
	SetUserAuthState(ctx context.Context, user *userdomain.User) error
	DelUserAuthState(ctx context.Context, userID uint) error
}

// AdminHandler 处理后台用户管理 HTTP 请求。
type AdminHandler struct {
	users         UserDirectory
	emails        EmailNormalizer
	wallets       WalletBalances
	oauth         OAuthIdentityDirectory
	oauthUnbinder OAuthIdentityUnbinder
	couponUsages  CouponUsageDirectory
	coupons       CouponDirectory
	products      ProductDirectory
	authState     AuthStateCache
}

func NewAdminHandler(
	users UserDirectory,
	emails EmailNormalizer,
	wallets WalletBalances,
	oauth OAuthIdentityDirectory,
	oauthUnbinder OAuthIdentityUnbinder,
	couponUsages CouponUsageDirectory,
	coupons CouponDirectory,
	products ProductDirectory,
	authState AuthStateCache,
) *AdminHandler {
	if users == nil {
		panic("admin user handler: users is nil")
	}
	if emails == nil {
		panic("admin user handler: emails is nil")
	}
	if wallets == nil {
		panic("admin user handler: wallets is nil")
	}
	if oauth == nil {
		panic("admin user handler: oauth is nil")
	}
	if oauthUnbinder == nil {
		panic("admin user handler: oauthUnbinder is nil")
	}
	if couponUsages == nil {
		panic("admin user handler: couponUsages is nil")
	}
	if coupons == nil {
		panic("admin user handler: coupons is nil")
	}
	if products == nil {
		panic("admin user handler: products is nil")
	}
	return &AdminHandler{
		users:         users,
		emails:        emails,
		wallets:       wallets,
		oauth:         oauth,
		oauthUnbinder: oauthUnbinder,
		couponUsages:  couponUsages,
		coupons:       coupons,
		products:      products,
		authState:     authState,
	}
}

// UpdateAdminUserRequest 管理员更新用户请求。
type UpdateAdminUserRequest struct {
	Nickname      *string `json:"nickname"`
	Locale        *string `json:"locale"`
	Status        *string `json:"status"`
	Email         *string `json:"email"`
	Password      *string `json:"password"`
	AdminNote     *string `json:"admin_note"`
	EmailVerified *bool   `json:"email_verified"`
}

// BatchUpdateUserStatusRequest 批量更新用户状态请求。
type BatchUpdateUserStatusRequest struct {
	UserIDs []uint `json:"user_ids" binding:"required"`
	Status  string `json:"status" binding:"required"`
}

// UserCouponUsageProduct 优惠券适用商品。
type UserCouponUsageProduct struct {
	ID    uint         `json:"id"`
	Title jsonmap.JSON `json:"title"`
}

// UserCouponUsageItem 用户优惠券使用记录返回。
type UserCouponUsageItem struct {
	ID             uint                     `json:"id"`
	CouponID       uint                     `json:"coupon_id"`
	CouponCode     string                   `json:"coupon_code"`
	CouponType     string                   `json:"coupon_type"`
	OrderID        uint                     `json:"order_id"`
	DiscountAmount money.Amount             `json:"discount_amount"`
	CreatedAt      time.Time                `json:"created_at"`
	ScopeRefIDs    []uint                   `json:"scope_ref_ids"`
	ScopeProducts  []UserCouponUsageProduct `json:"scope_products"`
}

// AdminUserListItem 管理端用户列表项。
type AdminUserListItem struct {
	userdomain.User
	WalletBalance money.Amount `json:"wallet_balance"`
}

// AdminUserOAuthIdentityItem 管理端用户第三方身份项。
type AdminUserOAuthIdentityItem struct {
	ID             uint       `json:"id"`
	Provider       string     `json:"provider"`
	ProviderUserID string     `json:"provider_user_id"`
	Username       string     `json:"username"`
	AvatarURL      string     `json:"avatar_url"`
	AuthAt         *time.Time `json:"auth_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// AdminUserDetail 管理端用户详情。
type AdminUserDetail struct {
	userdomain.User
	WalletBalance   money.Amount                 `json:"wallet_balance"`
	OAuthIdentities []AdminUserOAuthIdentityItem `json:"oauth_identities"`
}

// GetAdminUsers 获取用户列表。
func (h *AdminHandler) GetAdminUsers(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	userID, err := ginutil.ParseQueryUint(c.Query("user_id"), true)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", err)
		return
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))

	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	lastLoginFrom, lastLoginTo, err := ginutil.ParseQueryTimeRange(c, "last_login_from", "last_login_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	users, total, err := h.users.List(UserListFilter{
		Page:          page,
		PageSize:      pageSize,
		UserID:        userID,
		Keyword:       keyword,
		Status:        status,
		CreatedFrom:   createdFrom,
		CreatedTo:     createdTo,
		LastLoginFrom: lastLoginFrom,
		LastLoginTo:   lastLoginTo,
		SortBy:        strings.TrimSpace(c.Query("sort_by")),
		SortOrder:     strings.TrimSpace(c.Query("sort_order")),
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}

	userIDs := make([]uint, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	balanceMap, err := h.wallets.GetBalancesByUserIDs(userIDs)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	items := make([]AdminUserListItem, 0, len(users))
	for _, user := range users {
		balance, ok := balanceMap[user.ID]
		if !ok {
			balance = money.FromDecimal(decimal.Zero)
		}
		items = append(items, AdminUserListItem{
			User:          user,
			WalletBalance: balance,
		})
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, items, pagination)
}

// GetAdminUser 获取用户详情。
func (h *AdminHandler) GetAdminUser(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}

	user, err := h.users.GetByID(userID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if user == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		return
	}
	account, err := h.wallets.GetAccount(user.ID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	identities, err := h.oauth.ListByUserID(user.ID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
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

// UpdateAdminUser 更新用户信息。
func (h *AdminHandler) UpdateAdminUser(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}

	var req UpdateAdminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	user, err := h.users.GetByID(userID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if user == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		return
	}

	updated := false
	revokeToken := false
	if req.Email != nil {
		normalized, err := h.emails.NormalizeEmail(*req.Email)
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_invalid", nil)
			return
		}
		existing, err := h.users.GetByEmail(normalized)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
			return
		}
		if existing != nil && existing.ID != user.ID {
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_exists", nil)
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
			if !userdomain.ValidDisplayName(trimmed) {
				ginutil.RespondError(c, response.CodeBadRequest, "error.nickname_invalid", nil)
				return
			}
			user.DisplayName = trimmed
			updated = true
		}
	}
	if req.Password != nil {
		trimmed := strings.TrimSpace(*req.Password)
		if trimmed != "" {
			hashed, err := bcrypt.GenerateFromPassword([]byte(trimmed), bcrypt.DefaultCost)
			if err != nil {
				ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
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
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	now := time.Now()
	user.UpdatedAt = now
	if revokeToken {
		user.TokenVersion++
		user.TokenInvalidBefore = &now
	}
	if err := h.users.Update(user); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		return
	}
	if h.authState != nil {
		_ = h.authState.SetUserAuthState(c.Request.Context(), user)
	}

	response.Success(c, user)
}

// UnbindAdminUserTelegram 管理员解除目标用户的 Telegram 绑定。
// DELETE /admin/users/:id/oauth/telegram
func (h *AdminHandler) UnbindAdminUserTelegram(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}

	if err := h.oauthUnbinder.UnbindTelegram(userID); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		case errors.Is(err, ErrUserDisabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.user_disabled", nil)
		case errors.Is(err, ErrUserOAuthNotBound):
			ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_not_bound", nil)
		case errors.Is(err, ErrTelegramUnbindRequiresEmail):
			ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_unbind_requires_email", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"unbound": true})
}

// UnbindAdminUserGoogle 管理员解除目标用户的 Google 绑定。
// DELETE /admin/users/:id/oauth/google
func (h *AdminHandler) UnbindAdminUserGoogle(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}

	if err := h.oauthUnbinder.UnbindGoogle(userID); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		case errors.Is(err, ErrUserDisabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.user_disabled", nil)
		case errors.Is(err, ErrUserOAuthNotBound):
			ginutil.RespondError(c, response.CodeBadRequest, "error.google_not_bound", nil)
		case errors.Is(err, ErrGoogleUnbindLocked):
			ginutil.RespondError(c, response.CodeBadRequest, "error.google_unbind_locked", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"unbound": true})
}

// GetAdminUserCouponUsages 获取用户优惠券使用记录。
func (h *AdminHandler) GetAdminUserCouponUsages(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}

	page, pageSize := ginutil.ParsePagination(c)

	usages, total, err := h.couponUsages.ListByUser(couponcontract.UsageListFilter{
		Page:     page,
		PageSize: pageSize,
		UserID:   userID,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
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

	coupons := make(map[uint]*coupondomain.Coupon)
	if len(couponIDs) > 0 {
		items, err := h.coupons.ListByIDs(couponIDs)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
			return
		}
		for i := range items {
			item := items[i]
			coupons[item.ID] = &item
		}
	}

	productIDs := make(map[uint]struct{})
	couponScopeIDs := make(map[uint][]uint)
	for _, item := range coupons {
		scopeIDs := decodeScopeRefIDs(item.ScopeRefIDs)
		couponScopeIDs[item.ID] = scopeIDs
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
		products, err := h.products.ListByIDs(ids)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
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
		if couponItem, ok := coupons[usage.CouponID]; ok {
			item.CouponCode = couponItem.Code
			item.CouponType = couponItem.Type
			scopeIDs := couponScopeIDs[couponItem.ID]
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

// BatchUpdateUserStatus 批量更新用户状态。
func (h *AdminHandler) BatchUpdateUserStatus(c *gin.Context) {
	var req BatchUpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if len(req.UserIDs) == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	normalizedStatus := strings.ToLower(strings.TrimSpace(req.Status))
	if normalizedStatus != constants.UserStatusActive && normalizedStatus != constants.UserStatusDisabled {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	if err := h.users.BatchUpdateStatus(req.UserIDs, normalizedStatus); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		return
	}
	if h.authState != nil {
		for _, userID := range req.UserIDs {
			_ = h.authState.DelUserAuthState(c.Request.Context(), userID)
		}
	}

	response.Success(c, gin.H{"updated": len(req.UserIDs)})
}
