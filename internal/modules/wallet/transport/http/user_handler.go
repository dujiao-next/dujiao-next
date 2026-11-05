package wallethttp

import (
	"context"
	"errors"
	"strings"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	walletpresenter "github.com/dujiao-next/internal/modules/wallet/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

var (
	ErrInvalidAmount                    = errors.New("wallet invalid amount")
	ErrNotSupportedForGuest             = errors.New("wallet not supported for guest")
	ErrRechargeNotFound                 = errors.New("wallet recharge not found")
	ErrPaymentInvalid                   = errors.New("payment invalid")
	ErrPaymentNotFound                  = errors.New("payment not found")
	ErrOrderNotFound                    = errors.New("order not found")
	ErrOrderStatusInvalid               = errors.New("order status invalid")
	ErrPaymentChannelNotFound           = errors.New("payment channel not found")
	ErrPaymentChannelInactive           = errors.New("payment channel inactive")
	ErrPaymentProviderNotSupported      = errors.New("payment provider not supported")
	ErrPaymentChannelConfigInvalid      = errors.New("payment channel config invalid")
	ErrPaymentGatewayRequestFailed      = errors.New("payment gateway request failed")
	ErrPaymentGatewayResponseInvalid    = errors.New("payment gateway response invalid")
	ErrPaymentCurrencyMismatch          = errors.New("payment currency mismatch")
	ErrPaymentChannelNotAllowedProduct  = errors.New("payment channel not allowed for product")
	ErrPaymentChannelNotAllowedRecharge = errors.New("payment channel not allowed for wallet recharge")
	ErrWalletOnlyPaymentRequired        = errors.New("wallet only payment required")
	ErrPaymentStatusInvalid             = errors.New("payment status invalid")
	ErrPaymentAmountMismatch            = errors.New("payment amount mismatch")
)

// WalletService 是用户钱包查询所需的最小端口。
type WalletService interface {
	GetAccount(userID uint) (*walletdomain.Account, error)
	ListTransactions(userID uint, page, pageSize int) ([]walletdomain.Transaction, int64, error)
	ListUserRechargeOrders(userID uint, page, pageSize int, status, rechargeNo string) ([]walletdomain.RechargeOrder, int64, error)
	StatsUserRechargeOrders(userID uint, rechargeNo string) (map[string]int64, error)
	GetRechargeOrderByRechargeNo(userID uint, rechargeNo string) (*walletdomain.RechargeOrder, error)
	GetRechargeOrderByPaymentIDAndUser(paymentID uint, userID uint) (*walletdomain.RechargeOrder, error)
}

// PaymentService 是用户钱包充值支付所需的最小端口。
type PaymentService interface {
	GetAvailableWalletRechargeChannels(amount money.Amount, user *userdomain.User) ([]map[string]interface{}, error)
	CreateWalletRechargePayment(input CreateRechargePaymentInput) (*CreateRechargePaymentResult, error)
	GetPayment(id uint) (*paymentdomain.Payment, error)
	CapturePayment(input CapturePaymentInput) (*paymentdomain.Payment, error)
}

// UserReader 用于读取支付渠道匹配需要的用户信息。
type UserReader interface {
	GetByID(id uint) (*userdomain.User, error)
}

// SiteCurrencyReader 用于提供默认站点币种。
type SiteCurrencyReader interface {
	GetSiteCurrency(defaultValue string) (string, error)
}

type CreateRechargePaymentInput struct {
	UserID        uint
	ChannelID     uint
	Amount        money.Amount
	Currency      string
	Remark        string
	ClientIP      string
	Context       context.Context
	RequestScheme string
}

type CreateRechargePaymentResult struct {
	Recharge *walletdomain.RechargeOrder
	Payment  *paymentdomain.Payment
}

type CapturePaymentInput struct {
	PaymentID uint
	Context   context.Context
}

// UserHandler 处理用户钱包 HTTP 请求。
type UserHandler struct {
	wallets  WalletService
	payments PaymentService
	users    UserReader
	settings SiteCurrencyReader
}

func NewUserHandler(wallets WalletService, payments PaymentService, users UserReader, settings SiteCurrencyReader) *UserHandler {
	if wallets == nil || payments == nil || users == nil {
		panic("wallet user handler: required dependency is nil")
	}
	return &UserHandler{wallets: wallets, payments: payments, users: users, settings: settings}
}

type walletRechargeRequest struct {
	Amount    string `json:"amount" binding:"required"`
	ChannelID uint   `json:"channel_id" binding:"required"`
	Currency  string `json:"currency"`
	Remark    string `json:"remark"`
}

type walletPaymentChannelsRequest struct {
	Amount string `json:"amount" binding:"required"`
}

func (h *UserHandler) GetPaymentChannels(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req walletPaymentChannelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	user, _ := h.users.GetByID(uid)
	channels, err := h.payments.GetAvailableWalletRechargeChannels(money.FromDecimal(amount), user)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	response.Success(c, channels)
}

func (h *UserHandler) GetWallet(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	account, err := h.wallets.GetAccount(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, walletpresenter.NewWalletAccountResp(account))
}

func (h *UserHandler) GetTransactions(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	transactions, total, err := h.wallets.ListTransactions(uid, page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, walletpresenter.NewWalletTransactionRespList(transactions), response.BuildPagination(page, pageSize, total))
}

func (h *UserHandler) Recharge(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req walletRechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	currency := strings.TrimSpace(req.Currency)
	if currency == "" && h.settings != nil {
		if siteCurrency, currencyErr := h.settings.GetSiteCurrency(constants.SiteCurrencyDefault); currencyErr == nil {
			currency = siteCurrency
		}
	}
	result, err := h.payments.CreateWalletRechargePayment(CreateRechargePaymentInput{
		UserID: uid, ChannelID: req.ChannelID, Amount: money.FromDecimal(amount), Currency: currency,
		Remark: strings.TrimSpace(req.Remark), ClientIP: c.ClientIP(), Context: c.Request.Context(), RequestScheme: ginutil.RequestScheme(c),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidAmount):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		case errors.Is(err, ErrNotSupportedForGuest):
			ginutil.RespondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		default:
			respondPaymentCreateError(c, err)
		}
		return
	}
	account, err := h.wallets.GetAccount(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, walletpresenter.NewWalletRechargePaymentPayload(result.Recharge, result.Payment, account))
}

func (h *UserHandler) GetRecharge(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	rechargeNo := strings.TrimSpace(c.Param("recharge_no"))
	if rechargeNo == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	recharge, err := h.wallets.GetRechargeOrderByRechargeNo(uid, rechargeNo)
	if err != nil {
		if errors.Is(err, ErrRechargeNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.payment_not_found", nil)
		} else {
			ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		}
		return
	}
	payment, err := h.payments.GetPayment(recharge.PaymentID)
	if err != nil {
		respondPaymentCaptureError(c, err)
		return
	}
	account, err := h.wallets.GetAccount(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, walletpresenter.NewWalletRechargePaymentPayload(recharge, payment, account))
}

func (h *UserHandler) ListRecharges(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	orders, total, err := h.wallets.ListUserRechargeOrders(uid, page, pageSize, strings.TrimSpace(c.Query("status")), strings.TrimSpace(c.Query("recharge_no")))
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, walletpresenter.NewWalletRechargeRespList(orders), response.BuildPagination(page, pageSize, total))
}

func (h *UserHandler) RechargeStats(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	stats, err := h.wallets.StatsUserRechargeOrders(uid, strings.TrimSpace(c.Query("recharge_no")))
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	var total int64
	for _, value := range stats {
		total += value
	}
	response.Success(c, gin.H{"total": total, "by_status": stats})
}

func (h *UserHandler) CaptureRechargePayment(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	paymentID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
		return
	}
	recharge, err := h.wallets.GetRechargeOrderByPaymentIDAndUser(paymentID, uid)
	if err != nil {
		if errors.Is(err, ErrRechargeNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.payment_not_found", nil)
		} else {
			ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		}
		return
	}
	payment, err := h.payments.CapturePayment(CapturePaymentInput{PaymentID: paymentID, Context: c.Request.Context()})
	if err != nil {
		if !errors.Is(err, ErrPaymentProviderNotSupported) {
			respondPaymentCaptureError(c, err)
			return
		}
		payment, err = h.payments.GetPayment(paymentID)
		if err != nil {
			respondPaymentCaptureError(c, err)
			return
		}
	}
	recharge, err = h.wallets.GetRechargeOrderByRechargeNo(uid, recharge.RechargeNo)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	account, err := h.wallets.GetAccount(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, walletpresenter.NewWalletRechargePaymentPayload(recharge, payment, account))
}

func respondPaymentCreateError(c *gin.Context, err error) {
	respondPaymentError(c, err, response.CodeInternal, "error.payment_create_failed", false)
}

func respondPaymentCaptureError(c *gin.Context, err error) {
	respondPaymentError(c, err, response.CodeInternal, "error.payment_callback_failed", true)
}

func respondPaymentError(c *gin.Context, err error, fallbackCode int, fallbackKey string, capture bool) {
	switch {
	case errors.Is(err, ErrPaymentInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_invalid", nil)
	case errors.Is(err, ErrPaymentNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.payment_not_found", nil)
	case errors.Is(err, ErrOrderNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
	case !capture && errors.Is(err, ErrOrderStatusInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
	case errors.Is(err, ErrPaymentChannelNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.payment_channel_not_found", nil)
	case errors.Is(err, ErrPaymentChannelInactive):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_channel_inactive", nil)
	case errors.Is(err, ErrPaymentProviderNotSupported):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_provider_not_supported", nil)
	case errors.Is(err, ErrPaymentChannelConfigInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_channel_config_invalid", nil)
	case errors.Is(err, ErrPaymentGatewayRequestFailed):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_gateway_request_failed", nil)
	case errors.Is(err, ErrPaymentGatewayResponseInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_gateway_response_invalid", nil)
	case errors.Is(err, ErrPaymentCurrencyMismatch):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_currency_mismatch", nil)
	case errors.Is(err, ErrPaymentChannelNotAllowedRecharge):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_channel_not_allowed_for_recharge", nil)
	case !capture && errors.Is(err, ErrPaymentChannelNotAllowedProduct):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_channel_not_allowed_for_product", nil)
	case !capture && errors.Is(err, ErrWalletOnlyPaymentRequired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.wallet_only_payment_required", nil)
	case capture && errors.Is(err, ErrPaymentStatusInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_status_invalid", nil)
	case capture && errors.Is(err, ErrPaymentAmountMismatch):
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_amount_mismatch", nil)
	default:
		ginutil.RespondError(c, fallbackCode, fallbackKey, err)
	}
}
