package channelhttp

import (
	"errors"
	"strings"
	"time"

	paymentpresenter "github.com/dujiao-next/internal/modules/payment/transport/presenter"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type createPaymentRequest struct {
	ChannelUserID  string `json:"channel_user_id"`
	TelegramUserID string `json:"telegram_user_id"`
	OrderID        uint   `json:"order_id" binding:"required"`
	ChannelID      uint   `json:"channel_id"`
	UseBalance     bool   `json:"use_balance"`
}

type latestPaymentQuery struct {
	OrderID uint `form:"order_id" binding:"required"`
}

// GetPaymentChannels GET /api/v1/channel/payment-channels
// 可选查询参数:
//   - order_no: 根据订单中的商品过滤允许的支付渠道
//   - context: "recharge" 返回钱包充值允许的支付渠道
func (h *Handler) GetPaymentChannels(c *gin.Context) {
	contextParam := strings.TrimSpace(c.Query("context"))
	orderNo := strings.TrimSpace(c.Query("order_no"))

	var channels []paymentdomain.PaymentChannel
	var err error

	if contextParam == "recharge" {
		// 钱包充值渠道
		channels, err = h.PaymentService.GetWalletRechargeChannels()
		if channelUserID := channelUserIDFromQuery(c); err == nil && channelUserID != "" {
			channels = h.filterWalletEligibleChannels(channels, channelUserID)
		}
	} else if orderNo != "" {
		// 按订单商品过滤渠道
		channelUserID := channelUserIDFromQuery(c)
		if channelUserID == "" {
			// 缺少 channel_user_id，无法查找订单，返回全部渠道
			channels, _, err = h.PaymentService.ListChannels(PaymentChannelListFilter{
				ActiveOnly: true,
				Page:       1,
				PageSize:   50,
			})
		} else {
			userID, resolveErr := h.provisionTelegramChannelUserID(TelegramIdentityInput{ChannelUserID: channelUserID})
			if resolveErr != nil {
				logger.Errorw("channel_payment_channels_resolve_user", "channel_user_id", channelUserID, "error", resolveErr)
				channels, _, err = h.PaymentService.ListChannels(PaymentChannelListFilter{
					ActiveOnly: true,
					Page:       1,
					PageSize:   50,
				})
			} else {
				order, orderErr := h.OrderService.GetOrderByUserOrderNo(orderNo, userID)
				if orderErr != nil || order == nil {
					// 找不到订单，返回全部渠道
					channels, _, err = h.PaymentService.ListChannels(PaymentChannelListFilter{
						ActiveOnly: true,
						Page:       1,
						PageSize:   50,
					})
				} else {
					allItems := order.Items
					for _, child := range order.Children {
						allItems = append(allItems, child.Items...)
					}
					productIDSet := make(map[uint]struct{})
					for _, item := range allItems {
						if item.ProductID > 0 {
							productIDSet[item.ProductID] = struct{}{}
						}
					}
					productIDs := make([]uint, 0, len(productIDSet))
					for id := range productIDSet {
						productIDs = append(productIDs, id)
					}
					channels, err = h.PaymentService.GetAllowedChannelsForProducts(productIDs)
					if err == nil {
						channels = h.PaymentService.FilterOrderEligibleChannels(channels, order)
					}
				}
			}
		}
	} else {
		// 默认：全部活跃渠道
		channels, _, err = h.PaymentService.ListChannels(PaymentChannelListFilter{
			ActiveOnly: true,
			Page:       1,
			PageSize:   50,
		})
	}

	if err != nil {
		logger.Errorw("channel_order_list_payment_channels", "error", err)
		respondChannelError(c, 500, 500, "internal_error", "error.internal_error", err)
		return
	}

	type channelItem struct {
		ID              uint   `json:"id"`
		Name            string `json:"name"`
		ProviderType    string `json:"provider_type"`
		ChannelType     string `json:"channel_type"`
		InteractionMode string `json:"interaction_mode"`
		FeeRate         string `json:"fee_rate"`
		FixedFee        string `json:"fixed_fee"`
	}

	items := make([]channelItem, 0, len(channels))
	for _, ch := range channels {
		if ch.ProviderType == "balance" || ch.ProviderType == "wallet" {
			continue
		}
		items = append(items, channelItem{
			ID:              ch.ID,
			Name:            ch.Name,
			ProviderType:    ch.ProviderType,
			ChannelType:     ch.ChannelType,
			InteractionMode: ch.InteractionMode,
			FeeRate:         ch.FeeRate.StringFixed(2),
			FixedFee:        ch.FixedFee.StringFixed(2),
		})
	}

	resp := gin.H{"items": items}
	if h.SettingService != nil && h.SettingService.GetWalletOnlyPayment() {
		resp["wallet_only_payment"] = true
	}
	respondChannelSuccess(c, resp)
}

// filterWalletEligibleChannels 按用户角色与会员等级过滤充值渠道；用户解析失败时保留原列表，创建充值时仍会校验。
func (h *Handler) filterWalletEligibleChannels(channels []paymentdomain.PaymentChannel, channelUserID string) []paymentdomain.PaymentChannel {
	userID, err := h.provisionTelegramChannelUserID(TelegramIdentityInput{ChannelUserID: channelUserID})
	if err != nil {
		logger.Errorw("channel_payment_channels_resolve_user", "channel_user_id", channelUserID, "error", err)
		return channels
	}
	filtered, err := h.PaymentService.FilterWalletEligibleChannels(channels, userID)
	if err != nil {
		logger.Errorw("channel_payment_channels_filter_wallet", "user_id", userID, "error", err)
		return channels
	}
	return filtered
}

// GetLatestPayment GET /api/v1/channel/payments/latest
func (h *Handler) GetLatestPayment(c *gin.Context) {
	var query latestPaymentQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		respondChannelBindError(c, err)
		return
	}

	channelUserID := channelUserIDFromQuery(c)
	if channelUserID == "" {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.provisionTelegramChannelUserID(TelegramIdentityInput{ChannelUserID: channelUserID})
	if err != nil {
		logger.Errorw("channel_payment_latest_resolve_user", "channel_user_id", channelUserID, "error", err)
		respondChannelIdentityServiceError(c, err)
		return
	}

	order, err := h.OrderService.GetOrderByUser(query.OrderID, userID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			respondChannelError(c, 404, response.CodeNotFound, "order_not_found", "error.order_not_found", nil)
			return
		}
		respondChannelError(c, 500, response.CodeInternal, "internal_error", "error.order_fetch_failed", err)
		return
	}
	if order == nil {
		respondChannelError(c, 404, response.CodeNotFound, "order_not_found", "error.order_not_found", nil)
		return
	}
	if order.ParentID != nil {
		respondChannelError(c, 400, response.CodeBadRequest, "payment_invalid", "error.payment_invalid", nil)
		return
	}
	if order.Status != constants.OrderStatusPendingPayment {
		respondChannelError(c, 400, response.CodeBadRequest, "order_status_invalid", "error.order_status_invalid", nil)
		return
	}
	if order.ExpiresAt != nil && !order.ExpiresAt.After(time.Now()) {
		respondChannelError(c, 400, response.CodeBadRequest, "order_status_invalid", "error.order_status_invalid", nil)
		return
	}

	payment, err := h.PaymentStore.GetLatestPendingByOrder(order.ID, time.Now())
	if err != nil {
		respondChannelError(c, 500, response.CodeInternal, "internal_error", "error.payment_fetch_failed", err)
		return
	}
	if payment == nil {
		respondChannelError(c, 404, response.CodeNotFound, "payment_not_found", "error.payment_not_found", nil)
		return
	}

	respondChannelSuccess(c, buildChannelPaymentResponse(order, payment))
}

// CreatePayment POST /api/v1/channel/payments
func (h *Handler) CreatePayment(c *gin.Context) {
	var req createPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondChannelBindError(c, err)
		return
	}

	channelUserID := channelUserIDValue(req.ChannelUserID, req.TelegramUserID)
	if channelUserID == "" {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.provisionTelegramChannelUserID(TelegramIdentityInput{ChannelUserID: channelUserID})
	if err != nil {
		logger.Errorw("channel_payment_resolve_user", "channel_user_id", channelUserID, "error", err)
		respondChannelIdentityServiceError(c, err)
		return
	}

	order, err := h.OrderService.GetOrderByUser(req.OrderID, userID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			respondChannelError(c, 404, response.CodeNotFound, "order_not_found", "error.order_not_found", nil)
			return
		}
		respondChannelError(c, 500, response.CodeInternal, "internal_error", "error.internal_error", err)
		return
	}

	result, err := h.PaymentService.CreatePayment(CreatePaymentInput{
		OrderID:    req.OrderID,
		ChannelID:  req.ChannelID,
		UseBalance: req.UseBalance,
		ClientIP:   c.ClientIP(),
		Context:    c.Request.Context(),
	})
	if err != nil {
		logger.Errorw("channel_order_create_payment", "order_id", req.OrderID, "error", err)
		respondChannelPaymentCreateError(c, err)
		return
	}

	resp := gin.H{
		"order_paid":         result.OrderPaid,
		"wallet_paid_amount": result.WalletPaidAmount.StringFixed(2),
		"online_pay_amount":  result.OnlinePayAmount.StringFixed(2),
	}
	if result.Payment != nil {
		for key, value := range buildChannelPaymentResponse(nil, result.Payment) {
			resp[key] = value
		}
	}
	if result.Channel != nil {
		resp["channel_name"] = result.Channel.Name
	}
	if order != nil {
		// 手续费按订单币种计算，换汇渠道下 currency 是结算币种，需单独告知
		resp["fee_currency"] = order.Currency
	}

	respondChannelSuccess(c, resp)
}

// GetPaymentDetail GET /api/v1/channel/payments/:id
func (h *Handler) GetPaymentDetail(c *gin.Context) {
	paymentID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	channelUserID := channelUserIDFromQuery(c)
	if channelUserID == "" {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.provisionTelegramChannelUserID(TelegramIdentityInput{ChannelUserID: channelUserID})
	if err != nil {
		logger.Errorw("channel_payment_detail_resolve_user", "channel_user_id", channelUserID, "error", err)
		respondChannelIdentityServiceError(c, err)
		return
	}

	payment, err := h.PaymentStore.GetByID(uint(paymentID))
	if err != nil {
		respondChannelError(c, 500, response.CodeInternal, "internal_error", "error.payment_fetch_failed", err)
		return
	}
	if payment == nil {
		respondChannelError(c, 404, response.CodeNotFound, "payment_not_found", "error.payment_not_found", nil)
		return
	}

	order, err := h.OrderService.GetOrderByUser(payment.OrderID, userID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			respondChannelError(c, 404, response.CodeNotFound, "payment_not_found", "error.payment_not_found", nil)
			return
		}
		respondChannelError(c, 500, response.CodeInternal, "internal_error", "error.order_fetch_failed", err)
		return
	}
	if order == nil {
		respondChannelError(c, 404, response.CodeNotFound, "payment_not_found", "error.payment_not_found", nil)
		return
	}

	respondChannelSuccess(c, buildChannelPaymentResponse(order, payment))
}

func buildChannelPaymentResponse(order *orderdomain.Order, payment *paymentdomain.Payment) gin.H {
	resp := gin.H{
		"payment_id":       payment.ID,
		"order_id":         payment.OrderID,
		"channel_id":       payment.ChannelID,
		"status":           payment.Status,
		"provider_type":    payment.ProviderType,
		"channel_type":     payment.ChannelType,
		"interaction_mode": payment.InteractionMode,
		"amount":           payment.Amount.StringFixed(2),
		"fee_rate":         payment.FeeRate.StringFixed(2),
		"fee_amount":       payment.FeeAmount.StringFixed(2),
		"fee_policy":       payment.FeePolicy,
		"currency":         payment.Currency,
		"pay_url":          payment.PayURL,
		"qr_code":          payment.QRCode,
		"paid_at":          payment.PaidAt,
		"expires_at":       payment.ExpiredAt,
		"callback_at":      payment.CallbackAt,
		"created_at":       payment.CreatedAt,
		"updated_at":       payment.UpdatedAt,
	}
	if info := paymentpresenter.ExtractCryptoWalletInfo(payment.ProviderType, payment.InteractionMode, payment.ProviderPayload); info.HasAny() {
		if info.Address != "" {
			resp["wallet_address"] = info.Address
		}
		if info.ChainAmount != "" {
			resp["chain_amount"] = info.ChainAmount
		}
		if info.Chain != "" {
			resp["chain"] = info.Chain
		}
		if info.TokenID != "" {
			resp["token_id"] = info.TokenID
		}
	}
	if order != nil {
		resp["order_no"] = order.OrderNo
		resp["fee_currency"] = order.Currency
		resp["total_amount"] = order.TotalAmount.StringFixed(2)
		resp["wallet_paid_amount"] = order.WalletPaidAmount.StringFixed(2)
		resp["online_paid_amount"] = order.OnlinePaidAmount.StringFixed(2)
		resp["paid_amount"] = channelOrderPaidAmount(order)
	}
	return resp
}
