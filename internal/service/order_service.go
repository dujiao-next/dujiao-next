package service

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/repository"

	"github.com/hibiken/asynq"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// OrderService 订单服务
type OrderService struct {
	orderRepo               repository.OrderRepository
	orderRefundRecordRepo   repository.OrderRefundRecordRepository
	paymentRepo             repository.PaymentRepository
	userRepo                repository.UserRepository
	productRepo             repository.ProductRepository
	productSKURepo          repository.ProductSKURepository
	cardSecretRepo          repository.CardSecretRepository
	resellerRepo            repository.ResellerRepository
	couponRepo              repository.CouponRepository
	couponUsageRepo         repository.CouponUsageRepository
	promotionRepo           repository.PromotionRepository
	queueClient             orderQueueClient
	settingService          *SettingService
	defaultEmailConfig      config.EmailConfig
	walletService           *WalletService
	affiliateSvc            *AffiliateService
	memberLevelService      *MemberLevelService
	resellerPricingResolver *ResellerPricingResolver
	resellerAccountingSvc   *ResellerAccountingService
	riskControlSvc          *OrderRiskControlService
	productMappingService   *ProductMappingService
	expireMinutes           int
}

type orderQueueClient interface {
	Enabled() bool
	EnqueueOrderTimeoutCancel(payload queue.OrderTimeoutCancelPayload, delay time.Duration) error
	EnqueueOrderStatusEmail(payload queue.OrderStatusEmailPayload, opts ...asynq.Option) error
}

// OrderServiceOptions 订单服务构造参数
type OrderServiceOptions struct {
	OrderRepo                 repository.OrderRepository
	OrderRefundRecordRepo     repository.OrderRefundRecordRepository
	PaymentRepo               repository.PaymentRepository
	UserRepo                  repository.UserRepository
	ProductRepo               repository.ProductRepository
	ProductSKURepo            repository.ProductSKURepository
	CardSecretRepo            repository.CardSecretRepository
	ResellerRepo              repository.ResellerRepository
	CouponRepo                repository.CouponRepository
	CouponUsageRepo           repository.CouponUsageRepository
	PromotionRepo             repository.PromotionRepository
	QueueClient               *queue.Client
	SettingService            *SettingService
	DefaultEmailConfig        config.EmailConfig
	WalletService             *WalletService
	AffiliateService          *AffiliateService
	MemberLevelService        *MemberLevelService
	ResellerPricingResolver   *ResellerPricingResolver
	ResellerAccountingService *ResellerAccountingService
	RiskControlService        *OrderRiskControlService
	ProductMappingService     *ProductMappingService
	ExpireMinutes             int
}

// SetProductMappingService 注入商品映射服务（用于下单前上游库存兜底校验）。
// 由 provider 在 ProductMappingService 构造之后调用，避免构造顺序耦合。
func (s *OrderService) SetProductMappingService(svc *ProductMappingService) {
	if s == nil {
		return
	}
	s.productMappingService = svc
}

// NewOrderService 创建订单服务
func NewOrderService(opts OrderServiceOptions) *OrderService {
	return &OrderService{
		orderRepo:               opts.OrderRepo,
		orderRefundRecordRepo:   opts.OrderRefundRecordRepo,
		paymentRepo:             opts.PaymentRepo,
		userRepo:                opts.UserRepo,
		productRepo:             opts.ProductRepo,
		productSKURepo:          opts.ProductSKURepo,
		cardSecretRepo:          opts.CardSecretRepo,
		resellerRepo:            opts.ResellerRepo,
		couponRepo:              opts.CouponRepo,
		couponUsageRepo:         opts.CouponUsageRepo,
		promotionRepo:           opts.PromotionRepo,
		queueClient:             opts.QueueClient,
		settingService:          opts.SettingService,
		defaultEmailConfig:      opts.DefaultEmailConfig,
		walletService:           opts.WalletService,
		affiliateSvc:            opts.AffiliateService,
		memberLevelService:      opts.MemberLevelService,
		resellerPricingResolver: opts.ResellerPricingResolver,
		resellerAccountingSvc:   opts.ResellerAccountingService,
		riskControlSvc:          opts.RiskControlService,
		productMappingService:   opts.ProductMappingService,
		expireMinutes:           opts.ExpireMinutes,
	}
}

// CreateOrderInput 创建订单输入
type CreateOrderInput struct {
	UserID              uint
	Tenant              TenantContext
	Items               []CreateOrderItem
	CouponCode          string
	AffiliateCode       string
	AffiliateVisitorKey string
	ClientIP            string
	ManualFormData      map[string]models.JSON
	SkipRiskControl     bool // 完全跳过风控（下游订单）
	SkipIPRiskControl   bool // 跳过 IP 维度风控（渠道/Bot 订单）
}

// CreateGuestOrderInput 游客创建订单输入
type CreateGuestOrderInput struct {
	Email               string
	OrderPassword       string
	Locale              string
	Tenant              TenantContext
	Items               []CreateOrderItem
	CouponCode          string
	AffiliateCode       string
	AffiliateVisitorKey string
	ClientIP            string
	ManualFormData      map[string]models.JSON
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
	Product           *models.Product
	SKU               *models.ProductSKU
	Item              models.OrderItem
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
		constants.OrderStatusFulfilled:          true,
		constants.OrderStatusCompleted:          true,
		constants.OrderStatusPartiallyRefunded:  true,
		constants.OrderStatusRefunded:           true,
	},
	constants.OrderStatusFulfilling: {
		constants.OrderStatusPartiallyDelivered: true,
		constants.OrderStatusDelivered:          true,
		constants.OrderStatusFulfilled:          true,
		constants.OrderStatusCompleted:          true,
		constants.OrderStatusPartiallyRefunded:  true,
		constants.OrderStatusRefunded:           true,
	},
	constants.OrderStatusPartiallyDelivered: {
		constants.OrderStatusDelivered:         true,
		constants.OrderStatusFulfilled:         true,
		constants.OrderStatusCompleted:         true,
		constants.OrderStatusPartiallyRefunded: true,
		constants.OrderStatusRefunded:          true,
	},
	constants.OrderStatusDelivered: {
		constants.OrderStatusFulfilled:         true,
		constants.OrderStatusCompleted:         true,
		constants.OrderStatusPartiallyRefunded: true,
		constants.OrderStatusRefunded:          true,
	},
	constants.OrderStatusFulfilled: {
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
func (s *OrderService) CreateOrder(input CreateOrderInput) (*models.Order, error) {
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
func (s *OrderService) CreateGuestOrder(input CreateGuestOrderInput) (*models.Order, error) {
	email, err := normalizeGuestEmail(input.Email)
	if err != nil {
		return nil, err
	}
	password := strings.TrimSpace(input.OrderPassword)
	if password == "" {
		return nil, ErrGuestPasswordRequired
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
	UserID              uint
	GuestEmail          string
	GuestPassword       string
	GuestLocale         string
	Tenant              TenantContext
	Items               []CreateOrderItem
	CouponCode          string
	AffiliateCode       string
	AffiliateVisitorKey string
	ClientIP            string
	IsGuest             bool
	ManualFormData      map[string]models.JSON
	SkipManualFormCheck bool
	SkipRiskControl     bool
	SkipIPRiskControl   bool
}

// OrderPreview 订单金额预览
type OrderPreview struct {
	Currency                string             `json:"currency"`
	OriginalAmount          models.Money       `json:"original_amount"`
	MemberDiscountAmount    models.Money       `json:"member_discount_amount"`
	DiscountAmount          models.Money       `json:"discount_amount"`
	PromotionDiscountAmount models.Money       `json:"promotion_discount_amount"`
	WholesaleDiscountAmount models.Money       `json:"wholesale_discount_amount"`
	TotalAmount             models.Money       `json:"total_amount"`
	Items                   []OrderPreviewItem `json:"items"`
}

// OrderPreviewItem 订单项金额预览
type OrderPreviewItem struct {
	ProductID          uint               `json:"product_id"`
	SKUID              uint               `json:"sku_id"`
	TitleJSON          models.JSON        `json:"title"`
	SKUSnapshotJSON    models.JSON        `json:"sku_snapshot"`
	Tags               models.StringArray `json:"tags"`
	OriginalUnitPrice  models.Money       `json:"original_unit_price"`
	UnitPrice          models.Money       `json:"unit_price"`
	Quantity           int                `json:"quantity"`
	OriginalTotalPrice models.Money       `json:"original_total_price"`
	TotalPrice         models.Money       `json:"total_price"`
	MemberDiscount     models.Money       `json:"member_discount_amount"`
	CouponDiscount     models.Money       `json:"coupon_discount_amount"`
	PromotionDiscount  models.Money       `json:"promotion_discount_amount"`
	WholesaleDiscount  models.Money       `json:"wholesale_discount_amount"`
	FulfillmentType    string             `json:"fulfillment_type"`
}

type orderBuildResult struct {
	Plans                   []childOrderPlan
	OrderItems              []models.OrderItem
	OriginalAmount          decimal.Decimal
	MemberDiscountAmount    decimal.Decimal
	PromotionDiscountAmount decimal.Decimal
	WholesaleDiscountAmount decimal.Decimal
	DiscountAmount          decimal.Decimal
	TotalAmount             decimal.Decimal
	Currency                string
	OrderPromotionID        *uint
	MemberLevelID           *uint
	AppliedCoupon           *models.Coupon
}

// PreviewOrder 用户订单金额预览
func (s *OrderService) PreviewOrder(input CreateOrderInput) (*OrderPreview, error) {
	if input.UserID == 0 {
		return nil, ErrInvalidOrderItem
	}
	return s.previewOrder(orderCreateParams{
		UserID:              input.UserID,
		Tenant:              input.Tenant,
		Items:               input.Items,
		CouponCode:          input.CouponCode,
		AffiliateCode:       input.AffiliateCode,
		AffiliateVisitorKey: input.AffiliateVisitorKey,
		ClientIP:            input.ClientIP,
		ManualFormData:      input.ManualFormData,
		SkipManualFormCheck: true,
	})
}

// PreviewGuestOrder 游客订单金额预览
func (s *OrderService) PreviewGuestOrder(input CreateGuestOrderInput) (*OrderPreview, error) {
	return s.previewOrder(orderCreateParams{
		GuestEmail:          input.Email,
		GuestPassword:       input.OrderPassword,
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
	})
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
		OriginalAmount:          models.NewMoneyFromDecimal(result.OriginalAmount),
		MemberDiscountAmount:    models.NewMoneyFromDecimal(result.MemberDiscountAmount),
		DiscountAmount:          models.NewMoneyFromDecimal(result.DiscountAmount),
		PromotionDiscountAmount: models.NewMoneyFromDecimal(result.PromotionDiscountAmount),
		WholesaleDiscountAmount: models.NewMoneyFromDecimal(result.WholesaleDiscountAmount),
		TotalAmount:             models.NewMoneyFromDecimal(result.TotalAmount),
		Items:                   items,
	}, nil
}

func (s *OrderService) createOrder(input orderCreateParams) (*models.Order, error) {
	if s.queueClient == nil || !s.queueClient.Enabled() {
		return nil, ErrQueueUnavailable
	}

	// 风控检查（在锁库存之前）
	if s.riskControlSvc != nil && !input.SkipRiskControl {
		if err := s.riskControlSvc.CheckOrderAllowed(RiskCheckInput{
			UserID:      input.UserID,
			GuestEmail:  input.GuestEmail,
			ClientIP:    input.ClientIP,
			IsGuest:     input.IsGuest,
			SkipIPCheck: input.SkipIPRiskControl,
		}); err != nil {
			return nil, err
		}
	}

	result, err := s.buildOrderResult(input)
	if err != nil {
		return nil, err
	}
	var pricingCtx *ResellerOrderPricingContext
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
			return nil, ErrWalletOnlyPaymentRequired
		}
		if s.walletService == nil {
			return nil, ErrWalletOnlyPaymentRequired
		}
		account, accErr := s.walletService.GetAccount(input.UserID)
		if accErr != nil {
			return nil, ErrWalletOnlyPaymentRequired
		}
		if account.Balance.Decimal.LessThan(result.TotalAmount) {
			return nil, ErrWalletInsufficientBalance
		}
	}

	affiliateCode := normalizeAffiliateCode(input.AffiliateCode)
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
	if input.IsGuest && input.GuestPassword == "" {
		return nil, ErrGuestPasswordRequired
	}

	expireMinutes := s.resolveExpireMinutes()
	now := time.Now()
	expiresAt := now.Add(time.Duration(expireMinutes) * time.Minute)
	order := &models.Order{
		OrderNo:                 generateOrderNo(),
		UserID:                  input.UserID,
		GuestEmail:              input.GuestEmail,
		GuestPassword:           input.GuestPassword,
		GuestLocale:             input.GuestLocale,
		Status:                  constants.OrderStatusPendingPayment,
		Currency:                result.Currency,
		OriginalAmount:          models.NewMoneyFromDecimal(result.OriginalAmount),
		MemberDiscountAmount:    models.NewMoneyFromDecimal(result.MemberDiscountAmount),
		DiscountAmount:          models.NewMoneyFromDecimal(result.DiscountAmount),
		PromotionDiscountAmount: models.NewMoneyFromDecimal(result.PromotionDiscountAmount),
		WholesaleDiscountAmount: models.NewMoneyFromDecimal(result.WholesaleDiscountAmount),
		TotalAmount:             models.NewMoneyFromDecimal(result.TotalAmount),
		WalletPaidAmount:        models.NewMoneyFromDecimal(decimal.Zero),
		OnlinePaidAmount:        models.NewMoneyFromDecimal(result.TotalAmount),
		RefundedAmount:          models.NewMoneyFromDecimal(decimal.Zero),
		MemberLevelID:           result.MemberLevelID,
		CouponID:                nil,
		PromotionID:             result.OrderPromotionID,
		AffiliateProfileID:      affiliateProfileID,
		AffiliateCode:           affiliateCode,
		ExpiresAt:               &expiresAt,
		ClientIP:                strings.TrimSpace(input.ClientIP),
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if pricingCtx != nil {
		resellerID := pricingCtx.ResellerID
		order.ResellerID = &resellerID
		order.ResellerDomain = pricingCtx.Domain
		order.ResellerProfitAmount = models.NewMoneyFromDecimal(pricingCtx.EffectiveProfit)
	}

	if result.AppliedCoupon != nil {
		order.CouponID = &result.AppliedCoupon.ID
	}

	err = s.orderRepo.Transaction(func(tx *gorm.DB) error {
		orderRepo := s.orderRepo.WithTx(tx)
		var productSKURepo repository.ProductSKURepository
		if s.productSKURepo != nil {
			productSKURepo = s.productSKURepo.WithTx(tx)
		}
		if err := orderRepo.Create(order, nil); err != nil {
			return err
		}

		for idx := range result.Plans {
			plan := result.Plans[idx]
			childProfit := decimal.Zero
			if pricingCtx != nil && idx < len(pricingCtx.Items) && pricingCtx.ProfitEligible {
				childProfit = pricingCtx.Items[idx].ProfitAmount
			}
			childOrder := &models.Order{
				OrderNo:                 buildChildOrderNo(order.OrderNo, idx+1),
				ParentID:                &order.ID,
				UserID:                  order.UserID,
				GuestEmail:              order.GuestEmail,
				GuestPassword:           order.GuestPassword,
				GuestLocale:             order.GuestLocale,
				Status:                  constants.OrderStatusPendingPayment,
				Currency:                plan.Currency,
				OriginalAmount:          models.NewMoneyFromDecimal(plan.TotalAmount),
				MemberDiscountAmount:    models.NewMoneyFromDecimal(plan.MemberDiscount),
				DiscountAmount:          models.NewMoneyFromDecimal(plan.CouponDiscount),
				PromotionDiscountAmount: models.NewMoneyFromDecimal(plan.PromotionDiscount),
				WholesaleDiscountAmount: models.NewMoneyFromDecimal(plan.WholesaleDiscount),
				TotalAmount:             models.NewMoneyFromDecimal(normalizeOrderAmount(plan.TotalAmount.Sub(plan.CouponDiscount))),
				WalletPaidAmount:        models.NewMoneyFromDecimal(decimal.Zero),
				OnlinePaidAmount:        models.NewMoneyFromDecimal(normalizeOrderAmount(plan.TotalAmount.Sub(plan.CouponDiscount))),
				RefundedAmount:          models.NewMoneyFromDecimal(decimal.Zero),
				CouponID:                nil,
				PromotionID:             plan.Item.PromotionID,
				AffiliateProfileID:      affiliateProfileID,
				AffiliateCode:           affiliateCode,
				ExpiresAt:               &expiresAt,
				ClientIP:                order.ClientIP,
				CreatedAt:               now,
				UpdatedAt:               now,
			}
			if pricingCtx != nil {
				resellerID := pricingCtx.ResellerID
				childOrder.ResellerID = &resellerID
				childOrder.ResellerDomain = pricingCtx.Domain
				childOrder.ResellerProfitAmount = models.NewMoneyFromDecimal(childProfit)
			}
			if result.AppliedCoupon != nil && plan.CouponDiscount.GreaterThan(decimal.Zero) {
				childOrder.CouponID = &result.AppliedCoupon.ID
			}
			items := []models.OrderItem{plan.Item}
			if err := orderRepo.Create(childOrder, items); err != nil {
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
				if s.cardSecretRepo == nil {
					return ErrCardSecretInsufficient
				}
				secretRepo := s.cardSecretRepo.WithTx(tx)
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
				shouldEnforceManualSKUStock(plan.Product, plan.SKU) {
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
			couponRepo := s.couponRepo.WithTx(tx)
			usageRepo := s.couponUsageRepo.WithTx(tx)
			usage := &models.CouponUsage{
				CouponID:       result.AppliedCoupon.ID,
				UserID:         input.UserID,
				OrderID:        order.ID,
				DiscountAmount: models.NewMoneyFromDecimal(result.DiscountAmount),
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
			if s.resellerRepo == nil {
				return ErrOrderCreateFailed
			}
			resellerRepo := s.resellerRepo.WithTx(tx)
			if err := resellerRepo.CreateOrderSnapshot(pricingCtx.BuildSnapshot(order.ID, now)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCardSecretInsufficient) {
			return nil, ErrCardSecretInsufficient
		}
		if errors.Is(err, ErrManualStockInsufficient) {
			return nil, ErrManualStockInsufficient
		}
		return nil, ErrOrderCreateFailed
	}

	if s.queueClient != nil {
		if err := s.queueClient.EnqueueOrderTimeoutCancel(queue.OrderTimeoutCancelPayload{
			OrderID: order.ID,
		}, time.Duration(expireMinutes)*time.Minute); err != nil {
			logger.Errorw("order_enqueue_timeout_cancel_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
			full, fetchErr := s.orderRepo.GetByID(order.ID)
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

	full, err := s.orderRepo.GetByID(order.ID)
	if err == nil && full != nil {
		fillOrderItemsFromChildren(full)
		return full, nil
	}
	fillOrderItemsFromChildren(order)
	return order, nil
}

func generateOrderNo() string {
	return generateSerialNo("DJ")
}
