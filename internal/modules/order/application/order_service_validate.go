package application

import (
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	couponapp "github.com/dujiao-next/internal/modules/coupon/application"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	promotionapp "github.com/dujiao-next/internal/modules/promotion/application"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func (s *OrderService) buildOrderResult(input orderCreateParams) (*orderBuildResult, error) {
	if len(input.Items) == 0 {
		return nil, ErrInvalidOrderItem
	}
	if input.IsGuest && input.GuestEmail == "" {
		return nil, ErrGuestEmailRequired
	}
	if input.IsGuest {
		if _, err := normalizeGuestPassword(input.GuestPassword); err != nil {
			return nil, err
		}
	}
	resellerOrder := isResellerOrderContext(input.Tenant)
	if resellerOrder && strings.TrimSpace(input.CouponCode) != "" {
		return nil, ErrResellerCouponNotAllowed
	}

	mergedItems, err := mergeCreateOrderItems(input.Items)
	if err != nil {
		return nil, err
	}
	if len(mergedItems) == 0 {
		return nil, ErrInvalidOrderItem
	}

	var plans []childOrderPlan
	var orderItems []orderdomain.OrderItem
	originalAmount := decimal.Zero
	memberDiscountAmount := decimal.Zero
	promotionDiscountAmount := decimal.Zero
	wholesaleDiscountAmount := decimal.Zero
	currency := resolveSiteCurrency(s.settingService)
	now := time.Now()
	var promotionIDValue uint
	var promotionSeen bool
	promotionSame := true
	var noPromotionSeen bool
	productQuantityTotals := make(map[uint]int, len(mergedItems))
	for _, item := range mergedItems {
		if item.ProductID == 0 || item.Quantity <= 0 {
			continue
		}
		productQuantityTotals[item.ProductID] += item.Quantity
	}

	// 解析用户会员等级
	var userMemberLevelID uint
	var memberLevelIDSnapshot *uint
	if !resellerOrder && input.UserID > 0 && s.userRepo != nil {
		user, _ := s.userRepo.GetByID(input.UserID)
		if user != nil && user.MemberLevelID > 0 {
			userMemberLevelID = user.MemberLevelID
			lid := user.MemberLevelID
			memberLevelIDSnapshot = &lid
		}
	}

	var promotionService *promotionapp.Service
	if !resellerOrder {
		promotionService = promotionapp.NewService(s.promotionRepo)
	}
	manualFormData := input.ManualFormData
	if manualFormData == nil {
		manualFormData = map[string]jsonmap.JSON{}
	}
	for _, item := range mergedItems {
		if item.ProductID == 0 || item.Quantity <= 0 {
			return nil, ErrInvalidOrderItem
		}
		product, err := s.productRepo.GetByID(strconv.FormatUint(uint64(item.ProductID), 10))
		if err != nil {
			return nil, err
		}
		if product == nil || !product.IsActive {
			return nil, ErrProductNotAvailable
		}
		if err := productdomain.ValidatePurchaseQuantity(product, item.Quantity); err != nil {
			return nil, err
		}
		purchaseType := strings.TrimSpace(product.PurchaseType)
		if purchaseType == "" {
			purchaseType = constants.ProductPurchaseMember
		}
		if input.IsGuest && purchaseType == constants.ProductPurchaseMember {
			return nil, ErrProductPurchaseNotAllowed
		}
		sku, err := resolveProductOrderSKU(s.productSKURepo, product, item.SKUID)
		if err != nil {
			return nil, err
		}

		productCurrency := currency
		basePrice := sku.PriceAmount.Decimal.Round(2)

		// 1. 计算活动价
		priceCarrier := *product
		priceCarrier.PriceAmount = sku.PriceAmount
		var promotion *promotiondomain.Promotion
		promoUnitPriceAmount := basePrice
		if promotionService != nil {
			var promoUnitPrice money.Amount
			promotion, promoUnitPrice, err = promotionService.ApplyPromotion(&priceCarrier, item.Quantity)
			if err != nil {
				return nil, err
			}
			promoUnitPriceAmount = promoUnitPrice.Decimal.Round(2)
		}

		// 2. 活动价与批发价取更优单价，避免两个商品级阶梯价叠加造成不可预期折扣。
		unitPriceAmount := promoUnitPriceAmount
		promotionDiscount := decimal.Zero
		if promotion != nil && basePrice.GreaterThan(promoUnitPriceAmount) {
			promotionDiscount = basePrice.Sub(promoUnitPriceAmount).
				Mul(decimal.NewFromInt(int64(item.Quantity))).
				Round(2)
		}

		wholesaleMatchQuantity := productQuantityTotals[item.ProductID]
		if wholesaleMatchQuantity <= 0 {
			wholesaleMatchQuantity = item.Quantity
		}
		var wholesaleUnitPrice decimal.Decimal
		wholesaleDiscount := decimal.Zero
		wholesaleMatched := false
		if !resellerOrder {
			wholesaleUnitPrice, wholesaleDiscount, wholesaleMatched = productdomain.ResolveWholesaleUnitPriceForSKU(product, basePrice, sku.ID, sku.SKUCode, wholesaleMatchQuantity, item.Quantity)
		}
		if wholesaleMatched && wholesaleUnitPrice.LessThan(unitPriceAmount) {
			unitPriceAmount = wholesaleUnitPrice
			promotion = nil
			promotionDiscount = decimal.Zero
			wholesaleDiscountAmount = wholesaleDiscountAmount.Add(wholesaleDiscount).Round(2)
		} else if promotionDiscount.GreaterThan(decimal.Zero) {
			promotionDiscountAmount = promotionDiscountAmount.Add(promotionDiscount).Round(2)
			wholesaleDiscount = decimal.Zero
		} else {
			wholesaleDiscount = decimal.Zero
		}

		// 3. 在已命中的商品级优惠单价基础上应用会员价。
		itemMemberDiscount := decimal.Zero
		if !resellerOrder && userMemberLevelID > 0 && s.memberLevelService != nil {
			memberUnitPrice, _ := s.memberLevelService.ResolveMemberPrice(userMemberLevelID, product.ID, sku.ID, unitPriceAmount)
			if memberUnitPrice.LessThan(unitPriceAmount) {
				itemMemberDiscount = unitPriceAmount.Sub(memberUnitPrice).
					Mul(decimal.NewFromInt(int64(item.Quantity))).
					Round(2)
				memberDiscountAmount = memberDiscountAmount.Add(itemMemberDiscount).Round(2)
				unitPriceAmount = memberUnitPrice
			}
		}

		// 4. 兼容活动规则命中但未形成实际优惠的情况
		if promotion != nil && promotionDiscount.IsZero() && !basePrice.GreaterThan(promoUnitPriceAmount) {
			promotion = nil
		}

		if unitPriceAmount.LessThanOrEqual(decimal.Zero) || productCurrency == "" {
			return nil, ErrProductPriceInvalid
		}

		baseTotal := basePrice.Mul(decimal.NewFromInt(int64(item.Quantity))).Round(2)
		total := unitPriceAmount.Mul(decimal.NewFromInt(int64(item.Quantity))).Round(2)
		originalAmount = originalAmount.Add(baseTotal).Round(2)
		fulfillmentType := strings.TrimSpace(product.FulfillmentType)
		if fulfillmentType == "" {
			fulfillmentType = constants.FulfillmentTypeManual
		}
		if fulfillmentType != constants.FulfillmentTypeManual && fulfillmentType != constants.FulfillmentTypeAuto && fulfillmentType != constants.FulfillmentTypeUpstream {
			return nil, ErrFulfillmentInvalid
		}
		if fulfillmentType == constants.FulfillmentTypeManual &&
			productdomain.ShouldEnforceManualSKUStock(product, sku) &&
			productdomain.ManualSKUAvailable(sku) < item.Quantity {
			return nil, ErrManualStockInsufficient
		}
		if fulfillmentType == constants.FulfillmentTypeUpstream && s.productMappingService != nil {
			if err := s.productMappingService.EnsureUpstreamStockForOrder(sku.ID, item.Quantity); err != nil {
				return nil, err
			}
		}

		manualSchemaSnapshot := jsonmap.JSON{}
		manualSubmission := jsonmap.JSON{}
		if !input.SkipManualFormCheck && (fulfillmentType == constants.FulfillmentTypeManual ||
			(fulfillmentType == constants.FulfillmentTypeUpstream && len(product.ManualFormSchemaJSON) > 0)) {
			submission := resolveManualFormSubmission(manualFormData, product.ID, sku.ID)
			normalizedSchema, normalizedSubmission, err := manualform.ValidateAndNormalize(product.ManualFormSchemaJSON, submission)
			if err != nil {
				return nil, err
			}
			manualSchemaSnapshot = normalizedSchema
			manualSubmission = normalizedSubmission
		}

		var promotionID *uint
		if promotion != nil {
			pid := promotion.ID
			promotionID = &pid
			if !promotionSeen {
				promotionSeen = true
				promotionIDValue = pid
			} else if promotionIDValue != pid {
				promotionSame = false
			}
		} else {
			noPromotionSeen = true
		}

		orderItem := orderdomain.OrderItem{
			ProductID: product.ID,
			SKUID:     sku.ID,
			TitleJSON: product.TitleJSON,
			SKUSnapshotJSON: jsonmap.JSON{
				"sku_id":      sku.ID,
				"sku_code":    sku.SKUCode,
				"spec_values": sku.SpecValuesJSON,
				"image":       firstProductImage(product.Images),
			},
			Tags:                         product.Tags,
			OriginalUnitPrice:            money.FromDecimal(basePrice),
			UnitPrice:                    money.FromDecimal(unitPriceAmount),
			CostPrice:                    sku.CostPriceAmount, // 成本价快照
			Quantity:                     item.Quantity,
			OriginalTotalPrice:           money.FromDecimal(baseTotal),
			TotalPrice:                   money.FromDecimal(total),
			MemberDiscount:               money.FromDecimal(itemMemberDiscount),
			CouponDiscount:               money.FromDecimal(decimal.Zero),
			PromotionDiscount:            money.FromDecimal(promotionDiscount),
			WholesaleDiscount:            money.FromDecimal(wholesaleDiscount),
			PromotionID:                  promotionID,
			FulfillmentType:              fulfillmentType,
			ManualFormSchemaSnapshotJSON: manualSchemaSnapshot,
			ManualFormSubmissionJSON:     manualSubmission,
			InstructionsJSON:             product.InstructionsJSON,
			CreatedAt:                    now,
			UpdatedAt:                    now,
		}
		orderItems = append(orderItems, orderItem)
		plans = append(plans, childOrderPlan{
			Product:           product,
			SKU:               sku,
			Item:              orderItem,
			TotalAmount:       total,
			MemberDiscount:    itemMemberDiscount,
			PromotionDiscount: promotionDiscount,
			WholesaleDiscount: wholesaleDiscount,
			Currency:          productCurrency,
		})
	}
	if currency == "" {
		return nil, ErrInvalidOrderAmount
	}

	var orderPromotionID *uint
	if promotionSeen && promotionSame && !noPromotionSeen {
		orderPromotionID = &promotionIDValue
	}

	discountAmount := decimal.Zero
	var appliedCoupon *coupondomain.Coupon
	couponCode := strings.TrimSpace(input.CouponCode)
	if !resellerOrder && couponCode != "" {
		couponService := couponapp.NewService(s.couponRepo, s.couponUsageRepo)
		couponItems := make([]couponcontract.EligibilityItem, 0, len(orderItems))
		for _, item := range orderItems {
			couponItems = append(couponItems, couponcontract.EligibilityItem{
				ProductID:         item.ProductID,
				Quantity:          item.Quantity,
				TotalPrice:        item.TotalPrice,
				WholesaleDiscount: item.WholesaleDiscount,
			})
		}
		discount, coupon, err := couponService.ApplyCoupon(
			money.FromDecimal(originalAmount),
			couponCode,
			input.UserID,
			couponItems,
			input.IsGuest,
			userMemberLevelID,
		)
		if err != nil {
			return nil, err
		}
		discountAmount = discount.Decimal.Round(2)
		appliedCoupon = coupon
	}

	if appliedCoupon != nil && discountAmount.GreaterThan(decimal.Zero) {
		if err := applyCouponDiscountToItems(plans, appliedCoupon, discountAmount); err != nil {
			return nil, err
		}
		discountAmount = decimal.Zero
		for i := range plans {
			discountAmount = discountAmount.Add(plans[i].CouponDiscount).Round(2)
		}
	}

	totalAmount := decimal.Zero
	for i := range plans {
		plan := &plans[i]
		plan.Item.MemberDiscount = money.FromDecimal(plan.MemberDiscount)
		plan.Item.CouponDiscount = money.FromDecimal(plan.CouponDiscount)
		plan.Item.PromotionDiscount = money.FromDecimal(plan.PromotionDiscount)
		plan.Item.WholesaleDiscount = money.FromDecimal(plan.WholesaleDiscount)
		plan.Item.TotalPrice = money.FromDecimal(plan.TotalAmount)
		planTotal := plan.TotalAmount.Sub(plan.CouponDiscount).Round(2)
		if planTotal.LessThan(decimal.Zero) {
			planTotal = decimal.Zero
		}
		totalAmount = totalAmount.Add(planTotal).Round(2)
	}
	if totalAmount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidOrderAmount
	}

	return &orderBuildResult{
		Plans:                   plans,
		OrderItems:              orderItems,
		OriginalAmount:          originalAmount,
		MemberDiscountAmount:    memberDiscountAmount,
		PromotionDiscountAmount: promotionDiscountAmount,
		WholesaleDiscountAmount: wholesaleDiscountAmount,
		DiscountAmount:          discountAmount,
		TotalAmount:             totalAmount,
		Currency:                currency,
		OrderPromotionID:        orderPromotionID,
		MemberLevelID:           memberLevelIDSnapshot,
		AppliedCoupon:           appliedCoupon,
	}, nil
}

func normalizeGuestEmail(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "", ErrGuestEmailRequired
	}
	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", ErrInvalidEmail
	}
	return normalized, nil
}

func (s *OrderService) resolveExpireMinutes() int {
	return ResolvePaymentExpireMinutes(s.settingService, s.expireMinutes)
}

func resolveManualFormSubmission(manualFormData map[string]jsonmap.JSON, productID, skuID uint) jsonmap.JSON {
	if len(manualFormData) == 0 || productID == 0 {
		return jsonmap.JSON{}
	}

	itemKey := orderdomain.ItemKey(productID, skuID)
	if submission, ok := manualFormData[itemKey]; ok {
		if submission == nil {
			return jsonmap.JSON{}
		}
		return submission
	}

	legacyKey := strconv.FormatUint(uint64(productID), 10)
	if submission, ok := manualFormData[legacyKey]; ok {
		if submission == nil {
			return jsonmap.JSON{}
		}
		return submission
	}

	return jsonmap.JSON{}
}

func firstProductImage(images jsonslice.Strings) string {
	for _, raw := range images {
		image := strings.TrimSpace(raw)
		if image != "" {
			return image
		}
	}
	return ""
}

// mergeCreateOrderItems 合并重复商品的下单项
func mergeCreateOrderItems(items []CreateOrderItem) ([]CreateOrderItem, error) {
	if len(items) == 0 {
		return nil, nil
	}
	merged := make([]CreateOrderItem, 0, len(items))
	indexMap := make(map[string]int)
	for _, item := range items {
		if item.ProductID == 0 || item.Quantity <= 0 {
			return nil, ErrInvalidOrderItem
		}
		key := orderdomain.ItemKey(item.ProductID, item.SKUID)
		if idx, ok := indexMap[key]; ok {
			merged[idx].Quantity += item.Quantity
			continue
		}
		indexMap[key] = len(merged)
		merged = append(merged, CreateOrderItem{
			ProductID: item.ProductID,
			SKUID:     item.SKUID,
			Quantity:  item.Quantity,
		})
	}
	return merged, nil
}

// applyCouponDiscountToItems 分摊优惠券折扣到订单项
func applyCouponDiscountToItems(plans []childOrderPlan, coupon *coupondomain.Coupon, discountAmount decimal.Decimal) error {
	if coupon == nil || discountAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	scopeType := strings.ToLower(strings.TrimSpace(coupon.ScopeType))
	if scopeType != constants.ScopeTypeProduct {
		return couponcontract.ErrScopeInvalid
	}
	ids, err := coupondomain.DecodeScopeIDs(coupon.ScopeRefIDs)
	if err != nil {
		return couponcontract.ErrScopeInvalid
	}
	eligibleIndexes := make([]int, 0, len(plans))
	eligibleTotal := decimal.Zero
	scopeMatched := 0
	wholesaleExcluded := 0
	for i := range plans {
		if _, ok := ids[plans[i].Item.ProductID]; !ok {
			continue
		}
		scopeMatched++
		if coupon.DisabledWholesalePrice && plans[i].WholesaleDiscount.GreaterThan(decimal.Zero) {
			wholesaleExcluded++
			continue
		}
		eligibleIndexes = append(eligibleIndexes, i)
		eligibleTotal = eligibleTotal.Add(plans[i].TotalAmount)
	}
	if len(eligibleIndexes) == 0 || eligibleTotal.LessThanOrEqual(decimal.Zero) {
		if scopeMatched > 0 && wholesaleExcluded == scopeMatched {
			return couponcontract.ErrWholesaleDisabled
		}
		return couponcontract.ErrScopeInvalid
	}

	remaining := discountAmount
	for i, idx := range eligibleIndexes {
		if i == len(eligibleIndexes)-1 {
			alloc := remaining.Round(2)
			if alloc.LessThan(decimal.Zero) {
				alloc = decimal.Zero
			}
			if alloc.GreaterThan(plans[idx].TotalAmount) {
				alloc = plans[idx].TotalAmount
			}
			plans[idx].CouponDiscount = alloc
			break
		}
		ratio := plans[idx].TotalAmount.Div(eligibleTotal)
		alloc := discountAmount.Mul(ratio).Round(2)
		if alloc.GreaterThan(remaining) {
			alloc = remaining
		}
		if alloc.LessThan(decimal.Zero) {
			alloc = decimal.Zero
		}
		if alloc.GreaterThan(plans[idx].TotalAmount) {
			alloc = plans[idx].TotalAmount
		}
		plans[idx].CouponDiscount = alloc
		remaining = remaining.Sub(alloc).Round(2)
	}
	return nil
}

// buildChildOrderNo 生成子订单号
func buildChildOrderNo(parentOrderNo string, seq int) string {
	if seq <= 0 {
		return parentOrderNo
	}
	return fmt.Sprintf("%s-%02d", parentOrderNo, seq)
}

// FillOrderItemsFromChildren 从子订单聚合订单项（用于响应兼容）
func FillOrderItemsFromChildren(order *orderdomain.Order) {
	if order == nil || len(order.Items) > 0 || len(order.Children) == 0 {
		return
	}
	items := make([]orderdomain.OrderItem, 0)
	for _, child := range order.Children {
		for _, item := range child.Items {
			copied := item
			copied.OrderID = order.ID
			items = append(items, copied)
		}
	}
	order.Items = items
}

// FillOrdersItemsFromChildren 批量填充聚合订单项
func FillOrdersItemsFromChildren(orders []orderdomain.Order) {
	for i := range orders {
		FillOrderItemsFromChildren(&orders[i])
	}
}
