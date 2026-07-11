package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/payment/provider"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/repository"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PaymentService 支付服务
type PaymentService struct {
	orderRepo               repository.OrderRepository
	productRepo             repository.ProductRepository
	productSKURepo          repository.ProductSKURepository
	paymentRepo             repository.PaymentRepository
	channelRepo             repository.PaymentChannelRepository
	walletRepo              repository.WalletRepository
	userRepo                repository.UserRepository
	userOAuthIdentityRepo   repository.UserOAuthIdentityRepository
	queueClient             *queue.Client
	walletSvc               *WalletService
	settingService          *SettingService
	defaultEmailConfig      config.EmailConfig
	expireMinutes           int
	affiliateSvc            *AffiliateService
	notificationSvc         *NotificationService
	procurementSvc          *ProcurementOrderService
	downstreamCallbackSvc   *DownstreamCallbackService
	memberLevelSvc          *MemberLevelService
	paymentProviderRegistry *provider.Registry
	resellerAccountingSvc   *ResellerAccountingService
}

// SetProcurementService 设置采购单服务（解决循环依赖）
func (s *PaymentService) SetProcurementService(svc *ProcurementOrderService) {
	s.procurementSvc = svc
}

// SetDownstreamCallbackService 设置下游回调服务（解决循环依赖）
func (s *PaymentService) SetDownstreamCallbackService(svc *DownstreamCallbackService) {
	s.downstreamCallbackSvc = svc
}

// SetMemberLevelService 设置会员等级服务
func (s *PaymentService) SetMemberLevelService(svc *MemberLevelService) {
	s.memberLevelSvc = svc
}

// PaymentServiceOptions 支付服务构造参数
type PaymentServiceOptions struct {
	OrderRepo                 repository.OrderRepository
	ProductRepo               repository.ProductRepository
	ProductSKURepo            repository.ProductSKURepository
	PaymentRepo               repository.PaymentRepository
	ChannelRepo               repository.PaymentChannelRepository
	WalletRepo                repository.WalletRepository
	UserRepo                  repository.UserRepository
	UserOAuthIdentityRepo     repository.UserOAuthIdentityRepository
	QueueClient               *queue.Client
	WalletService             *WalletService
	SettingService            *SettingService
	DefaultEmailConfig        config.EmailConfig
	ExpireMinutes             int
	AffiliateService          *AffiliateService
	NotificationService       *NotificationService
	PaymentProviderRegistry   *provider.Registry
	ResellerAccountingService *ResellerAccountingService
}

// NewPaymentService 创建支付服务
func NewPaymentService(opts PaymentServiceOptions) *PaymentService {
	return &PaymentService{
		orderRepo:               opts.OrderRepo,
		productRepo:             opts.ProductRepo,
		productSKURepo:          opts.ProductSKURepo,
		paymentRepo:             opts.PaymentRepo,
		channelRepo:             opts.ChannelRepo,
		walletRepo:              opts.WalletRepo,
		userRepo:                opts.UserRepo,
		userOAuthIdentityRepo:   opts.UserOAuthIdentityRepo,
		queueClient:             opts.QueueClient,
		walletSvc:               opts.WalletService,
		settingService:          opts.SettingService,
		defaultEmailConfig:      opts.DefaultEmailConfig,
		expireMinutes:           opts.ExpireMinutes,
		affiliateSvc:            opts.AffiliateService,
		notificationSvc:         opts.NotificationService,
		paymentProviderRegistry: opts.PaymentProviderRegistry,
		resellerAccountingSvc:   opts.ResellerAccountingService,
	}
}

// CreatePaymentInput 创建支付请求
type CreatePaymentInput struct {
	OrderID          uint
	ChannelID        uint
	UseBalance       bool
	ClientIP         string
	Context          context.Context
	ReturnBizType    string
	ReturnBusinessNo string
	ReturnGuest      bool
}

// CreatePaymentResult 创建支付结果
type CreatePaymentResult struct {
	Payment          *models.Payment
	Channel          *models.PaymentChannel
	OrderPaid        bool
	WalletPaidAmount models.Money
	OnlinePayAmount  models.Money
}

// CreateWalletRechargePaymentInput 创建钱包充值支付请求
type CreateWalletRechargePaymentInput struct {
	UserID    uint
	ChannelID uint
	Amount    models.Money
	Currency  string
	Remark    string
	ClientIP  string
	Context   context.Context
}

// CreateWalletRechargePaymentResult 创建钱包充值支付结果
type CreateWalletRechargePaymentResult struct {
	Recharge *models.WalletRechargeOrder
	Payment  *models.Payment
}

func hasProviderResult(payment *models.Payment) bool {
	if payment == nil {
		return false
	}
	if strings.TrimSpace(payment.PayURL) != "" || strings.TrimSpace(payment.QRCode) != "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(payment.ProviderType), constants.PaymentProviderBepusdt) &&
		strings.EqualFold(strings.TrimSpace(payment.InteractionMode), constants.PaymentInteractionQR) &&
		strings.TrimSpace(payment.ProviderRef) != ""
}

func shouldMarkFulfilling(order *models.Order) bool {
	if order == nil {
		return false
	}
	if len(order.Items) == 0 {
		return false
	}
	for _, item := range order.Items {
		fulfillmentType := strings.TrimSpace(item.FulfillmentType)
		if fulfillmentType == "" || fulfillmentType == constants.FulfillmentTypeManual || fulfillmentType == constants.FulfillmentTypeUpstream {
			return true
		}
	}
	return false
}

func paymentLogger(kv ...interface{}) *zap.SugaredLogger {
	if len(kv) == 0 {
		return logger.S()
	}
	return logger.SW(kv...)
}

// PaymentCallbackInput 支付回调输入
type PaymentCallbackInput struct {
	PaymentID   uint
	OrderNo     string
	ChannelID   uint
	Status      string
	ProviderRef string
	Amount      models.Money
	Currency    string
	PaidAt      *time.Time
	Payload     models.JSON
}

// CapturePaymentInput 捕获支付输入。
type CapturePaymentInput struct {
	PaymentID uint
	Context   context.Context
}

// WebhookCallbackInput Webhook 回调输入。
type WebhookCallbackInput struct {
	ChannelID uint
	Headers   map[string]string
	Body      []byte
	Context   context.Context
}

// CreatePayment 创建支付单
func (s *PaymentService) CreatePayment(input CreatePaymentInput) (*CreatePaymentResult, error) {
	if input.OrderID == 0 {
		return nil, ErrPaymentInvalid
	}

	log := paymentLogger(
		"order_id", input.OrderID,
		"channel_id", input.ChannelID,
	)

	var payment *models.Payment
	var order *models.Order
	var channel *models.PaymentChannel
	feeRate := decimal.Zero
	reusedPending := false
	orderPaidByWallet := false
	now := time.Now()

	// 在事务外查询设置，避免 SQLite 单连接池下自锁
	walletOnly := s.settingService != nil && s.settingService.GetWalletOnlyPayment()
	if walletOnly {
		input.UseBalance = true
		if input.ChannelID != 0 {
			return nil, ErrWalletOnlyPaymentRequired
		}
	}

	err := s.paymentRepo.Transaction(func(tx *gorm.DB) error {
		preloaded, err := s.orderRepo.WithTx(tx).GetByIDForUpdateWithChildren(input.OrderID)
		if err != nil {
			return ErrOrderFetchFailed
		}
		if preloaded == nil {
			return ErrOrderNotFound
		}
		lockedOrder := *preloaded
		if lockedOrder.ParentID != nil {
			return ErrPaymentInvalid
		}
		if lockedOrder.Status != constants.OrderStatusPendingPayment {
			return ErrOrderStatusInvalid
		}
		if lockedOrder.ExpiresAt != nil && !lockedOrder.ExpiresAt.After(time.Now()) {
			return ErrOrderStatusInvalid
		}

		paymentRepo := s.paymentRepo.WithTx(tx)
		channelRepo := s.channelRepo.WithTx(tx)
		if input.ChannelID != 0 {
			if channel == nil {
				// 事务内必须使用 tx 绑定仓储，避免在单连接池下发生自锁等待。
				resolvedChannel, err := channelRepo.GetByID(input.ChannelID)
				if err != nil {
					return err
				}
				if resolvedChannel == nil {
					return ErrPaymentChannelNotFound
				}
				if !resolvedChannel.IsActive {
					return ErrPaymentChannelInactive
				}
				resolvedFeeRate := resolvedChannel.FeeRate.Decimal.Round(2)
				if resolvedFeeRate.LessThan(decimal.Zero) || resolvedFeeRate.GreaterThan(decimal.NewFromInt(100)) {
					return ErrPaymentChannelConfigInvalid
				}
				channel = resolvedChannel
				feeRate = resolvedFeeRate
			}

			// 校验商品是否允许该支付渠道（传入 tx 避免 SQLite 自锁）
			allItems := lockedOrder.Items
			for _, child := range lockedOrder.Children {
				allItems = append(allItems, child.Items...)
			}
			if err := s.validateProductPaymentChannel(allItems, channel.ID, tx); err != nil {
				return err
			}

			existing, err := paymentRepo.GetLatestPendingByOrderChannel(lockedOrder.ID, channel.ID, time.Now())
			if err != nil {
				return ErrPaymentCreateFailed
			}
			if existing != nil && hasProviderResult(existing) {
				reusedPending = true
				payment = existing
				order = &lockedOrder
				return nil
			}
		}

		if s.walletSvc != nil {
			if input.UseBalance {
				if _, err := s.walletSvc.ApplyOrderBalance(tx, &lockedOrder, true); err != nil {
					return err
				}
			} else if lockedOrder.WalletPaidAmount.Decimal.GreaterThan(decimal.Zero) {
				if _, err := s.walletSvc.ReleaseOrderBalance(tx, &lockedOrder, constants.WalletTxnTypeOrderRefund, "用户改为在线支付，退回余额"); err != nil {
					return err
				}
			}
		}

		onlineAmount := normalizeOrderAmount(lockedOrder.TotalAmount.Decimal.Sub(lockedOrder.WalletPaidAmount.Decimal))
		if onlineAmount.LessThanOrEqual(decimal.Zero) {
			walletPaidAmount := normalizeOrderAmount(lockedOrder.WalletPaidAmount.Decimal)
			paidAt := time.Now()
			payment = &models.Payment{
				OrderID:         lockedOrder.ID,
				ChannelID:       0,
				ProviderType:    constants.PaymentProviderWallet,
				ChannelType:     constants.PaymentChannelTypeBalance,
				InteractionMode: constants.PaymentInteractionBalance,
				Amount:          models.NewMoneyFromDecimal(walletPaidAmount),
				FeeRate:         models.NewMoneyFromDecimal(decimal.Zero),
				FixedFee:        models.NewMoneyFromDecimal(decimal.Zero),
				FeeAmount:       models.NewMoneyFromDecimal(decimal.Zero),
				Currency:        lockedOrder.Currency,
				Status:          constants.PaymentStatusSuccess,
				CreatedAt:       paidAt,
				UpdatedAt:       paidAt,
				PaidAt:          &paidAt,
			}
			if err := paymentRepo.Create(payment); err != nil {
				return ErrPaymentCreateFailed
			}
			if err := s.markOrderPaid(tx, &lockedOrder, paidAt); err != nil {
				return err
			}
			orderPaidByWallet = true
			order = &lockedOrder
			return nil
		}
		if channel == nil {
			if walletOnly {
				return ErrWalletOnlyPaymentRequired
			}
			return ErrPaymentInvalid
		}
		if err := validatePaymentCurrencyForChannel(lockedOrder.Currency, channel); err != nil {
			return err
		}
		if err := validatePaymentAmountForChannel(onlineAmount, channel); err != nil {
			return err
		}

		fixedFee := decimal.Zero
		if channel.FixedFee.Decimal.GreaterThan(decimal.Zero) {
			fixedFee = channel.FixedFee.Decimal.Round(2)
		}

		feeAmount := fixedFee
		if feeRate.GreaterThan(decimal.Zero) {
			feeAmount = feeAmount.Add(onlineAmount.Mul(feeRate).Div(decimal.NewFromInt(100))).Round(2)
		}
		payableAmount := onlineAmount.Add(feeAmount).Round(2)
		payment = &models.Payment{
			OrderID:         lockedOrder.ID,
			ChannelID:       channel.ID,
			ProviderType:    channel.ProviderType,
			ChannelType:     channel.ChannelType,
			InteractionMode: channel.InteractionMode,
			Amount:          models.NewMoneyFromDecimal(payableAmount),
			FeeRate:         models.NewMoneyFromDecimal(feeRate),
			FixedFee:        models.NewMoneyFromDecimal(fixedFee),
			FeeAmount:       models.NewMoneyFromDecimal(feeAmount),
			Currency:        lockedOrder.Currency,
			Status:          constants.PaymentStatusInitiated,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if shouldUseCNYPaymentCurrency(channel) {
			payment.Currency = "CNY"
		}

		if err := paymentRepo.Create(payment); err != nil {
			return ErrPaymentCreateFailed
		}
		if err := s.orderRepo.WithTx(tx).UpdateFields(lockedOrder.ID, map[string]interface{}{
			"online_paid_amount": models.NewMoneyFromDecimal(onlineAmount),
			"updated_at":         time.Now(),
		}); err != nil {
			return ErrOrderUpdateFailed
		}
		lockedOrder.OnlinePaidAmount = models.NewMoneyFromDecimal(onlineAmount)
		lockedOrder.UpdatedAt = time.Now()
		order = &lockedOrder
		return nil
	})
	if err != nil {
		return nil, err
	}

	if order == nil {
		return nil, ErrOrderFetchFailed
	}

	if reusedPending {
		log.Infow("payment_create_reuse_pending",
			"payment_id", payment.ID,
			"provider_type", payment.ProviderType,
			"channel_type", payment.ChannelType,
		)
		return &CreatePaymentResult{
			Payment:          payment,
			Channel:          channel,
			WalletPaidAmount: order.WalletPaidAmount,
			OnlinePayAmount:  order.OnlinePaidAmount,
		}, nil
	}

	if orderPaidByWallet {
		log.Infow("payment_create_wallet_success",
			"payment_id", payment.ID,
			"provider_type", payment.ProviderType,
			"channel_type", payment.ChannelType,
			"interaction_mode", payment.InteractionMode,
			"currency", payment.Currency,
			"amount", payment.Amount.String(),
			"wallet_paid_amount", order.WalletPaidAmount.String(),
			"online_pay_amount", order.OnlinePaidAmount.String(),
		)
		s.enqueueOrderPaidAsync(order, payment, log)
		return &CreatePaymentResult{
			Payment:          nil,
			Channel:          nil,
			OrderPaid:        true,
			WalletPaidAmount: order.WalletPaidAmount,
			OnlinePayAmount:  models.NewMoneyFromDecimal(decimal.Zero),
		}, nil
	}

	if payment == nil {
		return nil, ErrPaymentCreateFailed
	}

	if err := s.applyProviderPayment(input, order, channel, payment); err != nil {
		rollbackErr := s.paymentRepo.Transaction(func(tx *gorm.DB) error {
			paymentRepo := s.paymentRepo.WithTx(tx)
			payment.Status = constants.PaymentStatusFailed
			payment.UpdatedAt = time.Now()
			if updateErr := paymentRepo.Update(payment); updateErr != nil {
				return updateErr
			}
			if s.walletSvc == nil {
				return nil
			}
			preloaded, findErr := s.orderRepo.WithTx(tx).GetByIDForUpdate(order.ID)
			if findErr != nil {
				return findErr
			}
			if preloaded == nil {
				return ErrOrderNotFound
			}
			lockedOrder := *preloaded
			_, refundErr := s.walletSvc.ReleaseOrderBalance(tx, &lockedOrder, constants.WalletTxnTypeOrderRefund, "在线支付创建失败，退回余额")
			return refundErr
		})
		if rollbackErr != nil {
			log.Errorw("payment_create_provider_failed_with_rollback_error",
				"payment_id", payment.ID,
				"order_id", order.ID,
				"provider_type", payment.ProviderType,
				"channel_type", payment.ChannelType,
				"provider_error", err,
				"rollback_error", rollbackErr,
			)
		} else {
			log.Errorw("payment_create_provider_failed",
				"payment_id", payment.ID,
				"provider_type", payment.ProviderType,
				"channel_type", payment.ChannelType,
				"error", err,
			)
		}
		return nil, err
	}

	log.Infow("payment_create_success",
		"payment_id", payment.ID,
		"provider_type", payment.ProviderType,
		"channel_type", payment.ChannelType,
		"interaction_mode", payment.InteractionMode,
		"currency", payment.Currency,
		"amount", payment.Amount.String(),
		"wallet_paid_amount", order.WalletPaidAmount.String(),
		"online_pay_amount", order.OnlinePaidAmount.String(),
	)

	return &CreatePaymentResult{
		Payment:          payment,
		Channel:          channel,
		WalletPaidAmount: order.WalletPaidAmount,
		OnlinePayAmount:  order.OnlinePaidAmount,
	}, nil
}

// CreateWalletRechargePayment 创建钱包充值支付单
func (s *PaymentService) CreateWalletRechargePayment(input CreateWalletRechargePaymentInput) (*CreateWalletRechargePaymentResult, error) {
	if input.UserID == 0 || input.ChannelID == 0 {
		return nil, ErrPaymentInvalid
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrWalletInvalidAmount
	}
	if s.walletRepo == nil {
		return nil, ErrPaymentCreateFailed
	}

	channel, err := s.channelRepo.GetByID(input.ChannelID)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, ErrPaymentChannelNotFound
	}
	if !channel.IsActive {
		return nil, ErrPaymentChannelInactive
	}

	// 校验钱包充值是否允许该支付渠道
	if err := s.validateWalletRechargeChannel(channel.ID); err != nil {
		return nil, err
	}

	feeRate := channel.FeeRate.Decimal.Round(2)
	if feeRate.LessThan(decimal.Zero) || feeRate.GreaterThan(decimal.NewFromInt(100)) {
		return nil, ErrPaymentChannelConfigInvalid
	}
	fixedFee := channel.FixedFee.Decimal.Round(2)
	if fixedFee.LessThan(decimal.Zero) || fixedFee.GreaterThanOrEqual(decimal.NewFromInt(10000)) {
		return nil, ErrPaymentChannelConfigInvalid
	}
	if err := validatePaymentAmountForChannel(amount, channel); err != nil {
		return nil, err
	}

	feeAmount := fixedFee
	if feeRate.GreaterThan(decimal.Zero) {
		feeAmount = feeAmount.Add(amount.Mul(feeRate).Div(decimal.NewFromInt(100))).Round(2)
	}
	payableAmount := amount.Add(feeAmount).Round(2)
	currency := normalizeWalletCurrency(input.Currency)
	if err := validatePaymentCurrencyForChannel(currency, channel); err != nil {
		return nil, err
	}
	if shouldUseCNYPaymentCurrency(channel) {
		currency = "CNY"
	}
	now := time.Now()

	var payment *models.Payment
	var recharge *models.WalletRechargeOrder
	err = s.paymentRepo.Transaction(func(tx *gorm.DB) error {
		rechargeNo := generateWalletRechargeNo()
		paymentRepo := s.paymentRepo.WithTx(tx)
		payment = &models.Payment{
			OrderID:         0,
			ChannelID:       channel.ID,
			ProviderType:    channel.ProviderType,
			ChannelType:     channel.ChannelType,
			InteractionMode: channel.InteractionMode,
			Amount:          models.NewMoneyFromDecimal(payableAmount),
			FeeRate:         models.NewMoneyFromDecimal(feeRate),
			FixedFee:        models.NewMoneyFromDecimal(fixedFee),
			FeeAmount:       models.NewMoneyFromDecimal(feeAmount),
			Currency:        currency,
			Status:          constants.PaymentStatusInitiated,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := paymentRepo.Create(payment); err != nil {
			return ErrPaymentCreateFailed
		}

		rechargeRepo := s.walletRepo.WithTx(tx)
		recharge = &models.WalletRechargeOrder{
			RechargeNo:      rechargeNo,
			UserID:          input.UserID,
			PaymentID:       payment.ID,
			ChannelID:       channel.ID,
			ProviderType:    channel.ProviderType,
			ChannelType:     channel.ChannelType,
			InteractionMode: channel.InteractionMode,
			Amount:          models.NewMoneyFromDecimal(amount),
			PayableAmount:   models.NewMoneyFromDecimal(payableAmount),
			FeeRate:         models.NewMoneyFromDecimal(feeRate),
			FeeAmount:       models.NewMoneyFromDecimal(feeAmount),
			Currency:        currency,
			Status:          constants.WalletRechargeStatusPending,
			Remark:          cleanWalletRemark(input.Remark, "余额充值"),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := rechargeRepo.CreateRechargeOrder(recharge); err != nil {
			return ErrPaymentCreateFailed
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if payment == nil || recharge == nil {
		return nil, ErrPaymentCreateFailed
	}

	// 复用支付网关下单逻辑，使用充值单号作为业务单号。
	virtualOrder := &models.Order{
		OrderNo: recharge.RechargeNo,
		UserID:  recharge.UserID,
	}
	if err := s.applyProviderPayment(CreatePaymentInput{
		ChannelID:        input.ChannelID,
		ClientIP:         input.ClientIP,
		Context:          input.Context,
		ReturnBizType:    "recharge",
		ReturnBusinessNo: recharge.RechargeNo,
	}, virtualOrder, channel, payment); err != nil {
		_ = s.paymentRepo.Transaction(func(tx *gorm.DB) error {
			rechargeRepo := s.walletRepo.WithTx(tx)
			paymentRepo := s.paymentRepo.WithTx(tx)
			failedAt := time.Now()
			payment.Status = constants.PaymentStatusFailed
			payment.UpdatedAt = failedAt
			if updateErr := paymentRepo.Update(payment); updateErr != nil {
				return updateErr
			}
			lockedRecharge, getErr := rechargeRepo.GetRechargeOrderByPaymentIDForUpdate(payment.ID)
			if getErr != nil || lockedRecharge == nil {
				return getErr
			}
			lockedRecharge.Status = constants.WalletRechargeStatusFailed
			lockedRecharge.UpdatedAt = failedAt
			return rechargeRepo.UpdateRechargeOrder(lockedRecharge)
		})
		return nil, err
	}
	if s.queueClient != nil {
		delay := time.Duration(s.resolveExpireMinutes()) * time.Minute
		if err := s.queueClient.EnqueueWalletRechargeExpire(queue.WalletRechargeExpirePayload{
			PaymentID: payment.ID,
		}, delay); err != nil {
			logger.Errorw("wallet_recharge_enqueue_timeout_expire_failed",
				"payment_id", payment.ID,
				"recharge_no", recharge.RechargeNo,
				"delay_minutes", int(delay/time.Minute),
				"error", err,
			)
			_ = s.paymentRepo.Transaction(func(tx *gorm.DB) error {
				rechargeRepo := s.walletRepo.WithTx(tx)
				paymentRepo := s.paymentRepo.WithTx(tx)
				failedAt := time.Now()
				payment.Status = constants.PaymentStatusFailed
				payment.UpdatedAt = failedAt
				if updateErr := paymentRepo.Update(payment); updateErr != nil {
					return updateErr
				}
				lockedRecharge, getErr := rechargeRepo.GetRechargeOrderByPaymentIDForUpdate(payment.ID)
				if getErr != nil || lockedRecharge == nil {
					return getErr
				}
				if lockedRecharge.Status == constants.WalletRechargeStatusSuccess {
					return nil
				}
				lockedRecharge.Status = constants.WalletRechargeStatusFailed
				lockedRecharge.UpdatedAt = failedAt
				return rechargeRepo.UpdateRechargeOrder(lockedRecharge)
			})
			return nil, ErrQueueUnavailable
		}
	}

	reloadedRecharge, err := s.walletRepo.GetRechargeOrderByPaymentID(payment.ID)
	if err != nil {
		return nil, ErrPaymentUpdateFailed
	}
	if reloadedRecharge != nil {
		recharge = reloadedRecharge
	}
	return &CreateWalletRechargePaymentResult{
		Recharge: recharge,
		Payment:  payment,
	}, nil
}

// HandleCallback 处理支付回调

// ListPayments 管理端支付列表
func (s *PaymentService) ListPayments(filter repository.PaymentListFilter) ([]models.Payment, int64, error) {
	return s.paymentRepo.ListAdmin(filter)
}

// GetPayment 获取支付记录
func (s *PaymentService) GetPayment(id uint) (*models.Payment, error) {
	if id == 0 {
		return nil, ErrPaymentInvalid
	}
	payment, err := s.paymentRepo.GetByID(id)
	if err != nil {
		return nil, ErrPaymentUpdateFailed
	}
	if payment == nil {
		return nil, ErrPaymentNotFound
	}
	return payment, nil
}

// CapturePayment 捕获支付。

// ListChannels 支付渠道列表
func (s *PaymentService) ListChannels(filter repository.PaymentChannelListFilter) ([]models.PaymentChannel, int64, error) {
	return s.channelRepo.List(filter)
}

// GetChannel 获取支付渠道
func (s *PaymentService) GetChannel(id uint) (*models.PaymentChannel, error) {
	if id == 0 {
		return nil, ErrPaymentInvalid
	}
	channel, err := s.channelRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, ErrPaymentChannelNotFound
	}
	return channel, nil
}

func generateWalletRechargeNo() string {
	return generateSerialNo("WR")
}

func shouldUseGatewayOrderNo(channel *models.PaymentChannel) bool {
	return channel != nil
}

func buildGatewayOrderNo() string {
	return generateSerialNo("DJP")
}

func resolveGatewayOrderNo(channel *models.PaymentChannel, payment *models.Payment) string {
	if !shouldUseGatewayOrderNo(channel) {
		return ""
	}
	if payment != nil {
		if gatewayOrderNo := strings.TrimSpace(payment.GatewayOrderNo); gatewayOrderNo != "" {
			return gatewayOrderNo
		}
	}
	return buildGatewayOrderNo()
}

func resolveProviderOrderNo(businessOrderNo string, payment *models.Payment) string {
	if gatewayOrderNo := strings.TrimSpace(payment.GatewayOrderNo); gatewayOrderNo != "" {
		return gatewayOrderNo
	}
	return strings.TrimSpace(businessOrderNo)
}

func matchesBusinessOrderNo(callbackOrderNo string, businessOrderNo string, payment *models.Payment) bool {
	callbackOrderNo = strings.TrimSpace(callbackOrderNo)
	if callbackOrderNo == "" {
		return true
	}
	if callbackOrderNo == strings.TrimSpace(businessOrderNo) {
		return true
	}
	return callbackOrderNo == strings.TrimSpace(payment.GatewayOrderNo)
}

func buildPaymentReturnQuery(input CreatePaymentInput, order *models.Order, marker string, sessionID string) map[string]string {
	params := map[string]string{}

	bizType := strings.ToLower(strings.TrimSpace(input.ReturnBizType))
	businessNo := strings.TrimSpace(input.ReturnBusinessNo)
	isGuest := input.ReturnGuest

	if bizType == "" {
		bizType = "order"
	}
	if order != nil {
		if businessNo == "" {
			businessNo = strings.TrimSpace(order.OrderNo)
		}
		if !isGuest && order.UserID == 0 && bizType == "order" {
			isGuest = true
		}
	}

	if bizType != "" {
		params["biz_type"] = bizType
	}
	switch bizType {
	case "recharge":
		if businessNo != "" {
			params["recharge_no"] = businessNo
		}
	default:
		if businessNo != "" {
			params["order_no"] = businessNo
		}
		if isGuest {
			params["guest"] = "1"
		}
	}
	if marker = strings.TrimSpace(marker); marker != "" {
		params[marker] = "1"
	}
	if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
		params["session_id"] = sessionID
	}
	return params
}

func shouldUseCNYPaymentCurrency(channel *models.PaymentChannel) bool {
	if channel == nil {
		return false
	}
	providerType := strings.ToLower(strings.TrimSpace(channel.ProviderType))
	if providerType != constants.PaymentProviderOfficial {
		return false
	}
	channelType := strings.ToLower(strings.TrimSpace(channel.ChannelType))
	return channelType == constants.PaymentChannelTypeWechat || channelType == constants.PaymentChannelTypeAlipay
}

func validatePaymentAmountForChannel(amount decimal.Decimal, channel *models.PaymentChannel) error {
	if channel == nil {
		return nil
	}
	amountOverflow20_2 := decimal.NewFromInt(1000000000000000000)
	if amount.GreaterThanOrEqual(amountOverflow20_2) {
		return ErrPaymentAmountTooLarge
	}
	minAmount := channel.MinAmount.Decimal
	maxAmount := channel.MaxAmount.Decimal
	if minAmount.GreaterThan(decimal.Zero) && amount.LessThan(minAmount) {
		return ErrPaymentAmountTooSmall
	}
	if maxAmount.GreaterThan(decimal.Zero) && amount.GreaterThan(maxAmount) {
		return ErrPaymentAmountTooLarge
	}
	return nil
}

func validatePaymentCurrencyForChannel(currency string, channel *models.PaymentChannel) error {
	normalized := strings.ToUpper(strings.TrimSpace(currency))
	if !settingCurrencyCodePattern.MatchString(normalized) {
		return ErrPaymentCurrencyMismatch
	}
	if shouldUseCNYPaymentCurrency(channel) && normalized != constants.SiteCurrencyDefault {
		return ErrPaymentCurrencyMismatch
	}
	return nil
}

func (s *PaymentService) resolveExpireMinutes() int {
	return resolveOrderPaymentExpireMinutes(s.settingService, s.expireMinutes)
}

func normalizePaymentStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func isPaymentStatusValid(status string) bool {
	switch status {
	case constants.PaymentStatusInitiated, constants.PaymentStatusPending, constants.PaymentStatusSuccess, constants.PaymentStatusFailed, constants.PaymentStatusExpired:
		return true
	default:
		return false
	}
}

func shouldAutoFulfill(order *models.Order) bool {
	if order == nil || len(order.Items) == 0 {
		return false
	}
	for _, item := range order.Items {
		if strings.TrimSpace(item.FulfillmentType) != constants.FulfillmentTypeAuto {
			return false
		}
	}
	return true
}

// isOrderFullyAutoFulfill 判断订单是否完全为自动交付。
// 父订单：所有子订单均满足 shouldAutoFulfill；单订单：自身满足 shouldAutoFulfill。
// 用于支付成功时跳过"已支付"邮件——自动交付会紧接着发送含卡密内容的"已完成"邮件，避免重复打扰。
func isOrderFullyAutoFulfill(order *models.Order) bool {
	if order == nil {
		return false
	}
	if len(order.Children) > 0 {
		for i := range order.Children {
			if !shouldAutoFulfill(&order.Children[i]) {
				return false
			}
		}
		return true
	}
	return shouldAutoFulfill(order)
}

func buildOrderSubject(order *models.Order) string {
	if order == nil {
		return ""
	}
	if len(order.Items) > 0 {
		title := pickOrderItemTitle(order.Items[0].TitleJSON)
		if title != "" {
			return title
		}
	}
	return order.OrderNo
}

func pickOrderItemTitle(title models.JSON) string {
	if title == nil {
		return ""
	}
	for _, key := range constants.SupportedLocales {
		if val, ok := title[key]; ok {
			if str, ok := val.(string); ok && strings.TrimSpace(str) != "" {
				return strings.TrimSpace(str)
			}
		}
	}
	for _, val := range title {
		if str, ok := val.(string); ok && strings.TrimSpace(str) != "" {
			return strings.TrimSpace(str)
		}
	}
	return ""
}

// DecodeChannelIDs 解码 JSON 数组字符串 → []uint
func DecodeChannelIDs(raw string) []uint {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "[]" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(trimmed), &ids); err != nil {
		return nil
	}
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			result = append(result, id)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// EncodeChannelIDs 编码 []uint → JSON 数组字符串
func EncodeChannelIDs(ids []uint) string {
	if len(ids) == 0 {
		return ""
	}
	payload, err := json.Marshal(ids)
	if err != nil {
		return ""
	}
	return string(payload)
}

// computeProductChannelIntersection 计算多个商品允许支付渠道的交集
// 空列表表示不限制（全部允许），不参与交集计算
// 返回 nil 表示无限制，返回空切片表示交集为空（无可用渠道）
func computeProductChannelIntersection(products []models.Product) []uint {
	var intersection map[uint]struct{}
	hasRestriction := false

	for _, p := range products {
		allowed := DecodeChannelIDs(p.PaymentChannelIDs)
		if len(allowed) == 0 {
			continue // 该商品不限制
		}
		hasRestriction = true
		allowedSet := make(map[uint]struct{}, len(allowed))
		for _, id := range allowed {
			allowedSet[id] = struct{}{}
		}
		if intersection == nil {
			intersection = allowedSet
		} else {
			for id := range intersection {
				if _, ok := allowedSet[id]; !ok {
					delete(intersection, id)
				}
			}
		}
	}

	if !hasRestriction {
		return nil // 所有商品都不限制
	}

	result := make([]uint, 0, len(intersection))
	for id := range intersection {
		result = append(result, id)
	}
	return result
}

// validateProductPaymentChannel 校验支付渠道是否被订单中的商品允许
// tx 参数可选：在事务内调用时传入 tx 以避免 SQLite 自锁
func (s *PaymentService) validateProductPaymentChannel(items []models.OrderItem, channelID uint, tx ...*gorm.DB) error {
	if len(items) == 0 {
		return nil
	}
	productIDSet := make(map[uint]struct{})
	for _, item := range items {
		if item.ProductID > 0 {
			productIDSet[item.ProductID] = struct{}{}
		}
	}
	if len(productIDSet) == 0 {
		return nil
	}
	productIDs := make([]uint, 0, len(productIDSet))
	for id := range productIDSet {
		productIDs = append(productIDs, id)
	}
	repo := s.productRepo
	if len(tx) > 0 && tx[0] != nil {
		repo = s.productRepo.WithTx(tx[0])
	}
	products, err := repo.ListByIDs(productIDs)
	if err != nil {
		return ErrProductFetchFailed
	}
	allowed := computeProductChannelIntersection(products)
	if allowed == nil {
		return nil // 无限制
	}
	for _, id := range allowed {
		if id == channelID {
			return nil
		}
	}
	return ErrPaymentChannelNotAllowedForProduct
}

// validateWalletRechargeChannel 校验支付渠道是否被钱包充值设置允许
func (s *PaymentService) validateWalletRechargeChannel(channelID uint) error {
	if s.settingService == nil {
		return nil
	}
	allowedIDs := s.settingService.GetWalletRechargeChannelIDs()
	if len(allowedIDs) == 0 {
		return nil // 无限制
	}
	for _, id := range allowedIDs {
		if id == channelID {
			return nil
		}
	}
	return ErrPaymentChannelNotAllowedForRecharge
}

// GetAllowedChannelsForProducts 获取商品允许的支付渠道列表
func (s *PaymentService) GetAllowedChannelsForProducts(productIDs []uint) ([]models.PaymentChannel, error) {
	// 仅钱包余额支付模式下不返回任何在线支付渠道
	if s.settingService != nil && s.settingService.GetWalletOnlyPayment() {
		return []models.PaymentChannel{}, nil
	}
	channels, _, err := s.ListChannels(repository.PaymentChannelListFilter{
		Page:       1,
		PageSize:   200,
		ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}
	if len(productIDs) == 0 {
		return channels, nil
	}
	products, err := s.productRepo.ListByIDs(productIDs)
	if err != nil {
		return nil, ErrProductFetchFailed
	}
	allowed := computeProductChannelIntersection(products)
	if allowed == nil {
		return channels, nil // 无限制
	}
	allowedSet := make(map[uint]struct{}, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = struct{}{}
	}
	filtered := make([]models.PaymentChannel, 0, len(allowed))
	for _, ch := range channels {
		if _, ok := allowedSet[ch.ID]; ok {
			filtered = append(filtered, ch)
		}
	}
	return filtered, nil
}

// GetWalletRechargeChannels 获取钱包充值允许的支付渠道列表
func (s *PaymentService) GetWalletRechargeChannels() ([]models.PaymentChannel, error) {
	channels, _, err := s.ListChannels(repository.PaymentChannelListFilter{
		Page:       1,
		PageSize:   200,
		ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}
	if s.settingService == nil {
		return channels, nil
	}
	allowedIDs := s.settingService.GetWalletRechargeChannelIDs()
	if len(allowedIDs) == 0 {
		return channels, nil
	}
	allowedSet := make(map[uint]struct{}, len(allowedIDs))
	for _, id := range allowedIDs {
		allowedSet[id] = struct{}{}
	}
	filtered := make([]models.PaymentChannel, 0, len(allowedIDs))
	for _, ch := range channels {
		if _, ok := allowedSet[ch.ID]; ok {
			filtered = append(filtered, ch)
		}
	}
	return filtered, nil
}

// GetAllowedChannelIDsForOrder 获取订单中所有商品允许的支付渠道ID交集
func (s *PaymentService) GetAllowedChannelIDsForOrder(items []models.OrderItem) []uint {
	if len(items) == 0 {
		return nil
	}
	productIDSet := make(map[uint]struct{})
	for _, item := range items {
		if item.ProductID > 0 {
			productIDSet[item.ProductID] = struct{}{}
		}
	}
	if len(productIDSet) == 0 {
		return nil
	}
	productIDs := make([]uint, 0, len(productIDSet))
	for id := range productIDSet {
		productIDs = append(productIDs, id)
	}
	products, err := s.productRepo.ListByIDs(productIDs)
	if err != nil {
		return nil
	}
	return computeProductChannelIntersection(products)
}
