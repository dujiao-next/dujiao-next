package giftcardhttp

import (
	"errors"
	"net/http"
	"strings"

	giftcardapp "github.com/dujiao-next/internal/modules/giftcard/application"
	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcardpresenter "github.com/dujiao-next/internal/modules/giftcard/transport/presenter"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/dujiao-next/internal/platform/http/channelresponse"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// ChannelUserProvisioner 是渠道礼品卡兑换所需的身份开通端口。
type ChannelUserProvisioner interface {
	ProvisionUserID(channelUserID string) (uint, error)
}

// ChannelHandler 处理渠道礼品卡请求。
type ChannelHandler struct {
	cards UserService
	users ChannelUserProvisioner
}

func NewChannelHandler(cards UserService, users ChannelUserProvisioner) *ChannelHandler {
	if cards == nil {
		panic("giftcard channel handler: cards is nil")
	}
	if users == nil {
		panic("giftcard channel handler: users is nil")
	}
	return &ChannelHandler{cards: cards, users: users}
}

type channelRedeemRequest struct {
	ChannelUserID  string `json:"channel_user_id"`
	TelegramUserID string `json:"telegram_user_id"`
	Code           string `json:"code" binding:"required"`
}

// Redeem POST /api/v1/channel/wallet/gift-card/redeem
func (h *ChannelHandler) Redeem(c *gin.Context) {
	var req channelRedeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		channelresponse.BindError(c, err)
		return
	}

	channelUserID := channelresponse.UserIDValue(req.ChannelUserID, req.TelegramUserID)
	if channelUserID == "" {
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.users.ProvisionUserID(channelUserID)
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_wallet_gift_card_resolve_user", "channel_user_id", channelUserID, "error", err)
		respondChannelIdentityError(c, err)
		return
	}

	card, account, txn, err := h.cards.RedeemGiftCard(giftcardapp.RedeemInput{
		UserID: userID,
		Code:   strings.TrimSpace(req.Code),
	})
	if err != nil {
		ginutil.RequestLog(c).Warnw("channel_wallet_gift_card_redeem_failed", "user_id", userID, "channel_user_id", channelUserID, "error", err)
		respondChannelGiftCardError(c, err)
		return
	}

	channelresponse.Success(c, giftcardpresenter.NewGiftCardRedeemResp(card, account, txn))
}

func respondChannelIdentityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, telegramauthapp.ErrTelegramAuthPayloadInvalid):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
	case errors.Is(err, userauthapp.ErrInvalidEmail):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "validation_error", "error.email_invalid", nil)
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
	case errors.Is(err, userauthapp.ErrUserOAuthIdentityExists):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "channel_identity_conflict", "error.telegram_bind_conflict", nil)
	case errors.Is(err, userauthapp.ErrUserOAuthAlreadyBound):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "channel_identity_conflict", "error.telegram_already_bound", nil)
	default:
		channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal_error", "error.internal_error", err)
	}
}

func respondChannelGiftCardError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, giftcardcontract.ErrInvalid):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "gift_card_invalid", "error.gift_card_invalid", nil)
	case errors.Is(err, giftcardcontract.ErrNotFound):
		channelresponse.Error(c, http.StatusNotFound, response.CodeNotFound, "gift_card_not_found", "error.gift_card_not_found", nil)
	case errors.Is(err, giftcardcontract.ErrExpired):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "gift_card_expired", "error.gift_card_expired", nil)
	case errors.Is(err, giftcardcontract.ErrDisabled):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "gift_card_disabled", "error.gift_card_disabled", nil)
	case errors.Is(err, giftcardcontract.ErrRedeemed):
		channelresponse.Error(c, http.StatusBadRequest, response.CodeBadRequest, "gift_card_redeemed", "error.gift_card_redeemed", nil)
	default:
		channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "gift_card_redeem_failed", "error.gift_card_redeem_failed", err)
	}
}
