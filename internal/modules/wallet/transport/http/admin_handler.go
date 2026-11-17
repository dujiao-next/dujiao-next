package wallethttp

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
)

var ErrInsufficientBalance = errors.New("wallet insufficient balance")
var ErrRechargeRefundExceeded = errors.New("recharge refund exceeded")
var ErrRechargeStatusInvalid = errors.New("recharge status invalid")
var ErrRechargeRefundExpired = errors.New("recharge refund expired")

// AdminWalletService 是后台钱包管理所需的最小端口。
type AdminWalletService interface {
	GetAccount(userID uint) (*walletdomain.Account, error)
	ListAdminTransactions(userID uint, page, pageSize int, typ, direction string) ([]walletdomain.Transaction, int64, error)
	ListRechargeOrdersAdmin(filter AdminRechargeListFilter) ([]walletdomain.RechargeOrder, int64, error)
	AdminAdjustBalance(input AdjustBalanceInput) (*walletdomain.Account, *walletdomain.Transaction, error)
	GetRechargeDetail(id uint) (*walletdomain.RechargeOrder, *paymentdomain.Payment, error)
	ListRechargeRefunds(id uint) ([]walletdomain.Transaction, error)
	IsRechargeRefundWindowExpired(recharge *walletdomain.RechargeOrder) (bool, error)
	ConfirmRecharge(input ConfirmRechargeInput) error
	RefundRecharge(input RechargeRefundInput) error
}

// AdminUserReader 是后台钱包所需的用户读取端口。
type AdminUserReader interface {
	GetByID(id uint) (*userdomain.User, error)
	ListByIDs(ids []uint) ([]userdomain.User, error)
}

// PaymentChannelReader 读取支付渠道名称。
type PaymentChannelReader interface {
	ListByIDs(ids []uint) ([]paymentdomain.PaymentChannel, error)
}

// PaymentReader 读取支付状态。
type PaymentReader interface {
	GetByIDs(ids []uint) ([]paymentdomain.Payment, error)
}

// AdminRechargeListFilter 管理端充值单列表过滤条件。
type AdminRechargeListFilter struct {
	Page         int
	PageSize     int
	RechargeNo   string
	UserID       uint
	UserKeyword  string
	PaymentID    uint
	ChannelID    uint
	ProviderType string
	ChannelType  string
	Status       string
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	PaidFrom     *time.Time
	PaidTo       *time.Time
}

// AdjustBalanceInput 管理端余额调整输入。
type AdjustBalanceInput struct {
	UserID          uint
	OperatorAdminID uint
	Delta           money.Amount
	Currency        string
	Remark          string
}

// ConfirmRechargeInput 管理端人工确认充值支付的输入。
type ConfirmRechargeInput struct {
	RechargeID  uint
	PaymentID   uint
	AdminID     uint
	ProviderRef string
	Remark      string
}

// RechargeRefundInput 管理端登记充值手动退款的输入。
type RechargeRefundInput struct {
	PaymentFeeRefunded bool
	RechargeID         uint
	AdminID            uint
	Amount             money.Amount
	Remark             string
	RequestID          string
}

// AdminAdjustUserWalletRequest 管理端用户余额调整请求
type AdminAdjustUserWalletRequest struct {
	Amount    string `json:"amount" binding:"required"`
	Operation string `json:"operation"` // add/subtract
	Currency  string `json:"currency"`
	Remark    string `json:"remark"`
}

// AdminConfirmRechargeRequest 管理端人工确认充值支付的请求。
type AdminConfirmRechargeRequest struct {
	PaymentID   uint   `json:"payment_id" binding:"required"`
	ProviderRef string `json:"provider_ref" binding:"max=255"`
	Remark      string `json:"remark" binding:"max=500"`
}

// AdminRefundRechargeRequest 管理端登记充值手动退款的请求。
type AdminRefundRechargeRequest struct {
	PaymentFeeRefunded bool   `json:"payment_fee_refunded"`
	Amount             string `json:"amount" binding:"required,max=32"`
	Remark             string `json:"remark" binding:"max=255"`
	RequestID          string `json:"request_id" binding:"required,min=16,max=64,alphanumunicode|uuid"`
}

// adminWalletRechargeUser 后台充值记录关联的用户信息。
type adminWalletRechargeUser struct {
	ID          uint   `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// adminWalletRechargeItem 后台充值记录的展示信息。
type adminWalletRechargeItem struct {
	walletdomain.RechargeOrder
	User          *adminWalletRechargeUser `json:"user,omitempty"`
	ChannelName   string                   `json:"channel_name,omitempty"`
	PaymentStatus string                   `json:"payment_status,omitempty"`
}

// AdminHandler 处理后台钱包 HTTP 请求。
type AdminHandler struct {
	wallets  AdminWalletService
	users    AdminUserReader
	channels PaymentChannelReader
	payments PaymentReader
	settings SiteCurrencyReader
}

// NewAdminHandler 创建后台钱包 Handler。
func NewAdminHandler(
	wallets AdminWalletService,
	users AdminUserReader,
	channels PaymentChannelReader,
	payments PaymentReader,
	settings SiteCurrencyReader,
) *AdminHandler {
	if wallets == nil || users == nil || channels == nil || payments == nil {
		panic("wallet admin handler: required dependency is nil")
	}
	return &AdminHandler{
		wallets:  wallets,
		users:    users,
		channels: channels,
		payments: payments,
		settings: settings,
	}
}

// GetUserWallet 管理端获取用户钱包信息
func (h *AdminHandler) GetUserWallet(c *gin.Context) {
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
	account, err := h.wallets.GetAccount(userID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, gin.H{
		"user":    user,
		"account": account,
	})
}

// GetUserTransactions 管理端获取用户钱包流水
func (h *AdminHandler) GetUserTransactions(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}
	page, pageSize := ginutil.ParsePagination(c)

	transactions, total, err := h.wallets.ListAdminTransactions(
		userID,
		page,
		pageSize,
		strings.TrimSpace(c.Query("type")),
		strings.TrimSpace(c.Query("direction")),
	)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, transactions, pagination)
}

// GetRecharges 管理端分页获取钱包充值记录
func (h *AdminHandler) GetRecharges(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	userID, err := ginutil.ParseQueryUint(c.Query("user_id"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	paymentID, err := ginutil.ParseQueryUint(c.Query("payment_id"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	channelID, err := ginutil.ParseQueryUint(c.Query("channel_id"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	paidFrom, paidTo, err := ginutil.ParseQueryTimeRange(c, "paid_from", "paid_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	recharges, total, err := h.wallets.ListRechargeOrdersAdmin(AdminRechargeListFilter{
		Page:         page,
		PageSize:     pageSize,
		RechargeNo:   strings.TrimSpace(c.Query("recharge_no")),
		UserID:       userID,
		UserKeyword:  strings.TrimSpace(c.Query("user_keyword")),
		PaymentID:    paymentID,
		ChannelID:    channelID,
		ProviderType: strings.TrimSpace(strings.ToLower(c.Query("provider_type"))),
		ChannelType:  strings.TrimSpace(strings.ToLower(c.Query("channel_type"))),
		Status:       strings.TrimSpace(strings.ToLower(c.Query("status"))),
		CreatedFrom:  createdFrom,
		CreatedTo:    createdTo,
		PaidFrom:     paidFrom,
		PaidTo:       paidTo,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}

	userIDs := make([]uint, 0, len(recharges))
	channelIDs := make([]uint, 0, len(recharges))
	paymentIDs := make([]uint, 0, len(recharges))
	seenUsers := make(map[uint]struct{})
	seenChannels := make(map[uint]struct{})
	seenPayments := make(map[uint]struct{})
	for _, recharge := range recharges {
		if recharge.UserID != 0 {
			if _, ok := seenUsers[recharge.UserID]; !ok {
				seenUsers[recharge.UserID] = struct{}{}
				userIDs = append(userIDs, recharge.UserID)
			}
		}
		if recharge.ChannelID != 0 {
			if _, ok := seenChannels[recharge.ChannelID]; !ok {
				seenChannels[recharge.ChannelID] = struct{}{}
				channelIDs = append(channelIDs, recharge.ChannelID)
			}
		}
		if recharge.PaymentID != 0 {
			if _, ok := seenPayments[recharge.PaymentID]; !ok {
				seenPayments[recharge.PaymentID] = struct{}{}
				paymentIDs = append(paymentIDs, recharge.PaymentID)
			}
		}
	}

	userMap := make(map[uint]userdomain.User, len(userIDs))
	if len(userIDs) > 0 {
		users, userErr := h.users.ListByIDs(userIDs)
		if userErr != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", userErr)
			return
		}
		for _, user := range users {
			userMap[user.ID] = user
		}
	}

	channelNameMap := make(map[uint]string, len(channelIDs))
	if len(channelIDs) > 0 {
		channels, channelErr := h.channels.ListByIDs(channelIDs)
		if channelErr != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", channelErr)
			return
		}
		for _, channel := range channels {
			channelNameMap[channel.ID] = channel.Name
		}
	}

	paymentStatusMap := make(map[uint]string, len(paymentIDs))
	if len(paymentIDs) > 0 {
		payments, paymentErr := h.payments.GetByIDs(paymentIDs)
		if paymentErr != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", paymentErr)
			return
		}
		for _, payment := range payments {
			paymentStatusMap[payment.ID] = payment.Status
		}
	}

	items := make([]adminWalletRechargeItem, 0, len(recharges))
	for _, recharge := range recharges {
		item := adminWalletRechargeItem{
			RechargeOrder: recharge,
			ChannelName:   channelNameMap[recharge.ChannelID],
			PaymentStatus: paymentStatusMap[recharge.PaymentID],
		}
		if user, ok := userMap[recharge.UserID]; ok {
			item.User = &adminWalletRechargeUser{
				ID:          user.ID,
				Email:       user.Email,
				DisplayName: user.DisplayName,
			}
		}
		items = append(items, item)
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, items, pagination)
}

// AdjustUserWallet 管理端增减用户余额
func (h *AdminHandler) AdjustUserWallet(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}
	var req AdminAdjustUserWalletRequest
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
	op := strings.ToLower(strings.TrimSpace(req.Operation))
	delta := amount
	if op == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if op == "subtract" {
		delta = amount.Neg()
	}
	if op != "add" && op != "subtract" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.wallet_adjust_remark_required", nil)
		return
	}
	currency := strings.TrimSpace(req.Currency)
	if currency == "" && h.settings != nil {
		siteCurrency, currencyErr := h.settings.GetSiteCurrency(constants.SiteCurrencyDefault)
		if currencyErr == nil {
			currency = siteCurrency
		}
	}

	account, txn, err := h.wallets.AdminAdjustBalance(AdjustBalanceInput{
		UserID:          userID,
		OperatorAdminID: adminID,
		Delta:           money.FromDecimal(delta),
		Currency:        currency,
		Remark:          remark,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidAmount):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		case errors.Is(err, ErrInsufficientBalance):
			ginutil.RespondError(c, response.CodeBadRequest, "error.payment_amount_mismatch", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"account":     account,
		"transaction": txn,
	})
}

// GetRechargeDetail 管理端获取充值单详情、关联支付及退款记录。
func (h *AdminHandler) GetRechargeDetail(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	recharge, payment, err := h.wallets.GetRechargeDetail(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_fetch_failed", err)
		return
	}
	item := adminWalletRechargeItem{RechargeOrder: *recharge}
	users, err := h.users.ListByIDs([]uint{recharge.UserID})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	for _, user := range users {
		item.User = &adminWalletRechargeUser{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName}
	}
	channels, err := h.channels.ListByIDs([]uint{recharge.ChannelID})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	for _, channel := range channels {
		item.ChannelName = channel.Name
	}
	if payment != nil {
		copy := *payment
		copy.ProviderPayload = nil
		copy.PayURL = ""
		copy.QRCode = ""
		payment = &copy
		item.PaymentStatus = payment.Status
	}
	refunds, err := h.wallets.ListRechargeRefunds(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	refundable := decimal.Zero
	refundWindowExpired, err := h.wallets.IsRechargeRefundWindowExpired(recharge)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.payment_fetch_failed", err)
		return
	}
	if !refundWindowExpired && (recharge.Status == constants.WalletRechargeStatusSuccess || recharge.Status == constants.WalletRechargeStatusPartiallyRefunded) {
		account, err := h.wallets.GetAccount(recharge.UserID)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
			return
		}
		if account != nil {
			refundable = decimal.Max(decimal.Zero, decimal.Min(recharge.Amount.Decimal.Sub(recharge.RefundedAmount.Decimal), account.Balance.Decimal))
		}
	}
	response.Success(c, gin.H{"recharge": item, "payment": payment, "refunds": refunds, "refundable_amount": money.FromDecimal(refundable), "refund_window_expired": refundWindowExpired})
}

// ConfirmRecharge 管理端核实渠道到账后人工确认充值支付。
func (h *AdminHandler) ConfirmRecharge(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var req AdminConfirmRechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if err := h.wallets.ConfirmRecharge(ConfirmRechargeInput{RechargeID: id, PaymentID: req.PaymentID, AdminID: adminID, ProviderRef: req.ProviderRef, Remark: req.Remark}); err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.payment_update_failed", err)
		return
	}
	response.Success(c, nil)
}

// RefundRecharge 管理端登记充值手动退款，并同步扣回用户钱包余额。
func (h *AdminHandler) RefundRecharge(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var req AdminRefundRechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || !amount.GreaterThan(decimal.Zero) || !amount.Equal(amount.Round(2)) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	err = h.wallets.RefundRecharge(RechargeRefundInput{PaymentFeeRefunded: req.PaymentFeeRefunded, RechargeID: id, AdminID: adminID, Amount: money.FromDecimal(amount), Remark: req.Remark, RequestID: req.RequestID})
	if err != nil {
		key := "error.payment_update_failed"
		switch {
		case errors.Is(err, ErrInsufficientBalance):
			key = "error.recharge_refund_insufficient"
		case errors.Is(err, ErrRechargeRefundExceeded):
			key = "error.recharge_refund_exceeded"
		case errors.Is(err, ErrRechargeStatusInvalid):
			key = "error.recharge_status_invalid"
		case errors.Is(err, ErrRechargeRefundExpired):
			key = "error.order_refund_expired"
		case errors.Is(err, ErrInvalidAmount):
			key = "error.bad_request"
		}
		ginutil.RespondError(c, response.CodeBadRequest, key, err)
		return
	}
	response.Success(c, nil)
}
