package application

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderriskcontract "github.com/dujiao-next/internal/modules/orderrisk/contract"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/shared/serial"

	"github.com/shopspring/decimal"
)

const MinGuestPasswordLength = 12

func normalizeGuestPassword(raw string) (string, error) {
	password := strings.TrimSpace(raw)
	if password == "" {
		return "", ErrGuestPasswordRequired
	}
	if utf8.RuneCountInString(password) < MinGuestPasswordLength {
		return "", ErrGuestPasswordTooWeak
	}
	return password, nil
}

// OrderService 订单服务
type OrderService struct {
	orderStore              ordercontract.Store
	userRepo                usercontract.Store
	productRepo             productcontract.Repository
	productSKURepo          productcontract.SKURepository
	couponRepo              couponcontract.Repository
	couponUsageRepo         couponcontract.UsageRepository
	promotionRepo           promotioncontract.Repository
	queueClient             ordercontract.Queue
	settingService          *settingsapp.Service
	defaultEmailConfig      config.EmailConfig
	walletService           *walletapp.Service
	affiliateSvc            AffiliateOrderLifecycle
	memberLevelService      OrderMemberLevelService
	resellerPricingResolver *ResellerPricingResolver
	resellerAccounting      resellerAccountingTransactions
	riskControlSvc          orderriskcontract.Controller
	productMappingService   upstreamStockEnsurer
	expireMinutes           int
}

type OrderMemberLevelService interface {
	ResolveMemberPrice(levelID, productID, skuID uint, basePrice decimal.Decimal) (decimal.Decimal, decimal.Decimal)
	OnOrderPaid(userID uint, amount decimal.Decimal) error
}

// AffiliateOrderLifecycle 是订单域调用推广返利用例的最小端口。
type AffiliateOrderLifecycle interface {
	ResolveOrderAffiliateSnapshot(userID uint, rawCode, rawVisitorKey string) (*uint, string, error)
	HandleOrderPaid(orderID uint) error
	HandleOrderCanceled(orderID uint, reason string) error
}

// resellerAccountingTransactions 是订单事务内调用分销账务用例的最小端口。
type resellerAccountingTransactions interface {
	PostOrderProfitForOrder(store resellercontract.AccountingLedgerStore, order *orderdomain.Order) error
	HandleRefundDeduct(
		store resellercontract.AccountingLedgerStore,
		order *orderdomain.Order,
		refundRecord *orderdomain.OrderRefundRecord,
		refundedBefore decimal.Decimal,
	) error
}

// upstreamStockEnsurer 是下单校验依赖的最小 Catalog Mapping 用例端口。
type upstreamStockEnsurer interface {
	EnsureUpstreamStockForOrder(localSKUID uint, quantity int) error
}

// OrderServiceOptions 订单服务构造参数
type OrderServiceOptions struct {
	OrderStore              ordercontract.Store
	UserStore               usercontract.Store
	ProductStore            productcontract.Repository
	ProductSKUStore         productcontract.SKURepository
	CouponStore             couponcontract.Repository
	CouponUsageStore        couponcontract.UsageRepository
	PromotionRepo           promotioncontract.Repository
	Queue                   ordercontract.Queue
	SettingService          *settingsapp.Service
	DefaultEmailConfig      config.EmailConfig
	WalletService           *walletapp.Service
	AffiliateService        AffiliateOrderLifecycle
	MemberLevelService      OrderMemberLevelService
	ResellerPricingResolver *ResellerPricingResolver
	ResellerAccounting      resellerAccountingTransactions
	RiskControlService      orderriskcontract.Controller
	ProductMappingService   upstreamStockEnsurer
	ExpireMinutes           int
}

// SetProductMappingService 注入商品映射服务（用于下单前上游库存兜底校验）。
// 由 provider 在 ProductMappingService 构造之后调用，避免构造顺序耦合。
func (s *OrderService) SetProductMappingService(svc upstreamStockEnsurer) {
	if s == nil {
		return
	}
	s.productMappingService = svc
}

// NewOrderService 创建订单服务
func NewOrderService(opts OrderServiceOptions) *OrderService {
	return &OrderService{
		orderStore:              opts.OrderStore,
		userRepo:                opts.UserStore,
		productRepo:             opts.ProductStore,
		productSKURepo:          opts.ProductSKUStore,
		couponRepo:              opts.CouponStore,
		couponUsageRepo:         opts.CouponUsageStore,
		promotionRepo:           opts.PromotionRepo,
		queueClient:             opts.Queue,
		settingService:          opts.SettingService,
		defaultEmailConfig:      opts.DefaultEmailConfig,
		walletService:           opts.WalletService,
		affiliateSvc:            opts.AffiliateService,
		memberLevelService:      opts.MemberLevelService,
		resellerPricingResolver: opts.ResellerPricingResolver,
		resellerAccounting:      opts.ResellerAccounting,
		riskControlSvc:          opts.RiskControlService,
		productMappingService:   opts.ProductMappingService,
		expireMinutes:           opts.ExpireMinutes,
	}
}

// CreateOrderInput 创建订单输入
type CreateOrderInput struct {
	UserID              uint
	Tenant              resellercontract.TenantContext
	Items               []CreateOrderItem
	CouponCode          string
	AffiliateCode       string
	AffiliateVisitorKey string
	ClientIP            string
	ManualFormData      map[string]jsonmap.JSON
	SkipRiskControl     bool // 完全跳过风控（下游订单）
	SkipIPRiskControl   bool // 跳过 IP 维度风控（渠道/Bot 订单）
}

// CreateGuestOrderInput 游客创建订单输入
type CreateGuestOrderInput struct {
	Email               string
	OrderPassword       string
	Locale              string
	Tenant              resellercontract.TenantContext
	Items               []CreateOrderItem
	CouponCode          string
	AffiliateCode       string
	AffiliateVisitorKey string
	ClientIP            string
	ManualFormData      map[string]jsonmap.JSON
}

// CreateOrderItem 创建订单项输入
type CreateOrderItem struct {
	ProductID       uint
	SKUID           uint
	Quantity        int
	FulfillmentType string
}

// childOrderPlan 子订单计划数据
type childOrderPlan struct {
	Product           *productdomain.Product
	SKU               *productdomain.ProductSKU
	Item              orderdomain.OrderItem
	TotalAmount       decimal.Decimal
	MemberDiscount    decimal.Decimal
	PromotionDiscount decimal.Decimal
	WholesaleDiscount decimal.Decimal
	CouponDiscount    decimal.Decimal
	Currency          string
}

var allowedTransitions = map[string]map[string]bool{
	constants.OrderStatusPendingPayment: {
		constants.OrderStatusPaid:     true,
		constants.OrderStatusCanceled: true,
	},
	constants.OrderStatusPaid: {
		constants.OrderStatusFulfilling:         true,
		constants.OrderStatusPartiallyDelivered: true,
		constants.OrderStatusDelivered:          true,
		constants.OrderStatusPartiallyRefunded:  true,
		constants.OrderStatusRefunded:           true,
	},
	constants.OrderStatusFulfilling: {
		constants.OrderStatusPartiallyDelivered: true,
		constants.OrderStatusDelivered:          true,
		constants.OrderStatusPartiallyRefunded:  true,
		constants.OrderStatusRefunded:           true,
	},
	constants.OrderStatusPartiallyDelivered: {
		constants.OrderStatusDelivered:         true,
		constants.OrderStatusCompleted:         true,
		constants.OrderStatusPartiallyRefunded: true,
		constants.OrderStatusRefunded:          true,
	},
	constants.OrderStatusDelivered: {
		constants.OrderStatusCompleted:         true,
		constants.OrderStatusPartiallyRefunded: true,
		constants.OrderStatusRefunded:          true,
	},
	constants.OrderStatusCompleted: {
		constants.OrderStatusPartiallyRefunded: true,
		constants.OrderStatusRefunded:          true,
	},
	constants.OrderStatusPartiallyRefunded: {
		constants.OrderStatusRefunded: true,
	},
}

// CreateOrder 创建订单
func (s *OrderService) CreateOrder(input CreateOrderInput) (*orderdomain.Order, error) {
	if input.UserID == 0 {
		return nil, ErrInvalidOrderItem
	}
	return s.createOrder(orderCreateParams{
		UserID:              input.UserID,
		Tenant:              input.Tenant,
		Items:               input.Items,
		CouponCode:          input.CouponCode,
		AffiliateCode:       input.AffiliateCode,
		AffiliateVisitorKey: input.AffiliateVisitorKey,
		ClientIP:            input.ClientIP,
		ManualFormData:      input.ManualFormData,
		SkipRiskControl:     input.SkipRiskControl,
		SkipIPRiskControl:   input.SkipIPRiskControl,
	})
}

// CreateGuestOrder 游客创建订单
func (s *OrderService) CreateGuestOrder(input CreateGuestOrderInput) (*orderdomain.Order, error) {
	email, err := normalizeGuestEmail(input.Email)
	if err != nil {
		return nil, err
	}
	password, err := normalizeGuestPassword(input.OrderPassword)
	if err != nil {
		return nil, err
	}
	locale := strings.TrimSpace(input.Locale)
	return s.createOrder(orderCreateParams{
		UserID:              0,
		GuestEmail:          email,
		GuestPassword:       password,
		GuestLocale:         locale,
		Tenant:              input.Tenant,
		Items:               input.Items,
		CouponCode:          input.CouponCode,
		AffiliateCode:       input.AffiliateCode,
		AffiliateVisitorKey: input.AffiliateVisitorKey,
		ClientIP:            input.ClientIP,
		IsGuest:             true,
		ManualFormData:      input.ManualFormData,
	})
}

type orderCreateParams struct {
	UserID                   uint
	GuestEmail               string
	GuestPassword            string
	GuestLocale              string
	Tenant                   resellercontract.TenantContext
	Items                    []CreateOrderItem
	CouponCode               string
	AffiliateCode            string
	AffiliateVisitorKey      string
	ClientIP                 string
	RiskIP                   string
	RiskPaymentExpireMinutes int
	RiskCheckResult          orderriskcontract.CheckResult
	IsGuest                  bool
	ManualFormData           map[string]jsonmap.JSON
	SkipManualFormCheck      bool
	SkipRiskControl          bool
	SkipIPRiskControl        bool
}

// OrderPreview 订单金额预览
type OrderPreview struct {
	Currency                string             `json:"currency"`
	OriginalAmount          money.Amount       `json:"original_amount"`
	MemberDiscountAmount    money.Amount       `json:"member_discount_amount"`
	DiscountAmount          money.Amount       `json:"discount_amount"`
	PromotionDiscountAmount money.Amount       `json:"promotion_discount_amount"`
	WholesaleDiscountAmount money.Amount       `json:"wholesale_discount_amount"`
	TotalAmount             money.Amount       `json:"total_amount"`
	Items                   []OrderPreviewItem `json:"items"`
}

// OrderPreviewItem 订单项金额预览
type OrderPreviewItem struct {
	ProductID          uint              `json:"product_id"`
	SKUID              uint              `json:"sku_id"`
	TitleJSON          jsonmap.JSON      `json:"title"`
	SKUSnapshotJSON    jsonmap.JSON      `json:"sku_snapshot"`
	Tags               jsonslice.Strings `json:"tags"`
	OriginalUnitPrice  money.Amount      `json:"original_unit_price"`
	UnitPrice          money.Amount      `json:"unit_price"`
	Quantity           int               `json:"quantity"`
	OriginalTotalPrice money.Amount      `json:"original_total_price"`
	TotalPrice         money.Amount      `json:"total_price"`
	MemberDiscount     money.Amount      `json:"member_discount_amount"`
	CouponDiscount     money.Amount      `json:"coupon_discount_amount"`
	PromotionDiscount  money.Amount      `json:"promotion_discount_amount"`
	WholesaleDiscount  money.Amount      `json:"wholesale_discount_amount"`
	FulfillmentType    string            `json:"fulfillment_type"`
}

type orderBuildResult struct {
	Plans                   []childOrderPlan
	OrderItems              []orderdomain.OrderItem
	OriginalAmount          decimal.Decimal
	MemberDiscountAmount    decimal.Decimal
	PromotionDiscountAmount decimal.Decimal
	WholesaleDiscountAmount decimal.Decimal
	DiscountAmount          decimal.Decimal
	TotalAmount             decimal.Decimal
	Currency                string
	OrderPromotionID        *uint
	MemberLevelID           *uint
	AppliedCoupon           *coupondomain.Coupon
}

// PreviewOrder 用户订单金额预览
func (s *OrderService) PreviewOrder(input CreateOrderInput) (*OrderPreview, error) {
	if input.UserID == 0 {
		return nil, ErrInvalidOrderItem
	}
	params := orderCreateParams{
		UserID:              input.UserID,
		Tenant:              input.Tenant,
		Items:               input.Items,
		CouponCode:          input.CouponCode,
		AffiliateCode:       input.AffiliateCode,
		AffiliateVisitorKey: input.AffiliateVisitorKey,
		ClientIP:            input.ClientIP,
		ManualFormData:      input.ManualFormData,
		SkipManualFormCheck: true,
	}
	if err := s.checkOrderRisk(&params, false); err != nil {
		return nil, err
	}
	return s.previewOrder(params)
}

// PreviewGuestOrder 游客订单金额预览
func (s *OrderService) PreviewGuestOrder(input CreateGuestOrderInput) (*OrderPreview, error) {
	password, err := normalizeGuestPassword(input.OrderPassword)
	if err != nil {
		return nil, err
	}
	params := orderCreateParams{
		GuestEmail:          input.Email,
		GuestPassword:       password,
		GuestLocale:         input.Locale,
		Tenant:              input.Tenant,
		Items:               input.Items,
		CouponCode:          input.CouponCode,
		AffiliateCode:       input.AffiliateCode,
		AffiliateVisitorKey: input.AffiliateVisitorKey,
		ClientIP:            input.ClientIP,
		IsGuest:             true,
		ManualFormData:      input.ManualFormData,
		SkipManualFormCheck: true,
	}
	if err := s.checkOrderRisk(&params, false); err != nil {
		return nil, err
	}
	return s.previewOrder(params)
}

func (s *OrderService) previewOrder(input orderCreateParams) (*OrderPreview, error) {
	result, err := s.buildOrderResult(input)
	if err != nil {
		return nil, err
	}
	if s.resellerPricingResolver != nil {
		if _, err := s.resellerPricingResolver.ApplyToOrderBuildResult(input.Tenant, input.UserID, result); err != nil {
			return nil, err
		}
	} else if isResellerOrderContext(input.Tenant) {
		return nil, ErrResellerProductNotListed
	}
	items := make([]OrderPreviewItem, 0, len(result.Plans))
	for _, plan := range result.Plans {
		item := plan.Item
		items = append(items, OrderPreviewItem{
			ProductID:          item.ProductID,
			SKUID:              item.SKUID,
			TitleJSON:          item.TitleJSON,
			SKUSnapshotJSON:    item.SKUSnapshotJSON,
			Tags:               item.Tags,
			OriginalUnitPrice:  item.OriginalUnitPrice,
			UnitPrice:          item.UnitPrice,
			Quantity:           item.Quantity,
			OriginalTotalPrice: item.OriginalTotalPrice,
			TotalPrice:         item.TotalPrice,
			MemberDiscount:     item.MemberDiscount,
			CouponDiscount:     item.CouponDiscount,
			PromotionDiscount:  item.PromotionDiscount,
			WholesaleDiscount:  item.WholesaleDiscount,
			FulfillmentType:    item.FulfillmentType,
		})
	}
	return &OrderPreview{
		Currency:                result.Currency,
		OriginalAmount:          money.FromDecimal(result.OriginalAmount),
		MemberDiscountAmount:    money.FromDecimal(result.MemberDiscountAmount),
		DiscountAmount:          money.FromDecimal(result.DiscountAmount),
		PromotionDiscountAmount: money.FromDecimal(result.PromotionDiscountAmount),
		WholesaleDiscountAmount: money.FromDecimal(result.WholesaleDiscountAmount),
		TotalAmount:             money.FromDecimal(result.TotalAmount),
		Items:                   items,
	}, nil
}

func (s *OrderService) createOrder(input orderCreateParams) (*orderdomain.Order, error) {
	if s.queueClient == nil || !s.queueClient.Enabled() {
		return nil, ErrQueueUnavailable
	}

	if err := s.checkOrderRisk(&input, true); err != nil {
		return nil, err
	}

	result, err := s.buildOrderResult(input)
	if err != nil {
		return nil, err
	}
	var pricingCtx *resellercontract.OrderPricingContext
	if s.resellerPricingResolver != nil {
		pricingCtx, err = s.resellerPricingResolver.ApplyToOrderBuildResult(input.Tenant, input.UserID, result)
		if err != nil {
			return nil, err
		}
	} else if isResellerOrderContext(input.Tenant) {
		return nil, ErrResellerProductNotListed
	}

	// 仅允许钱包余额支付时，在创建订单（锁库存）前预校验余额是否充足
	if s.settingService != nil && s.settingService.GetWalletOnlyPayment() {
		if input.UserID == 0 {
			// 游客无钱包，wallet-only 模式下不允许下单
			return nil, walletcontract.ErrOnlyPaymentRequired
		}
		if s.walletService == nil {
			return nil, walletcontract.ErrOnlyPaymentRequired
		}
		account, accErr := s.walletService.GetAccount(input.UserID)
		if accErr != nil {
			return nil, walletcontract.ErrOnlyPaymentRequired
		}
		if account.Balance.Decimal.LessThan(result.TotalAmount) {
			return nil, walletcontract.ErrInsufficientBalance
		}
	}

	affiliateCode := affiliatedomain.NormalizeCode(input.AffiliateCode)
	affiliateVisitorKey := strings.TrimSpace(input.AffiliateVisitorKey)
	var affiliateProfileID *uint
	if pricingCtx != nil {
		affiliateCode = ""
		affiliateVisitorKey = ""
	} else if s.affiliateSvc != nil {
		resolvedID, resolvedCode, resolveErr := s.affiliateSvc.ResolveOrderAffiliateSnapshot(input.UserID, affiliateCode, affiliateVisitorKey)
		if resolveErr != nil {
			return nil, resolveErr
		}
		affiliateProfileID = resolvedID
		affiliateCode = resolvedCode
	}

	if len(input.Items) == 0 {
		return nil, ErrInvalidOrderItem
	}
	if s.productSKURepo == nil {
		return nil, ErrProductSKUInvalid
	}
	if input.IsGuest && input.GuestEmail == "" {
		return nil, ErrGuestEmailRequired
	}
	if input.IsGuest {
		password, passwordErr := normalizeGuestPassword(input.GuestPassword)
		if passwordErr != nil {
			return nil, passwordErr
		}
		input.GuestPassword = password
	}

	expireMinutes := s.resolveExpireMinutes()
	if input.RiskPaymentExpireMinutes > 0 {
		expireMinutes = input.RiskPaymentExpireMinutes
	}
	now := time.Now()
	expiresAt := now.Add(time.Duration(expireMinutes) * time.Minute)
	order := &orderdomain.Order{
		OrderNo:                 generateOrderNo(),
		UserID:                  input.UserID,
		GuestEmail:              input.GuestEmail,
		GuestPassword:           input.GuestPassword,
		GuestLocale:             input.GuestLocale,
		Status:                  constants.OrderStatusPendingPayment,
		Currency:                result.Currency,
		OriginalAmount:          money.FromDecimal(result.OriginalAmount),
		MemberDiscountAmount:    money.FromDecimal(result.MemberDiscountAmount),
		DiscountAmount:          money.FromDecimal(result.DiscountAmount),
		PromotionDiscountAmount: money.FromDecimal(result.PromotionDiscountAmount),
		WholesaleDiscountAmount: money.FromDecimal(result.WholesaleDiscountAmount),
		TotalAmount:             money.FromDecimal(result.TotalAmount),
		WalletPaidAmount:        money.FromDecimal(decimal.Zero),
		OnlinePaidAmount:        money.FromDecimal(result.TotalAmount),
		RefundedAmount:          money.FromDecimal(decimal.Zero),
		MemberLevelID:           result.MemberLevelID,
		CouponID:                nil,
		PromotionID:             result.OrderPromotionID,
		AffiliateProfileID:      affiliateProfileID,
		AffiliateCode:           affiliateCode,
		ExpiresAt:               &expiresAt,
		ClientIP:                strings.TrimSpace(input.ClientIP),
		RiskIP:                  input.RiskIP,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if pricingCtx != nil {
		resellerID := pricingCtx.ResellerID
		order.ResellerID = &resellerID
		order.ResellerDomain = pricingCtx.Domain
		order.ResellerProfitAmount = money.FromDecimal(pricingCtx.EffectiveProfit)
	}

	if result.AppliedCoupon != nil {
		order.CouponID = &result.AppliedCoupon.ID
	}

	err = s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
		orderStore := tx.Orders()
		productSKURepo := tx.ProductSKUs()
		if s.riskControlSvc != nil && !input.SkipRiskControl {
			if err := s.riskControlSvc.CheckPendingOrderAllowed(buildRiskCheckInput(input, false), input.RiskCheckResult, orderStore); err != nil {
				return err
			}
		}
		if err := orderStore.Create(order, nil); err != nil {
			return err
		}

		for idx := range result.Plans {
			plan := result.Plans[idx]
			childProfit := decimal.Zero
			if pricingCtx != nil && idx < len(pricingCtx.Items) && pricingCtx.ProfitEligible {
				childProfit = pricingCtx.Items[idx].ProfitAmount
			}
			childOrder := &orderdomain.Order{
				OrderNo:                 buildChildOrderNo(order.OrderNo, idx+1),
				ParentID:                &order.ID,
				UserID:                  order.UserID,
				GuestEmail:              order.GuestEmail,
				GuestPassword:           input.GuestPassword,
				GuestLocale:             order.GuestLocale,
				Status:                  constants.OrderStatusPendingPayment,
				Currency:                plan.Currency,
				OriginalAmount:          money.FromDecimal(plan.TotalAmount),
				MemberDiscountAmount:    money.FromDecimal(plan.MemberDiscount),
				DiscountAmount:          money.FromDecimal(plan.CouponDiscount),
				PromotionDiscountAmount: money.FromDecimal(plan.PromotionDiscount),
				WholesaleDiscountAmount: money.FromDecimal(plan.WholesaleDiscount),
				TotalAmount:             money.FromDecimal(normalizeOrderAmount(plan.TotalAmount.Sub(plan.CouponDiscount))),
				WalletPaidAmount:        money.FromDecimal(decimal.Zero),
				OnlinePaidAmount:        money.FromDecimal(normalizeOrderAmount(plan.TotalAmount.Sub(plan.CouponDiscount))),
				RefundedAmount:          money.FromDecimal(decimal.Zero),
				CouponID:                nil,
				PromotionID:             plan.Item.PromotionID,
				AffiliateProfileID:      affiliateProfileID,
				AffiliateCode:           affiliateCode,
				ExpiresAt:               &expiresAt,
				ClientIP:                order.ClientIP,
				RiskIP:                  order.RiskIP,
				CreatedAt:               now,
				UpdatedAt:               now,
			}
			if pricingCtx != nil {
				resellerID := pricingCtx.ResellerID
				childOrder.ResellerID = &resellerID
				childOrder.ResellerDomain = pricingCtx.Domain
				childOrder.ResellerProfitAmount = money.FromDecimal(childProfit)
			}
			if result.AppliedCoupon != nil && plan.CouponDiscount.GreaterThan(decimal.Zero) {
				childOrder.CouponID = &result.AppliedCoupon.ID
			}
			items := []orderdomain.OrderItem{plan.Item}
			if err := orderStore.Create(childOrder, items); err != nil {
				return err
			}
			if len(items) > 0 {
				result.Plans[idx].Item = items[0]
				plan.Item = items[0]
				if pricingCtx != nil {
					pricingCtx.BindCreatedOrderItem(idx, childOrder.ID, items[0].ID)
				}
			}

			if strings.TrimSpace(plan.Item.FulfillmentType) == constants.FulfillmentTypeAuto {
				secretRepo := tx.CardSecrets()
				rows, err := secretRepo.ListAvailableByProductForUpdate(plan.Item.ProductID, plan.Item.SKUID, plan.Item.Quantity)
				if err != nil {
					return err
				}
				if len(rows) < plan.Item.Quantity {
					return ErrCardSecretInsufficient
				}
				ids := make([]uint, 0, len(rows))
				for _, row := range rows {
					ids = append(ids, row.ID)
				}
				affected, err := secretRepo.Reserve(ids, childOrder.ID, now)
				if err != nil {
					return err
				}
				if int(affected) != len(ids) {
					return ErrCardSecretInsufficient
				}
			}
			if strings.TrimSpace(plan.Item.FulfillmentType) == constants.FulfillmentTypeManual &&
				plan.SKU != nil &&
				productdomain.ShouldEnforceManualSKUStock(plan.Product, plan.SKU) {
				affected, err := productSKURepo.ReserveManualStock(plan.Item.SKUID, plan.Item.Quantity)
				if err != nil {
					return err
				}
				if affected == 0 {
					return ErrManualStockInsufficient
				}
			}
		}

		if result.AppliedCoupon != nil {
			couponRepo := tx.Coupons()
			usageRepo := tx.CouponUsages()
			usage := &coupondomain.CouponUsage{
				CouponID:       result.AppliedCoupon.ID,
				UserID:         input.UserID,
				OrderID:        order.ID,
				DiscountAmount: money.FromDecimal(result.DiscountAmount),
				CreatedAt:      now,
			}
			if err := usageRepo.Create(usage); err != nil {
				return err
			}
			if err := couponRepo.IncrementUsedCount(result.AppliedCoupon.ID, 1); err != nil {
				return err
			}
		}
		if pricingCtx != nil {
			resellerRepo := tx.ResellerOrders()
			if err := resellerRepo.CreateOrderSnapshot(pricingCtx.BuildSnapshot(order.ID, now)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		for _, riskErr := range []error{
			orderriskcontract.ErrClientIPUnavailable,
			orderriskcontract.ErrTooManyPendingOrders,
			orderriskcontract.ErrProductQuantityLimit,
			orderriskcontract.ErrPendingProductQuantityLimit,
			orderriskcontract.ErrOrderRateLimited,
		} {
			if errors.Is(err, riskErr) {
				return nil, err
			}
		}
		if errors.Is(err, ErrCardSecretInsufficient) {
			return nil, ErrCardSecretInsufficient
		}
		if errors.Is(err, ErrManualStockInsufficient) {
			return nil, ErrManualStockInsufficient
		}
		return nil, ErrOrderCreateFailed
	}

	if s.queueClient != nil {
		if err := s.queueClient.EnqueueTimeoutCancel(order.ID, time.Duration(expireMinutes)*time.Minute); err != nil {
			logger.Errorw("order_enqueue_timeout_cancel_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
			full, fetchErr := s.orderStore.GetByID(order.ID)
			if fetchErr != nil {
				logger.Errorw("order_fetch_for_timeout_rollback_failed",
					"order_id", order.ID,
					"order_no", order.OrderNo,
					"error", fetchErr,
				)
			} else if full != nil {
				if cancelErr := s.cancelOrderWithChildren(full, true); cancelErr != nil {
					logger.Errorw("order_timeout_rollback_cancel_failed",
						"order_id", order.ID,
						"order_no", order.OrderNo,
						"error", cancelErr,
					)
				}
			}
			return nil, ErrQueueUnavailable
		}
	}

	full, err := s.orderStore.GetByID(order.ID)
	if err == nil && full != nil {
		FillOrderItemsFromChildren(full)
		return full, nil
	}
	FillOrderItemsFromChildren(order)
	return order, nil
}

func (s *OrderService) checkOrderRisk(input *orderCreateParams, consumeRateLimit bool) error {
	if input == nil {
		return nil
	}
	input.ClientIP = strings.TrimSpace(input.ClientIP)
	input.RiskIP = orderriskcontract.NormalizeRiskIP(input.ClientIP)
	input.RiskPaymentExpireMinutes = 0
	input.RiskCheckResult = orderriskcontract.CheckResult{}
	if s.riskControlSvc == nil || input.SkipRiskControl {
		return nil
	}
	result, err := s.riskControlSvc.CheckOrderAllowed(buildRiskCheckInput(*input, consumeRateLimit))
	if err != nil {
		return err
	}
	input.RiskIP = result.RiskIP
	input.RiskPaymentExpireMinutes = result.PaymentExpireMinutes
	input.RiskCheckResult = result
	return nil
}

func buildRiskCheckInput(input orderCreateParams, consumeRateLimit bool) orderriskcontract.CheckInput {
	items := make([]orderriskcontract.OrderItem, 0, len(input.Items))
	for _, item := range input.Items {
		items = append(items, orderriskcontract.OrderItem{ProductID: item.ProductID, Quantity: item.Quantity})
	}
	return orderriskcontract.CheckInput{
		UserID:           input.UserID,
		ClientIP:         input.ClientIP,
		RiskIP:           input.RiskIP,
		IsGuest:          input.IsGuest,
		SkipIPCheck:      input.SkipIPRiskControl,
		ConsumeRateLimit: consumeRateLimit,
		Items:            items,
	}
}

func generateOrderNo() string {
	return serial.Generate("DJ")
}
