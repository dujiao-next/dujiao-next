package application

import (
	"errors"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/logger"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	externalidentitycontract "github.com/dujiao-next/internal/modules/identity/externalidentity/contract"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

var (
	ErrPaymentInvalid                      = errors.New("payment invalid")
	ErrPaymentNotFound                     = errors.New("payment not found")
	ErrPaymentCreateFailed                 = errors.New("payment create failed")
	ErrPaymentUpdateFailed                 = errors.New("payment update failed")
	ErrPaymentStatusInvalid                = errors.New("payment status invalid")
	ErrPaymentAmountMismatch               = errors.New("payment amount mismatch")
	ErrPaymentCurrencyMismatch             = errors.New("payment currency mismatch")
	ErrPaymentChannelNotFound              = errors.New("payment channel not found")
	ErrPaymentChannelInactive              = errors.New("payment channel inactive")
	ErrPaymentProviderNotSupported         = errors.New("payment provider not supported")
	ErrPaymentChannelConfigInvalid         = errors.New("payment channel config invalid")
	ErrPaymentAmountTooSmall               = errors.New("payment amount too small")
	ErrPaymentAmountTooLarge               = errors.New("payment amount too large")
	ErrPaymentGatewayRequestFailed         = errors.New("payment gateway request failed")
	ErrPaymentGatewayResponseInvalid       = errors.New("payment gateway response invalid")
	ErrPaymentChannelNotAllowedForProduct  = errors.New("payment channel not allowed for product")
	ErrPaymentChannelNotAllowedForRecharge = errors.New("payment channel not allowed for wallet recharge")
	ErrProductFetchFailed                  = errors.New("product fetch failed")
	ErrQueueUnavailable                    = errors.New("queue unavailable")
	ErrPaymentUnderpaidCredited            = errors.New("payment underpaid already credited to wallet")
	ErrPaymentCanceledOrderUnderpaid       = errors.New("canceled order payment does not cover total")
)

// PaymentService 支付服务
type PaymentService struct {
	orderRepo               ordercontract.Store
	productRepo             productcontract.Repository
	productSKURepo          productcontract.SKURepository
	paymentRepo             paymentcontract.Store
	channelRepo             paymentcontract.ChannelStore
	walletRepo              walletcontract.Repository
	userRepo                usercontract.Store
	userOAuthIdentityRepo   externalidentitycontract.Store
	queue                   paymentcontract.Queue
	walletSvc               *walletapp.Service
	settingService          *settingsapp.Service
	defaultEmailConfig      config.EmailConfig
	expireMinutes           int
	affiliateSvc            AffiliatePaymentLifecycle
	notificationSvc         notificationcontract.NotificationEnqueuer
	procurementSvc          ProcurementCreator
	downstreamCallbackSvc   DownstreamCallbackEnqueuer
	memberLevelSvc          MemberLevelProgressor
	paymentProviderRegistry paymentcontract.GatewayRegistry
	resellerAccounting      resellerAccountingTransactions
}

type MemberLevelProgressor interface {
	OnOrderPaid(userID uint, amount decimal.Decimal) error
	OnRechargeCompleted(userID uint, amount decimal.Decimal) error
}

type ProcurementCreator interface {
	CreateForOrder(orderID uint) error
}

// DownstreamCallbackEnqueuer 是支付与交付上下文触发下游回调所需的最小端口。
type DownstreamCallbackEnqueuer interface {
	EnqueueCallback(orderID uint)
}

// AffiliatePaymentLifecycle 是支付成功回调所需的推广返利用例端口。
type AffiliatePaymentLifecycle interface {
	HandleOrderPaid(orderID uint) error
}

type resellerAccountingTransactions interface {
	PostOrderProfit(store resellercontract.AccountingLedgerStore, order *orderdomain.Order, payment *paymentdomain.Payment) error
}

// SetProcurementService 设置采购单服务（解决循环依赖）
func (s *PaymentService) SetProcurementService(svc ProcurementCreator) {
	s.procurementSvc = svc
}

// SetDownstreamCallbackService 设置下游回调服务（解决循环依赖）
func (s *PaymentService) SetDownstreamCallbackService(svc DownstreamCallbackEnqueuer) {
	s.downstreamCallbackSvc = svc
}

// SetMemberLevelService 设置会员等级服务
func (s *PaymentService) SetMemberLevelService(svc MemberLevelProgressor) {
	s.memberLevelSvc = svc
}

// PaymentServiceOptions 支付服务构造参数
type PaymentServiceOptions struct {
	OrderStore              ordercontract.Store
	ProductRepo             productcontract.Repository
	ProductSKURepo          productcontract.SKURepository
	PaymentStore            paymentcontract.Store
	ChannelStore            paymentcontract.ChannelStore
	WalletRepo              walletcontract.Repository
	UserStore               usercontract.Store
	ExternalIdentityStore   externalidentitycontract.Store
	Queue                   paymentcontract.Queue
	WalletService           *walletapp.Service
	SettingService          *settingsapp.Service
	DefaultEmailConfig      config.EmailConfig
	ExpireMinutes           int
	AffiliateService        AffiliatePaymentLifecycle
	NotificationService     notificationcontract.NotificationEnqueuer
	PaymentProviderRegistry paymentcontract.GatewayRegistry
	ResellerAccounting      resellerAccountingTransactions
}

// NewPaymentService 创建支付服务
func NewPaymentService(opts PaymentServiceOptions) *PaymentService {
	return &PaymentService{
		orderRepo:               opts.OrderStore,
		productRepo:             opts.ProductRepo,
		productSKURepo:          opts.ProductSKURepo,
		paymentRepo:             opts.PaymentStore,
		channelRepo:             opts.ChannelStore,
		walletRepo:              opts.WalletRepo,
		userRepo:                opts.UserStore,
		userOAuthIdentityRepo:   opts.ExternalIdentityStore,
		queue:                   opts.Queue,
		walletSvc:               opts.WalletService,
		settingService:          opts.SettingService,
		defaultEmailConfig:      opts.DefaultEmailConfig,
		expireMinutes:           opts.ExpireMinutes,
		affiliateSvc:            opts.AffiliateService,
		notificationSvc:         opts.NotificationService,
		paymentProviderRegistry: opts.PaymentProviderRegistry,
		resellerAccounting:      opts.ResellerAccounting,
	}
}

// ListPayments 管理端支付列表。
func (s *PaymentService) ListPayments(filter paymentcontract.ListFilter) ([]paymentdomain.Payment, int64, error) {
	return s.paymentRepo.ListAdmin(filter)
}

// GetPayment 获取支付记录。
func (s *PaymentService) GetPayment(id uint) (*paymentdomain.Payment, error) {
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

// ListChannels 支付渠道列表。
func (s *PaymentService) ListChannels(filter paymentcontract.ChannelListFilter) ([]paymentdomain.PaymentChannel, int64, error) {
	return s.channelRepo.List(filter)
}

// GetChannel 获取支付渠道。
func (s *PaymentService) GetChannel(id uint) (*paymentdomain.PaymentChannel, error) {
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

func paymentLogger(kv ...interface{}) *zap.SugaredLogger {
	if len(kv) == 0 {
		return logger.S()
	}
	return logger.SW(kv...)
}
