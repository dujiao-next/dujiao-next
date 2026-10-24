package affiliatehttp

import (
	"errors"
	"net/http"
	"strings"
	"time"

	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"

	affiliateapp "github.com/dujiao-next/internal/modules/affiliate/application"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/dujiao-next/internal/platform/http/channelresponse"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ChannelIdentity 渠道侧开通/解析用户所需的 Telegram 身份字段。
type ChannelIdentity struct {
	ChannelUserID string
	Username      string
	FirstName     string
	LastName      string
	AvatarURL     string
}

// ChannelUserProvisioner 渠道身份开通端口。
type ChannelUserProvisioner interface {
	ProvisionUserID(identity ChannelIdentity) (uint, error)
}

// AffiliateSettings 推广设置读取端口。
type AffiliateSettings interface {
	GetAffiliateSetting() (settingsintegration.AffiliateSetting, error)
}

// ChannelHandler 处理渠道推广返利请求。
type ChannelHandler struct {
	affiliate Service
	users     ChannelUserProvisioner
	settings  AffiliateSettings
}

func NewChannelHandler(affiliateSvc Service, users ChannelUserProvisioner, settingsSvc AffiliateSettings) *ChannelHandler {
	if affiliateSvc == nil {
		panic("affiliate channel handler: affiliate is nil")
	}
	if users == nil {
		panic("affiliate channel handler: users is nil")
	}
	if settingsSvc == nil {
		panic("affiliate channel handler: settings is nil")
	}
	return &ChannelHandler{affiliate: affiliateSvc, users: users, settings: settingsSvc}
}

type channelIdentityRequest struct {
	ChannelUserID  string `json:"channel_user_id"`
	TelegramUserID string `json:"telegram_user_id"`
	Username       string `json:"username"`
	TelegramUser   string `json:"telegram_username"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	AvatarURL      string `json:"avatar_url"`
}

type channelTrackClickRequest struct {
	ChannelUserID  string `json:"channel_user_id,omitempty"`
	TelegramUserID string `json:"telegram_user_id,omitempty"`
	AffiliateCode  string `json:"affiliate_code" binding:"required"`
	VisitorKey     string `json:"visitor_key,omitempty"`
	LandingPath    string `json:"landing_path,omitempty"`
	Referrer       string `json:"referrer,omitempty"`
}

type channelApplyWithdrawRequest struct {
	ChannelUserID  string `json:"channel_user_id,omitempty"`
	TelegramUserID string `json:"telegram_user_id,omitempty"`
	Amount         string `json:"amount" binding:"required"`
	Channel        string `json:"channel" binding:"required"`
	Account        string `json:"account" binding:"required"`
}

// OpenAffiliate POST /api/v1/channel/affiliate/open
func (h *ChannelHandler) OpenAffiliate(c *gin.Context) {
	var req channelIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		channelresponse.BindError(c, err)
		return
	}

	identity := buildChannelIdentity(req)
	if strings.TrimSpace(identity.ChannelUserID) == "" {
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.users.ProvisionUserID(identity)
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_affiliate_open_resolve_user", "channel_user_id", identity.ChannelUserID, "error", err)
		respondChannelIdentityError(c, err)
		return
	}

	profile, err := h.affiliate.OpenAffiliate(userID)
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrDisabled):
			channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "affiliate_disabled", "error.forbidden", nil)
		case errors.Is(err, affiliateapp.ErrNotFound):
			channelresponse.Error(c, http.StatusNotFound, response.CodeNotFound, "user_not_found", "error.user_not_found", nil)
		case errors.Is(err, affiliateapp.ErrUserDisabled):
			channelresponse.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "user_disabled", "error.user_disabled", nil)
		default:
			ginutil.RequestLog(c).Errorw("channel_affiliate_open_failed", "user_id", userID, "error", err)
			channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "affiliate_open_failed", "error.save_failed", err)
		}
		return
	}

	channelresponse.Success(c, buildChannelAffiliateProfileResponse(profile))
}

// TrackAffiliateClick POST /api/v1/channel/affiliate/click
func (h *ChannelHandler) TrackAffiliateClick(c *gin.Context) {
	var req channelTrackClickRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		channelresponse.BindError(c, err)
		return
	}

	channelUserID := channelresponse.UserIDValue(req.ChannelUserID, req.TelegramUserID)
	if channelUserID == "" {
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	visitorKey := strings.TrimSpace(req.VisitorKey)
	if visitorKey == "" {
		visitorKey = channelUserID
	}

	if err := h.affiliate.TrackClick(affiliateapp.TrackClickInput{
		AffiliateCode: req.AffiliateCode,
		VisitorKey:    visitorKey,
		LandingPath:   strings.TrimSpace(req.LandingPath),
		Referrer:      strings.TrimSpace(req.Referrer),
		ClientIP:      c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
	}); err != nil {
		ginutil.RequestLog(c).Errorw("channel_affiliate_track_click_failed", "channel_user_id", channelUserID, "affiliate_code", req.AffiliateCode, "error", err)
		channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "affiliate_track_click_failed", "error.save_failed", err)
		return
	}

	channelresponse.Success(c, gin.H{"ok": true})
}

// GetAffiliateDashboard GET /api/v1/channel/affiliate/dashboard
func (h *ChannelHandler) GetAffiliateDashboard(c *gin.Context) {
	userID, channelUserID, ok := h.resolveChannelAffiliateUserID(c)
	if !ok {
		return
	}

	dashboard, err := h.affiliate.GetUserDashboard(userID)
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_affiliate_dashboard_failed", "user_id", userID, "channel_user_id", channelUserID, "error", err)
		channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "affiliate_dashboard_failed", "error.user_fetch_failed", err)
		return
	}
	setting, settingErr := h.settings.GetAffiliateSetting()
	if settingErr != nil {
		ginutil.RequestLog(c).Errorw("channel_affiliate_dashboard_setting_failed", "user_id", userID, "channel_user_id", channelUserID, "error", settingErr)
		channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "affiliate_dashboard_failed", "error.user_fetch_failed", settingErr)
		return
	}

	channelresponse.Success(c, gin.H{
		"opened":               dashboard.Opened,
		"affiliate_code":       dashboard.AffiliateCode,
		"promotion_path":       dashboard.PromotionPath,
		"click_count":          dashboard.ClickCount,
		"valid_order_count":    dashboard.ValidOrderCount,
		"conversion_rate":      dashboard.ConversionRate,
		"pending_commission":   dashboard.PendingCommission,
		"available_commission": dashboard.AvailableCommission,
		"withdrawn_commission": dashboard.WithdrawnCommission,
		"min_withdraw_amount":  setting.MinWithdrawAmount,
		"withdraw_channels":    setting.WithdrawChannels,
	})
}

// ListAffiliateCommissions GET /api/v1/channel/affiliate/commissions
func (h *ChannelHandler) ListAffiliateCommissions(c *gin.Context) {
	userID, channelUserID, ok := h.resolveChannelAffiliateUserID(c)
	if !ok {
		return
	}

	page, pageSize := ginutil.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))

	rows, total, err := h.affiliate.ListUserCommissions(userID, page, pageSize, status)
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_affiliate_commissions_failed", "user_id", userID, "channel_user_id", channelUserID, "error", err)
		channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "affiliate_commissions_failed", "error.user_fetch_failed", err)
		return
	}

	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{
			"id":                   row.ID,
			"affiliate_profile_id": row.AffiliateProfileID,
			"order_id":             row.OrderID,
			"order_no":             strings.TrimSpace(row.Order.OrderNo),
			"order_item_id":        channelAffiliateUintValue(row.OrderItemID),
			"commission_type":      row.CommissionType,
			"base_amount":          row.BaseAmount,
			"rate_percent":         row.RatePercent,
			"commission_amount":    row.CommissionAmount,
			"status":               row.Status,
			"confirm_at":           channelAffiliateTimeValue(row.ConfirmAt),
			"available_at":         channelAffiliateTimeValue(row.AvailableAt),
			"withdraw_request_id":  channelAffiliateUintValue(row.WithdrawRequestID),
			"invalid_reason":       row.InvalidReason,
			"created_at":           row.CreatedAt,
			"updated_at":           row.UpdatedAt,
		})
	}

	channelresponse.Success(c, gin.H{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// ListAffiliateWithdraws GET /api/v1/channel/affiliate/withdraws
func (h *ChannelHandler) ListAffiliateWithdraws(c *gin.Context) {
	userID, channelUserID, ok := h.resolveChannelAffiliateUserID(c)
	if !ok {
		return
	}

	page, pageSize := ginutil.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))

	rows, total, err := h.affiliate.ListUserWithdraws(userID, page, pageSize, status)
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_affiliate_withdraws_failed", "user_id", userID, "channel_user_id", channelUserID, "error", err)
		channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "affiliate_withdraws_failed", "error.user_fetch_failed", err)
		return
	}

	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{
			"id":                   row.ID,
			"affiliate_profile_id": row.AffiliateProfileID,
			"amount":               row.Amount,
			"channel":              row.Channel,
			"account":              row.Account,
			"status":               row.Status,
			"reject_reason":        row.RejectReason,
			"processed_by":         channelAffiliateUintValue(row.ProcessedBy),
			"processed_at":         channelAffiliateTimeValue(row.ProcessedAt),
			"created_at":           row.CreatedAt,
			"updated_at":           row.UpdatedAt,
		})
	}

	channelresponse.Success(c, gin.H{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// ApplyAffiliateWithdraw POST /api/v1/channel/affiliate/withdraws
func (h *ChannelHandler) ApplyAffiliateWithdraw(c *gin.Context) {
	var req channelApplyWithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		channelresponse.BindError(c, err)
		return
	}

	channelUserID := channelresponse.UserIDValue(req.ChannelUserID, req.TelegramUserID)
	if channelUserID == "" {
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "affiliate_withdraw_amount_invalid", "error.bad_request", nil)
		return
	}

	userID, err := h.users.ProvisionUserID(ChannelIdentity{ChannelUserID: channelUserID})
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_affiliate_apply_withdraw_resolve_user", "channel_user_id", channelUserID, "error", err)
		respondChannelIdentityError(c, err)
		return
	}

	row, err := h.affiliate.ApplyWithdraw(userID, affiliateapp.WithdrawApplyInput{
		Amount:  amount,
		Channel: strings.TrimSpace(req.Channel),
		Account: strings.TrimSpace(req.Account),
	})
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrDisabled):
			channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "affiliate_disabled", "error.forbidden", nil)
		case errors.Is(err, affiliateapp.ErrNotOpened):
			channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "affiliate_not_opened", "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrWithdrawAmountInvalid):
			channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "affiliate_withdraw_amount_invalid", "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrWithdrawChannelInvalid):
			channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "affiliate_withdraw_channel_invalid", "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrWithdrawInsufficient):
			channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "affiliate_withdraw_insufficient", "error.bad_request", nil)
		default:
			ginutil.RequestLog(c).Errorw("channel_affiliate_apply_withdraw_failed", "user_id", userID, "channel_user_id", channelUserID, "error", err)
			channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "affiliate_withdraw_apply_failed", "error.save_failed", err)
		}
		return
	}

	channelresponse.Success(c, gin.H{
		"id":                   row.ID,
		"affiliate_profile_id": row.AffiliateProfileID,
		"amount":               row.Amount,
		"channel":              row.Channel,
		"account":              row.Account,
		"status":               row.Status,
		"reject_reason":        row.RejectReason,
		"processed_by":         channelAffiliateUintValue(row.ProcessedBy),
		"processed_at":         channelAffiliateTimeValue(row.ProcessedAt),
		"created_at":           row.CreatedAt,
		"updated_at":           row.UpdatedAt,
	})
}

func (h *ChannelHandler) resolveChannelAffiliateUserID(c *gin.Context) (uint, string, bool) {
	channelUserID := channelresponse.UserIDValue(c.Query("channel_user_id"), c.Query("telegram_user_id"))
	if channelUserID == "" {
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return 0, "", false
	}

	userID, err := h.users.ProvisionUserID(ChannelIdentity{ChannelUserID: channelUserID})
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_affiliate_resolve_user", "channel_user_id", channelUserID, "error", err)
		respondChannelIdentityError(c, err)
		return 0, channelUserID, false
	}
	return userID, channelUserID, true
}

func respondChannelIdentityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, telegramauthapp.ErrTelegramAuthPayloadInvalid),
		errors.Is(err, userauthapp.ErrInvalidEmail):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
	case errors.Is(err, userauthapp.ErrNotFound):
		channelresponse.Error(c, http.StatusNotFound, response.CodeNotFound, "user_not_found", "error.user_not_found", nil)
	case errors.Is(err, userauthapp.ErrVerifyCodeInvalid):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "verify_code_invalid", "error.verify_code_invalid", nil)
	case errors.Is(err, userauthapp.ErrVerifyCodeExpired):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "verify_code_expired", "error.verify_code_expired", nil)
	case errors.Is(err, userauthapp.ErrVerifyCodeAttemptsExceeded):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "verify_code_invalid", "error.verify_code_attempts_exceeded", nil)
	case errors.Is(err, userauthapp.ErrUserDisabled):
		channelresponse.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "user_disabled", "error.user_disabled", nil)
	// Bot 新用户在邀请注册模式下需先提交邀请码（/channel/identities/telegram/provision 的 invite_code）。
	case errors.Is(err, userauthapp.ErrInviteCodeRequired):
		channelresponse.Error(c, http.StatusForbidden, response.CodeForbidden, "invite_code_required", "error.invite_code_required", nil)
	case errors.Is(err, userauthapp.ErrInviteCodeInvalid):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invite_code_invalid", "error.invite_code_invalid", nil)
	case errors.Is(err, userauthapp.ErrRegistrationDisabled):
		channelresponse.Error(c, http.StatusForbidden, response.CodeForbidden, "registration_disabled", "error.registration_disabled", nil)
	case errors.Is(err, userauthapp.ErrUserOAuthIdentityExists),
		errors.Is(err, userauthapp.ErrUserOAuthAlreadyBound):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "channel_identity_conflict", "error.telegram_bind_conflict", nil)
	default:
		channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal_error", "error.internal_error", err)
	}
}

func buildChannelIdentity(req channelIdentityRequest) ChannelIdentity {
	return ChannelIdentity{
		ChannelUserID: channelresponse.UserIDValue(req.ChannelUserID, req.TelegramUserID),
		Username:      strings.TrimSpace(firstNonEmpty(req.Username, req.TelegramUser)),
		FirstName:     strings.TrimSpace(req.FirstName),
		LastName:      strings.TrimSpace(req.LastName),
		AvatarURL:     strings.TrimSpace(req.AvatarURL),
	}
}

func buildChannelAffiliateProfileResponse(profile *affiliatedomain.Profile) gin.H {
	if profile == nil {
		return gin.H{}
	}
	return gin.H{
		"id":         profile.ID,
		"user_id":    profile.UserID,
		"code":       profile.AffiliateCode,
		"status":     profile.Status,
		"created_at": profile.CreatedAt,
		"updated_at": profile.UpdatedAt,
	}
}

func channelAffiliateUintValue(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}

func channelAffiliateTimeValue(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
