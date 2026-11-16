package application

import (
	"time"

	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
)

const (
	affiliateCodeLength        = 8
	affiliateSplitTypePrefix   = "sp"
	affiliateAttributionWindow = 30 * 24 * time.Hour
	affiliateClickDedupeWindow = 10 * time.Minute
)

// WithdrawNotifier 在提现申请创建后投递通知。
// 由 SetWithdrawNotifier 延迟注入，解决容器装配顺序依赖
// （NotificationService 构建晚于本服务）。
//
// 参数用基本类型而不是结构体：reseller 与 affiliate 模块各自声明该接口，
// 同一份结构体会因包路径不同而无法被同一个实现同时满足。
type WithdrawNotifier interface {
	NotifyWithdrawSubmitted(scope string, requestID, userID uint, amount, currency, channel, account string)
}

// ScopeAffiliate 推广返利提现申请的类别标识。
const ScopeAffiliate = "affiliate"

// Service 推广返利业务服务
type Service struct {
	repo        affiliatecontract.Store
	userRepo    usercontract.Store
	orderRepo   affiliatecontract.OrderReader
	productRepo affiliatecontract.ProductReader
	skuRepo     affiliatecontract.SKUReader
	settings    affiliatecontract.SettingsReader
	notifier    WithdrawNotifier
}

// SetWithdrawNotifier 注入提现通知依赖（在容器构建 NotificationService 之后调用）。
func (s *Service) SetWithdrawNotifier(notifier WithdrawNotifier) {
	if s == nil {
		return
	}
	s.notifier = notifier
}

// SetSKUReader 注入 SKU 读取依赖，用于解析 SKU 级返利比例。
// 未注入时按商品级/全站比例计算，已有调用方无需改动。
func (s *Service) SetSKUReader(reader affiliatecontract.SKUReader) {
	if s == nil {
		return
	}
	s.skuRepo = reader
}

// NewService 创建推广返利服务
func NewService(
	repo affiliatecontract.Store,
	userRepo usercontract.Store,
	orderRepo affiliatecontract.OrderReader,
	productRepo affiliatecontract.ProductReader,
	settings affiliatecontract.SettingsReader,
) *Service {
	return &Service{
		repo:        repo,
		userRepo:    userRepo,
		orderRepo:   orderRepo,
		productRepo: productRepo,
		settings:    settings,
	}
}
