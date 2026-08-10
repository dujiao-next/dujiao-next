package application_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	. "github.com/dujiao-next/internal/modules/order/application"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	resellergormstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"

	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	coupongormstore "github.com/dujiao-next/internal/modules/coupon/infrastructure/gormstore"
	promotiongormstore "github.com/dujiao-next/internal/modules/promotion/infrastructure/gormstore"
	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type fakeOrderTimeoutQueue struct {
	enqueued     int
	statusEmails []string
}

func (q *fakeOrderTimeoutQueue) Enabled() bool {
	return true
}

func (q *fakeOrderTimeoutQueue) EnqueueTimeoutCancel(_ uint, _ time.Duration) error {
	q.enqueued++
	return nil
}

func (q *fakeOrderTimeoutQueue) EnqueueStatusEmail(_ uint, status string) error {
	q.statusEmails = append(q.statusEmails, status)
	return nil
}

type orderResellerSnapshotFixture struct {
	db           *gorm.DB
	svc          *OrderService
	resellerRepo *resellergormstore.Store
	queue        *fakeOrderTimeoutQueue
	owner        userdomain.User
	buyer        userdomain.User
	profile      resellerdomain.Profile
	product      productdomain.Product
	sku          productdomain.ProductSKU
	tenant       resellercontract.TenantContext
}

func newOrderResellerSnapshotFixture(t *testing.T) orderResellerSnapshotFixture {
	t.Helper()

	dsn := fmt.Sprintf("file:order_reseller_snapshot_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&promotiondomain.Promotion{},
		&paymentdomain.Payment{},
		&resellerdomain.Profile{},
		&resellerdomain.ProductSetting{},
		&resellerdomain.RelatedAccount{},
		&resellerdomain.OrderSnapshot{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	category := categorydomain.Category{Slug: "reseller-order", NameJSON: jsonmap.JSON{"zh-CN": "reseller-order"}, IsActive: true}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	owner := userdomain.User{Email: "reseller-order-owner@example.com", PasswordHash: "hash"}
	buyer := userdomain.User{Email: "reseller-order-buyer@example.com", PasswordHash: "hash"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner failed: %v", err)
	}
	if err := db.Create(&buyer).Error; err != nil {
		t.Fatalf("create buyer failed: %v", err)
	}
	profile := resellerdomain.Profile{
		UserID:               owner.ID,
		Status:               resellerdomain.ProfileStatusActive,
		DefaultMarkupPercent: money.FromDecimal(decimal.NewFromInt(20)),
		MaxMarkupPercent:     money.FromDecimal(decimal.NewFromInt(80)),
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "reseller-order-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "reseller-order-product"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		WholesalePrices: productdomain.WholesalePriceTiers{{MinQuantity: 2, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))}},
		PurchaseType:    constants.ProductPurchaseGuest,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID:        product.ID,
		SKUCode:          productdomain.DefaultSKUCode,
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		CostPriceAmount:  money.FromDecimal(decimal.NewFromInt(50)),
		ManualStockTotal: constants.ManualStockUnlimited,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	setting := resellerdomain.ProductSetting{
		ResellerID:       profile.ID,
		ProductID:        product.ID,
		SKUID:            sku.ID,
		IsListed:         true,
		PricingMode:      resellerdomain.PricingModeFixedPrice,
		FixedPriceAmount: money.FromDecimal(decimal.NewFromInt(130)),
	}
	if err := db.Create(&setting).Error; err != nil {
		t.Fatalf("create reseller setting failed: %v", err)
	}
	startsAt := time.Now().Add(-time.Hour)
	endsAt := time.Now().Add(time.Hour)
	promotion := promotiondomain.Promotion{
		Name:       "main-promotion",
		ScopeType:  constants.ScopeTypeProduct,
		ScopeRefID: product.ID,
		Type:       constants.PromotionTypeFixed,
		Value:      money.FromDecimal(decimal.NewFromInt(40)),
		MinAmount:  money.FromDecimal(decimal.Zero),
		IsActive:   true,
		StartsAt:   &startsAt,
		EndsAt:     &endsAt,
	}
	if err := db.Create(&promotion).Error; err != nil {
		t.Fatalf("create promotion failed: %v", err)
	}

	resellerRepo := resellergormstore.New(db)
	orderStore := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	q := &fakeOrderTimeoutQueue{}
	svc := NewOrderService(OrderServiceOptions{
		OrderStore:              orderStore,
		UserStore:               userstore.New(db),
		ProductStore:            productgormstore.NewProductStore(db),
		ProductSKUStore:         productgormstore.NewSKUStore(db),
		CouponStore:             coupongormstore.New(db),
		CouponUsageStore:        coupongormstore.NewUsageStore(db),
		PromotionRepo:           promotiongormstore.New(db),
		Queue:                   q,
		ExpireMinutes:           15,
		ResellerPricingResolver: NewResellerPricingResolver(resellerRepo),
	})
	tenant := resellercontract.ResellerTenantContext("alias.example.test", profile.ID, owner.ID, "primary.example.test")
	return orderResellerSnapshotFixture{
		db:           db,
		svc:          svc,
		resellerRepo: resellerRepo,
		queue:        q,
		owner:        owner,
		buyer:        buyer,
		profile:      profile,
		product:      product,
		sku:          sku,
		tenant:       tenant,
	}
}

func (f orderResellerSnapshotFixture) addResellerSnapshotProduct(t *testing.T, slug string, base decimal.Decimal, cost decimal.Decimal, fixedMarkup decimal.Decimal) (productdomain.Product, productdomain.ProductSKU) {
	t.Helper()
	product := productdomain.Product{
		CategoryID:      f.product.CategoryID,
		Slug:            slug,
		TitleJSON:       jsonmap.JSON{"zh-CN": slug},
		PriceAmount:     money.FromDecimal(base),
		PurchaseType:    constants.ProductPurchaseGuest,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := f.db.Create(&product).Error; err != nil {
		t.Fatalf("create extra product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID:        product.ID,
		SKUCode:          productdomain.DefaultSKUCode,
		PriceAmount:      money.FromDecimal(base),
		CostPriceAmount:  money.FromDecimal(cost),
		ManualStockTotal: constants.ManualStockUnlimited,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := f.db.Create(&sku).Error; err != nil {
		t.Fatalf("create extra sku failed: %v", err)
	}
	if fixedMarkup.GreaterThanOrEqual(decimal.Zero) {
		setting := resellerdomain.ProductSetting{
			ResellerID:        f.profile.ID,
			ProductID:         product.ID,
			SKUID:             0,
			IsListed:          true,
			PricingMode:       resellerdomain.PricingModeFixedMarkup,
			FixedMarkupAmount: money.FromDecimal(fixedMarkup),
		}
		if err := f.db.Create(&setting).Error; err != nil {
			t.Fatalf("create extra product-level setting failed: %v", err)
		}
	}
	return product, sku
}

func (f orderResellerSnapshotFixture) createInput(userID uint) CreateOrderInput {
	return CreateOrderInput{
		UserID: userID,
		Tenant: f.tenant,
		Items:  []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 1}},
	}
}

func TestPreviewOrderResellerUsesResellerFacingTotals(t *testing.T) {
	f := newOrderResellerSnapshotFixture(t)
	preview, err := f.svc.PreviewOrder(f.createInput(f.buyer.ID))
	if err != nil {
		t.Fatalf("PreviewOrder failed: %v", err)
	}
	if !preview.OriginalAmount.Decimal.Equal(decimal.NewFromInt(130)) || !preview.TotalAmount.Decimal.Equal(decimal.NewFromInt(130)) {
		t.Fatalf("expected reseller-facing total 130, got original=%s total=%s", preview.OriginalAmount.String(), preview.TotalAmount.String())
	}
	if !preview.PromotionDiscountAmount.Decimal.Equal(decimal.Zero) || !preview.WholesaleDiscountAmount.Decimal.Equal(decimal.Zero) || !preview.DiscountAmount.Decimal.Equal(decimal.Zero) {
		t.Fatalf("reseller preview should not expose main discounts: %+v", preview)
	}
	if len(preview.Items) != 1 || !preview.Items[0].UnitPrice.Decimal.Equal(decimal.NewFromInt(130)) || !preview.Items[0].OriginalUnitPrice.Decimal.Equal(decimal.NewFromInt(130)) {
		t.Fatalf("expected reseller-facing item prices, got %+v", preview.Items)
	}
	body, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("marshal preview failed: %v", err)
	}
	for _, forbidden := range []string{"reseller_id", "reseller_profit_amount", "base_amount", "pricing_snapshot_json"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("preview response leaked forbidden key %q: %s", forbidden, string(body))
		}
	}
}

func TestPreviewOrderResellerRejectsCouponCode(t *testing.T) {
	f := newOrderResellerSnapshotFixture(t)
	input := f.createInput(f.buyer.ID)
	input.CouponCode = "MAINCODE"
	_, err := f.svc.PreviewOrder(input)
	if !errors.Is(err, ErrResellerCouponNotAllowed) {
		t.Fatalf("expected ErrResellerCouponNotAllowed, got %v", err)
	}
}

func TestCreateOrderResellerWritesSnapshotAndTenantFields(t *testing.T) {
	f := newOrderResellerSnapshotFixture(t)
	order, err := f.svc.CreateOrder(f.createInput(f.buyer.ID))
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}
	if f.queue.enqueued != 1 {
		t.Fatalf("expected timeout cancel task enqueued once, got %d", f.queue.enqueued)
	}
	if order.ResellerID == nil || *order.ResellerID != f.profile.ID {
		t.Fatalf("parent reseller id mismatch: %+v", order.ResellerID)
	}
	if order.ResellerDomain != "primary.example.test" {
		t.Fatalf("parent reseller domain mismatch: %q", order.ResellerDomain)
	}
	if !order.TotalAmount.Decimal.Equal(decimal.NewFromInt(130)) || !order.ResellerProfitAmount.Decimal.Equal(decimal.NewFromInt(30)) {
		t.Fatalf("parent amounts mismatch total=%s profit=%s", order.TotalAmount.String(), order.ResellerProfitAmount.String())
	}
	if order.AffiliateProfileID != nil || order.AffiliateCode != "" {
		t.Fatalf("reseller order must not carry affiliate snapshot: profile=%v code=%q", order.AffiliateProfileID, order.AffiliateCode)
	}
	if len(order.Children) != 1 {
		t.Fatalf("expected one child order, got %d", len(order.Children))
	}
	child := order.Children[0]
	if child.ResellerID == nil || *child.ResellerID != f.profile.ID || child.ResellerDomain != "primary.example.test" {
		t.Fatalf("child reseller fields mismatch: %+v domain=%q", child.ResellerID, child.ResellerDomain)
	}
	if !child.TotalAmount.Decimal.Equal(decimal.NewFromInt(130)) || !child.ResellerProfitAmount.Decimal.Equal(decimal.NewFromInt(30)) {
		t.Fatalf("child amounts mismatch total=%s profit=%s", child.TotalAmount.String(), child.ResellerProfitAmount.String())
	}
	if len(child.Items) != 1 {
		t.Fatalf("expected child item, got %d", len(child.Items))
	}
	if !child.Items[0].UnitPrice.Decimal.Equal(decimal.NewFromInt(130)) || !child.Items[0].OriginalUnitPrice.Decimal.Equal(decimal.NewFromInt(130)) {
		t.Fatalf("child item should expose reseller-facing prices: %+v", child.Items[0])
	}

	snapshot, err := f.resellerRepo.GetOrderSnapshotByOrderID(order.ID)
	if err != nil {
		t.Fatalf("GetOrderSnapshotByOrderID failed: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected reseller order snapshot")
	}
	if snapshot.Currency != order.Currency || snapshot.ResellerUserID != f.owner.ID || snapshot.BuyerUserID != f.buyer.ID {
		t.Fatalf("snapshot identity mismatch: %+v", snapshot)
	}
	if !snapshot.BaseAmount.Decimal.Equal(decimal.NewFromInt(100)) || !snapshot.ResellerAmount.Decimal.Equal(decimal.NewFromInt(130)) || !snapshot.ProfitAmount.Decimal.Equal(decimal.NewFromInt(30)) {
		t.Fatalf("snapshot amounts mismatch base=%s reseller=%s profit=%s", snapshot.BaseAmount.String(), snapshot.ResellerAmount.String(), snapshot.ProfitAmount.String())
	}
	if !snapshot.ProfitEligible || snapshot.ProfitBlockReason != "" {
		t.Fatalf("expected eligible snapshot, got eligible=%t reason=%q", snapshot.ProfitEligible, snapshot.ProfitBlockReason)
	}
	items, _ := snapshot.PricingSnapshotJSON["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected one pricing snapshot item, got %#v", snapshot.PricingSnapshotJSON["items"])
	}
	item, _ := items[0].(map[string]interface{})
	if item["child_order_id"] == nil || item["order_item_id"] == nil {
		t.Fatalf("snapshot item should include generated child/item ids: %+v", item)
	}
}

func TestCreateOrderResellerRuntimePricesMatchPreviewAndSnapshotAcrossRuleSources(t *testing.T) {
	f := newOrderResellerSnapshotFixture(t)
	productWithProductRule, skuWithProductRule := f.addResellerSnapshotProduct(t, "reseller-order-product-rule", decimal.NewFromInt(80), decimal.NewFromInt(40), decimal.NewFromInt(25))
	productWithProfileRule, skuWithProfileRule := f.addResellerSnapshotProduct(t, "reseller-order-profile-rule", decimal.NewFromInt(50), decimal.NewFromInt(25), decimal.NewFromInt(-1))

	input := f.createInput(f.buyer.ID)
	input.Items = []CreateOrderItem{
		{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 1},
		{ProductID: productWithProductRule.ID, SKUID: skuWithProductRule.ID, Quantity: 2},
		{ProductID: productWithProfileRule.ID, SKUID: skuWithProfileRule.ID, Quantity: 1},
	}

	preview, err := f.svc.PreviewOrder(input)
	if err != nil {
		t.Fatalf("PreviewOrder failed: %v", err)
	}
	if preview.TotalAmount.String() != "400.00" || preview.OriginalAmount.String() != "400.00" {
		t.Fatalf("preview totals mismatch original=%s total=%s", preview.OriginalAmount.String(), preview.TotalAmount.String())
	}
	if preview.PromotionDiscountAmount.String() != "0.00" || preview.WholesaleDiscountAmount.String() != "0.00" || preview.DiscountAmount.String() != "0.00" {
		t.Fatalf("reseller preview must not apply main discounts: %+v", preview)
	}

	order, err := f.svc.CreateOrder(input)
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}
	if order.TotalAmount.String() != preview.TotalAmount.String() {
		t.Fatalf("order total should match preview total, order=%s preview=%s", order.TotalAmount.String(), preview.TotalAmount.String())
	}
	if order.ResellerProfitAmount.String() != "90.00" {
		t.Fatalf("parent reseller profit mismatch: %s", order.ResellerProfitAmount.String())
	}
	if len(order.Children) != 3 {
		t.Fatalf("expected 3 child orders, got %d", len(order.Children))
	}

	snapshot, err := f.resellerRepo.GetOrderSnapshotByOrderID(order.ID)
	if err != nil {
		t.Fatalf("GetOrderSnapshotByOrderID failed: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected reseller order snapshot")
	}
	if snapshot.BaseAmount.String() != "310.00" || snapshot.ResellerAmount.String() != "400.00" || snapshot.ProfitAmount.String() != "90.00" {
		t.Fatalf("snapshot totals mismatch base=%s reseller=%s profit=%s", snapshot.BaseAmount.String(), snapshot.ResellerAmount.String(), snapshot.ProfitAmount.String())
	}
	items, ok := snapshot.PricingSnapshotJSON["items"].([]interface{})
	if !ok || len(items) != 3 {
		t.Fatalf("expected 3 pricing snapshot items, got %#v", snapshot.PricingSnapshotJSON["items"])
	}
	seenSources := map[string]bool{}
	for _, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			if asJSON, ok := raw.(jsonmap.JSON); ok {
				item = map[string]interface{}(asJSON)
			} else {
				t.Fatalf("unexpected pricing snapshot item type: %#v", raw)
			}
		}
		source, _ := item["rule_source"].(string)
		seenSources[source] = true
		if fmt.Sprint(item["child_order_id"]) == "0" || fmt.Sprint(item["order_item_id"]) == "0" {
			t.Fatalf("pricing snapshot item should contain child/order item ids: %+v", item)
		}
	}
	for _, source := range []string{
		resellermodule.RuleSourceSKU,
		resellermodule.RuleSourceProduct,
		resellermodule.RuleSourceProfile,
	} {
		if !seenSources[source] {
			t.Fatalf("missing pricing snapshot source %q in %+v", source, seenSources)
		}
	}
}

func TestPreviewAndCreateOrderResellerRejectServicePersistedHiddenProductWithoutSnapshot(t *testing.T) {
	f := newOrderResellerSnapshotFixture(t)
	settingSvc := resellermodule.NewProductSettingService(
		f.resellerRepo,
		productgormstore.NewProductStore(f.db),
	)
	if _, err := settingSvc.SaveUserProductSettings(f.owner.ID, f.product.ID, resellermodule.ProductSettingSaveInput{
		Settings: []resellermodule.ProductSettingInput{
			{SKUID: 0, IsListed: false, PricingMode: resellerdomain.PricingModeInherit},
		},
	}); err != nil {
		t.Fatalf("save hidden product through service failed: %v", err)
	}

	var hiddenRow resellerdomain.ProductSetting
	if err := f.db.Where("reseller_id = ? AND product_id = ? AND sku_id = ?", f.profile.ID, f.product.ID, uint(0)).
		First(&hiddenRow).Error; err != nil {
		t.Fatalf("fetch hidden product row failed: %v", err)
	}
	if hiddenRow.IsListed {
		t.Fatalf("hidden product service save should persist is_listed=false, got true: %+v", hiddenRow)
	}

	input := f.createInput(f.buyer.ID)
	if _, err := f.svc.PreviewOrder(input); !errors.Is(err, ErrResellerProductNotListed) {
		t.Fatalf("PreviewOrder expected ErrResellerProductNotListed, got %v", err)
	}
	if _, err := f.svc.CreateOrder(input); !errors.Is(err, ErrResellerProductNotListed) && !errors.Is(err, ErrOrderCreateFailed) {
		t.Fatalf("CreateOrder expected reseller not listed/order create failed wrapper, got %v", err)
	}
	var orderCount int64
	if err := f.db.Model(&orderdomain.Order{}).Count(&orderCount).Error; err != nil {
		t.Fatalf("count orders failed: %v", err)
	}
	var snapshotCount int64
	if err := f.db.Model(&resellerdomain.OrderSnapshot{}).Count(&snapshotCount).Error; err != nil {
		t.Fatalf("count snapshots failed: %v", err)
	}
	if orderCount != 0 || snapshotCount != 0 {
		t.Fatalf("hidden reseller order should not create rows, orders=%d snapshots=%d", orderCount, snapshotCount)
	}
}

func TestCreateOrderResellerOwnerSelfDealingWritesZeroEffectiveProfit(t *testing.T) {
	f := newOrderResellerSnapshotFixture(t)
	order, err := f.svc.CreateOrder(f.createInput(f.owner.ID))
	if err != nil {
		t.Fatalf("CreateOrder owner failed: %v", err)
	}
	if !order.ResellerProfitAmount.Decimal.Equal(decimal.Zero) {
		t.Fatalf("self-dealing shortcut profit must be zero, got %s", order.ResellerProfitAmount.String())
	}
	snapshot, err := f.resellerRepo.GetOrderSnapshotByOrderID(order.ID)
	if err != nil {
		t.Fatalf("GetOrderSnapshotByOrderID failed: %v", err)
	}
	if snapshot == nil || snapshot.ProfitEligible || snapshot.ProfitBlockReason != "self_dealing_owner" {
		t.Fatalf("expected owner self-dealing snapshot, got %+v", snapshot)
	}
	if !snapshot.ProfitAmount.Decimal.Equal(decimal.NewFromInt(30)) {
		t.Fatalf("snapshot should keep calculated blocked profit, got %s", snapshot.ProfitAmount.String())
	}
}

func TestCreateGuestOrderResellerSnapshotBuyerUserIDZero(t *testing.T) {
	f := newOrderResellerSnapshotFixture(t)
	order, err := f.svc.CreateGuestOrder(CreateGuestOrderInput{
		Email:         "guest-reseller@example.com",
		OrderPassword: "guest-pass-strong",
		Tenant:        f.tenant,
		Items:         []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("CreateGuestOrder failed: %v", err)
	}
	snapshot, err := f.resellerRepo.GetOrderSnapshotByOrderID(order.ID)
	if err != nil {
		t.Fatalf("GetOrderSnapshotByOrderID failed: %v", err)
	}
	if snapshot == nil || snapshot.BuyerUserID != 0 || !snapshot.ProfitEligible {
		t.Fatalf("guest snapshot mismatch: %+v", snapshot)
	}
	if got := snapshot.RiskSnapshotJSON["buyer_user_id"]; fmt.Sprint(got) != "0" {
		t.Fatalf("risk snapshot should record buyer_user_id=0, got %#v", got)
	}
}

func TestOrderServiceTenantScopedUserQueries(t *testing.T) {
	f := newOrderResellerSnapshotFixture(t)
	now := time.Now()
	mainOrder := orderdomain.Order{
		OrderNo:          "MAIN-SCOPED-USER",
		UserID:           f.buyer.ID,
		Status:           constants.OrderStatusPendingPayment,
		Currency:         constants.SiteCurrencyDefault,
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(100)),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	resellerID := f.profile.ID
	resellerOrder := orderdomain.Order{
		OrderNo:          "RESELLER-SCOPED-USER",
		UserID:           f.buyer.ID,
		Status:           constants.OrderStatusPendingPayment,
		Currency:         constants.SiteCurrencyDefault,
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(130)),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(130)),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(130)),
		ResellerID:       &resellerID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := f.db.Create(&mainOrder).Error; err != nil {
		t.Fatalf("create main order failed: %v", err)
	}
	if err := f.db.Create(&resellerOrder).Error; err != nil {
		t.Fatalf("create reseller order failed: %v", err)
	}

	if _, err := f.svc.GetOrderByUserOrderNoForTenant(resellercontract.MainTenantContext("main.example.test"), resellerOrder.OrderNo, f.buyer.ID); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("main scope should not read reseller order, got %v", err)
	}
	got, err := f.svc.GetOrderByUserOrderNoForTenant(f.tenant, resellerOrder.OrderNo, f.buyer.ID)
	if err != nil {
		t.Fatalf("reseller scope should read reseller order: %v", err)
	}
	if got.ID != resellerOrder.ID {
		t.Fatalf("reseller scoped order mismatch want %d got %d", resellerOrder.ID, got.ID)
	}
	orders, total, err := f.svc.ListOrdersByUserForTenant(f.tenant, ordercontract.ListFilter{UserID: f.buyer.ID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListOrdersByUserForTenant failed: %v", err)
	}
	if total != 1 || len(orders) != 1 || orders[0].ID != resellerOrder.ID {
		t.Fatalf("expected only reseller order, total=%d orders=%+v", total, orders)
	}
	stats, err := f.svc.StatsOrdersByUserForTenant(resellercontract.MainTenantContext("main.example.test"), ordercontract.ListFilter{UserID: f.buyer.ID})
	if err != nil {
		t.Fatalf("StatsOrdersByUserForTenant failed: %v", err)
	}
	if stats[constants.OrderStatusPendingPayment] != 1 {
		t.Fatalf("main stats should count only main order, got %+v", stats)
	}
}

func TestOrderServiceTenantScopedGuestQueries(t *testing.T) {
	f := newOrderResellerSnapshotFixture(t)
	now := time.Now()
	resellerID := f.profile.ID
	mainOrder := orderdomain.Order{
		OrderNo:          "MAIN-SCOPED-GUEST",
		GuestEmail:       "scoped-guest@example.com",
		GuestPassword:    "pw",
		Status:           constants.OrderStatusPendingPayment,
		Currency:         constants.SiteCurrencyDefault,
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(100)),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	resellerOrder := orderdomain.Order{
		OrderNo:          "RESELLER-SCOPED-GUEST",
		GuestEmail:       "scoped-guest@example.com",
		GuestPassword:    "pw",
		Status:           constants.OrderStatusPendingPayment,
		Currency:         constants.SiteCurrencyDefault,
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(130)),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(130)),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(130)),
		ResellerID:       &resellerID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	orderStore := ordergormstore.New(f.db, "test-guest-credential-secret-with-32-bytes")
	if err := orderStore.Create(&mainOrder, nil); err != nil {
		t.Fatalf("create main guest order failed: %v", err)
	}
	if err := orderStore.Create(&resellerOrder, nil); err != nil {
		t.Fatalf("create reseller guest order failed: %v", err)
	}

	if _, err := f.svc.GetOrderByGuestOrderNoForTenant(resellercontract.MainTenantContext("main.example.test"), resellerOrder.OrderNo, "scoped-guest@example.com", "pw"); !errors.Is(err, ErrGuestOrderNotFound) {
		t.Fatalf("main scope should not read reseller guest order, got %v", err)
	}
	got, err := f.svc.GetOrderByGuestOrderNoForTenant(f.tenant, resellerOrder.OrderNo, "scoped-guest@example.com", "pw")
	if err != nil {
		t.Fatalf("reseller scope should read reseller guest order: %v", err)
	}
	if got.ID != resellerOrder.ID {
		t.Fatalf("reseller scoped guest order mismatch want %d got %d", resellerOrder.ID, got.ID)
	}
	orders, total, err := f.svc.ListOrdersByGuestForTenant(f.tenant, "scoped-guest@example.com", "pw", 1, 20)
	if err != nil {
		t.Fatalf("ListOrdersByGuestForTenant failed: %v", err)
	}
	if total != 1 || len(orders) != 1 || orders[0].ID != resellerOrder.ID {
		t.Fatalf("expected only reseller guest order, total=%d orders=%+v", total, orders)
	}
}
