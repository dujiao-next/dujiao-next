package wallethttp

import (
	"net/http"
	"strings"

	paymentpresenter "github.com/dujiao-next/internal/modules/payment/transport/presenter"

	"github.com/dujiao-next/internal/platform/http/channelresponse"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/dujiao-next/internal/constants"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ChannelUserProvisioner 是渠道钱包所需的身份开通端口。
type ChannelUserProvisioner interface {
	ProvisionUserID(channelUserID string) (uint, error)
}

// ChannelHandler 处理渠道钱包请求。
type ChannelHandler struct {
	wallets  WalletService
	payments PaymentService
	users    ChannelUserProvisioner
	settings SiteCurrencyReader
}

// NewChannelHandler 创建渠道钱包 Handler。
func NewChannelHandler(
	wallets WalletService,
	payments PaymentService,
	users ChannelUserProvisioner,
	settings SiteCurrencyReader,
) *ChannelHandler {
	if wallets == nil || payments == nil || users == nil {
		panic("wallet channel handler: required dependency is nil")
	}
	return &ChannelHandler{
		wallets:  wallets,
		payments: payments,
		users:    users,
		settings: settings,
	}
}

// GetWallet GET /api/v1/channel/wallet?telegram_user_id=xxx
func (h *ChannelHandler) GetWallet(c *gin.Context) {
	channelUserID := channelresponse.UserIDValue(c.Query("channel_user_id"), c.Query("telegram_user_id"))
	if channelUserID == "" {
		channelresponse.Error(c, http.StatusBadRequest, 400, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.users.ProvisionUserID(channelUserID)
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_wallet_resolve_user", "channel_user_id", channelUserID, "error", err)
		channelIdentityError(c, err)
		return
	}

	account, err := h.wallets.GetAccount(userID)
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_wallet_get_account", "user_id", userID, "error", err)
		channelresponse.Error(c, http.StatusInternalServerError, 500, "internal_error", "error.internal_error", err)
		return
	}

	channelresponse.Success(c, gin.H{
		"balance":  account.Balance.StringFixed(2),
		"currency": "CNY",
	})
}

// GetWalletTransactions GET /api/v1/channel/wallet/transactions?telegram_user_id=xxx&page=1&page_size=5
func (h *ChannelHandler) GetWalletTransactions(c *gin.Context) {
	channelUserID := channelresponse.UserIDValue(c.Query("channel_user_id"), c.Query("telegram_user_id"))
	if channelUserID == "" {
		channelresponse.Error(c, http.StatusBadRequest, 400, "validation_error", "error.bad_request", nil)
		return
	}

	page, pageSize := ginutil.ParsePaginationWithBounds(c, "page", "page_size", 5, 20)

	userID, err := h.users.ProvisionUserID(channelUserID)
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_wallet_txns_resolve_user", "channel_user_id", channelUserID, "error", err)
		channelIdentityError(c, err)
		return
	}

	txns, total, err := h.wallets.ListTransactions(userID, page, pageSize)
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_wallet_list_txns", "user_id", userID, "error", err)
		channelresponse.Error(c, http.StatusInternalServerError, 500, "internal_error", "error.internal_error", err)
		return
	}

	type txnItem struct {
		Type         string `json:"type"`
		Direction    string `json:"direction"`
		Amount       string `json:"amount"`
		BalanceAfter string `json:"balance_after"`
		Remark       string `json:"remark"`
		CreatedAt    string `json:"created_at"`
	}

	items := make([]txnItem, 0, len(txns))
	for _, t := range txns {
		items = append(items, txnItem{
			Type:         t.Type,
			Direction:    t.Direction,
			Amount:       t.Amount.StringFixed(2),
			BalanceAfter: t.BalanceAfter.StringFixed(2),
			Remark:       t.Remark,
			CreatedAt:    t.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	channelresponse.Success(c, gin.H{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// CreateWalletRecharge POST /api/v1/channel/wallet/recharge
func (h *ChannelHandler) CreateWalletRecharge(c *gin.Context) {
	var req struct {
		ChannelUserID  string `json:"channel_user_id"`
		TelegramUserID string `json:"telegram_user_id"`
		Amount         string `json:"amount" binding:"required"`
		ChannelID      uint   `json:"channel_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		channelresponse.BindError(c, err)
		return
	}
	channelUserID := channelresponse.UserIDValue(req.ChannelUserID, req.TelegramUserID)
	if channelUserID == "" {
		channelresponse.Error(c, http.StatusBadRequest, 400, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.users.ProvisionUserID(channelUserID)
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_wallet_recharge_resolve_user", "channel_user_id", channelUserID, "error", err)
		channelIdentityError(c, err)
		return
	}

	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		channelresponse.Error(c, http.StatusBadRequest, 400, "validation_error", "error.bad_request", nil)
		return
	}

	currency := ""
	if h.settings != nil {
		currency, _ = h.settings.GetSiteCurrency(constants.SiteCurrencyDefault)
	}

	result, err := h.payments.CreateWalletRechargePayment(CreateRechargePaymentInput{
		UserID:    userID,
		ChannelID: req.ChannelID,
		Amount:    money.FromDecimal(amount),
		Currency:  currency,
		ClientIP:  c.ClientIP(),
		Context:   c.Request.Context(),
	})
	if err != nil {
		ginutil.RequestLog(c).Errorw("channel_wallet_recharge_create", "user_id", userID, "error", err)
		channelresponse.Error(c, http.StatusBadRequest, 400, "payment_create_failed", "error.payment_create_failed", err)
		return
	}

	paymentBlock := gin.H{
		"id":               result.Payment.ID,
		"amount":           result.Payment.Amount.StringFixed(2),
		"fee_amount":       result.Payment.FeeAmount.StringFixed(2),
		"fee_policy":       result.Payment.FeePolicy,
		"fee_currency":     result.Recharge.Currency,
		"currency":         result.Payment.Currency,
		"status":           result.Payment.Status,
		"interaction_mode": result.Payment.InteractionMode,
		"pay_url":          result.Payment.PayURL,
		"qr_code":          result.Payment.QRCode,
		"expires_at":       result.Payment.ExpiredAt,
	}
	if info := paymentpresenter.ExtractCryptoWalletInfo(result.Payment.ProviderType, result.Payment.InteractionMode, result.Payment.ProviderPayload); info.HasAny() {
		if info.Address != "" {
			paymentBlock["wallet_address"] = info.Address
		}
		if info.ChainAmount != "" {
			paymentBlock["chain_amount"] = info.ChainAmount
		}
		if info.Chain != "" {
			paymentBlock["chain"] = info.Chain
		}
		if info.TokenID != "" {
			paymentBlock["token_id"] = info.TokenID
		}
	}

	channelresponse.Success(c, gin.H{
		"recharge_no": result.Recharge.RechargeNo,
		"payment":     paymentBlock,
	})
}
