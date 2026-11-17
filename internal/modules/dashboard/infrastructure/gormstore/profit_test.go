package gormstore

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	dashboard "github.com/dujiao-next/internal/modules/dashboard/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
)

func TestGetProfitOverviewDeductsRefundRecords(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	now := time.Now().UTC().Truncate(time.Second)

	category := createDashboardCategory(t, db, "dashboard-profit-refund-category")
	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "dashboard-profit-refund-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "利润测试商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	manualRefundedOrder := createDashboardProfitOrderWithItem(t, db, product, "DJ-PROFIT-MANUAL", constants.OrderStatusRefunded, 100, 40, "利润测试商品", now)
	walletRefundedOrder := createDashboardProfitOrderWithItem(t, db, product, "DJ-PROFIT-WALLET", constants.OrderStatusPartiallyRefunded, 120, 50, "利润测试商品", now)

	records := []orderdomain.OrderRefundRecord{
		{
			UserID:    1,
			OrderID:   manualRefundedOrder.ID,
			Type:      constants.OrderRefundTypeManual,
			Amount:    money.FromDecimal(decimal.NewFromInt(100)),
			Currency:  "CNY",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			UserID:    1,
			OrderID:   walletRefundedOrder.ID,
			Type:      constants.OrderRefundTypeWallet,
			Amount:    money.FromDecimal(decimal.NewFromInt(20)),
			Currency:  "CNY",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			UserID:    1,
			OrderID:   walletRefundedOrder.ID,
			Type:      constants.OrderRefundTypeManual,
			Amount:    money.FromDecimal(decimal.NewFromInt(10)),
			Currency:  "CNY",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	for idx := range records {
		if err := db.Create(&records[idx]).Error; err != nil {
			t.Fatalf("create refund record failed: %v", err)
		}
	}

	result, err := repo.GetProfitOverview(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("get profit overview failed: %v", err)
	}
	if math.Abs(result.TotalRevenue-90) > 0.000001 {
		t.Fatalf("total revenue want 90 got %.2f", result.TotalRevenue)
	}
	if math.Abs(result.TotalCost-90) > 0.000001 {
		t.Fatalf("total cost want 90 got %.2f", result.TotalCost)
	}
	if math.Abs(result.RefundedCost-52.5) > 0.000001 {
		t.Fatalf("refunded cost want 52.5 got %.2f", result.RefundedCost)
	}
}

func TestGetProfitTrendsDeductsRefundRecords(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	category := createDashboardCategory(t, db, "dashboard-profit-trend-refund-category")
	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "dashboard-profit-trend-refund-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "利润趋势测试商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	day1Order := createDashboardProfitOrderWithItem(t, db, product, "DJ-PROFIT-TREND-DAY1", constants.OrderStatusRefunded, 80, 30, "利润趋势测试商品", base)
	day2Order := createDashboardProfitOrderWithItem(t, db, product, "DJ-PROFIT-TREND-DAY2", constants.OrderStatusRefunded, 100, 40, "利润趋势测试商品", base.Add(24*time.Hour))

	records := []orderdomain.OrderRefundRecord{
		{
			UserID:    1,
			OrderID:   day1Order.ID,
			Type:      constants.OrderRefundTypeManual,
			Amount:    money.FromDecimal(decimal.NewFromInt(80)),
			Currency:  "CNY",
			CreatedAt: base,
			UpdatedAt: base,
		},
		{
			UserID:    1,
			OrderID:   day2Order.ID,
			Type:      constants.OrderRefundTypeWallet,
			Amount:    money.FromDecimal(decimal.NewFromInt(30)),
			Currency:  "CNY",
			CreatedAt: base.Add(24 * time.Hour),
			UpdatedAt: base.Add(24 * time.Hour),
		},
		{
			UserID:    1,
			OrderID:   day2Order.ID,
			Type:      constants.OrderRefundTypeManual,
			Amount:    money.FromDecimal(decimal.NewFromInt(10)),
			Currency:  "CNY",
			CreatedAt: base.Add(24 * time.Hour),
			UpdatedAt: base.Add(24 * time.Hour),
		},
	}
	for idx := range records {
		if err := db.Create(&records[idx]).Error; err != nil {
			t.Fatalf("create refund record failed: %v", err)
		}
	}

	rows, err := repo.GetProfitTrends(base.Add(-time.Hour), base.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("get profit trends failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("profit trend rows want 2 got %d", len(rows))
	}
	rowMap := make(map[string]dashboard.ProfitTrendRow, len(rows))
	for _, row := range rows {
		rowMap[row.Day] = row
	}
	day1 := "2026-03-01"
	day2 := "2026-03-02"
	if math.Abs(rowMap[day1].Revenue-0) > 0.000001 || math.Abs(rowMap[day1].Cost-30) > 0.000001 || math.Abs(rowMap[day1].RefundedCost-30) > 0.000001 {
		t.Fatalf("unexpected day1 row: %+v", rowMap[day1])
	}
	if math.Abs(rowMap[day2].Revenue-60) > 0.000001 || math.Abs(rowMap[day2].Cost-40) > 0.000001 || math.Abs(rowMap[day2].RefundedCost-16) > 0.000001 {
		t.Fatalf("unexpected day2 row: %+v", rowMap[day2])
	}
}

func TestProfitStatisticsDeductSuccessfulExternalPaymentFees(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	base := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)

	category := createDashboardCategory(t, db, "dashboard-profit-payment-fee-category")
	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "dashboard-profit-payment-fee-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "手续费利润测试商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	order := createDashboardProfitOrderWithItem(t, db, product, "DJ-PROFIT-PAYMENT-FEE", constants.OrderStatusPaid, 100, 40, "手续费利润测试商品", base)

	payments := []paymentdomain.Payment{
		{
			OrderID: order.ID, ProviderType: constants.PaymentProviderOfficial,
			ChannelType: constants.PaymentChannelTypeAlipay, InteractionMode: constants.PaymentInteractionRedirect,
			Amount: money.FromDecimal(decimal.NewFromInt(100)), FeeAmount: money.FromDecimal(decimal.RequireFromString("3.00")), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed,
			Currency: "CNY", Status: constants.PaymentStatusSuccess, CreatedAt: base, UpdatedAt: base,
		},
		{
			OrderID: order.ID, ProviderType: constants.PaymentProviderOfficial,
			ChannelType: constants.PaymentChannelTypeWechat, InteractionMode: constants.PaymentInteractionQR,
			Amount: money.FromDecimal(decimal.NewFromInt(100)), FeeAmount: money.FromDecimal(decimal.RequireFromString("9.00")), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed,
			Currency: "CNY", Status: constants.PaymentStatusFailed, CreatedAt: base, UpdatedAt: base,
		},
		{
			OrderID: 0, ProviderType: constants.PaymentProviderOfficial,
			ChannelType: constants.PaymentChannelTypeAlipay, InteractionMode: constants.PaymentInteractionRedirect,
			Amount: money.FromDecimal(decimal.NewFromInt(50)), FeeAmount: money.FromDecimal(decimal.RequireFromString("2.00")), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed,
			Currency: "CNY", Status: constants.PaymentStatusSuccess, CreatedAt: base, UpdatedAt: base,
		},
		{
			OrderID: order.ID, ProviderType: constants.PaymentProviderWallet,
			ChannelType: constants.PaymentChannelTypeBalance, InteractionMode: constants.PaymentInteractionBalance,
			Amount: money.FromDecimal(decimal.NewFromInt(100)), FeeAmount: money.FromDecimal(decimal.RequireFromString("99.00")), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed,
			Currency: "CNY", Status: constants.PaymentStatusSuccess, CreatedAt: base, UpdatedAt: base,
		},
		{
			OrderID: order.ID, ProviderType: constants.PaymentProviderOfficial,
			ChannelType: constants.PaymentChannelTypeAlipay, InteractionMode: constants.PaymentInteractionRedirect,
			Amount: money.FromDecimal(decimal.NewFromInt(103)), FeeAmount: money.FromDecimal(decimal.RequireFromString("11.00")), FeePolicy: constants.PaymentFeePolicyLegacyCustomerSurcharge,
			Currency: "CNY", Status: constants.PaymentStatusSuccess, CreatedAt: base, UpdatedAt: base,
		},
		{
			OrderID: order.ID, ProviderType: constants.PaymentProviderOfficial,
			ChannelType: constants.PaymentChannelTypeAlipay, InteractionMode: constants.PaymentInteractionRedirect,
			Amount: money.FromDecimal(decimal.NewFromInt(100)), FeeAmount: money.FromDecimal(decimal.RequireFromString("7.00")), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed,
			Currency: "CNY", Status: constants.PaymentStatusSuccess, CreatedAt: base.Add(48 * time.Hour), UpdatedAt: base.Add(48 * time.Hour),
		},
	}
	if err := db.Create(&payments).Error; err != nil {
		t.Fatalf("create payments failed: %v", err)
	}

	startAt := base.Add(-time.Hour)
	endAt := base.Add(24 * time.Hour)
	overview, err := repo.GetProfitOverview(startAt, endAt)
	if err != nil {
		t.Fatalf("get profit overview failed: %v", err)
	}
	if math.Abs(overview.TotalRevenue-100) > 0.000001 || math.Abs(overview.TotalCost-40) > 0.000001 || math.Abs(overview.PaymentFee-5) > 0.000001 {
		t.Fatalf("unexpected profit overview: %+v", overview)
	}

	rows, err := repo.GetProfitTrends(startAt, endAt)
	if err != nil {
		t.Fatalf("get profit trends failed: %v", err)
	}
	if len(rows) != 1 || math.Abs(rows[0].PaymentFee-5) > 0.000001 {
		t.Fatalf("unexpected profit trends: %+v", rows)
	}
}

func TestProfitStatisticsReverseReturnedPaymentFeeOnRefundDay(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	base := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	refundAt := base.Add(24 * time.Hour)

	category := createDashboardCategory(t, db, "dashboard-profit-refunded-payment-fee-category")
	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "dashboard-profit-refunded-payment-fee-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "退款手续费冲回测试商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	order := createDashboardProfitOrderWithItem(t, db, product, "DJ-PROFIT-REFUNDED-PAYMENT-FEE", constants.OrderStatusRefunded, 100, 40, "退款手续费冲回测试商品", base)
	if err := db.Create(&paymentdomain.Payment{
		OrderID: order.ID, ProviderType: constants.PaymentProviderOfficial,
		ChannelType: constants.PaymentChannelTypeAlipay, InteractionMode: constants.PaymentInteractionRedirect,
		Amount: money.FromDecimal(decimal.NewFromInt(100)), FeeAmount: money.FromDecimal(decimal.RequireFromString("3.00")), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed,
		Currency: "CNY", Status: constants.PaymentStatusSuccess, CreatedAt: base, UpdatedAt: base,
	}).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}
	if err := db.Create(&orderdomain.OrderRefundRecord{
		OrderID: order.ID, Type: constants.OrderRefundTypeManual,
		Amount:                   money.FromDecimal(decimal.NewFromInt(100)),
		PaymentFeeRefunded:       true,
		PaymentFeeRefundedAmount: money.FromDecimal(decimal.RequireFromString("3.00")),
		Currency:                 "CNY",
		CreatedAt:                refundAt,
		UpdatedAt:                refundAt,
	}).Error; err != nil {
		t.Fatalf("create refund record failed: %v", err)
	}

	overview, err := repo.GetProfitOverview(base.Add(-time.Hour), refundAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("get profit overview failed: %v", err)
	}
	if math.Abs(overview.PaymentFee) > 0.000001 {
		t.Fatalf("net payment fee = %v, want 0", overview.PaymentFee)
	}

	rows, err := repo.GetProfitTrends(base.Add(-time.Hour), refundAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("get profit trends failed: %v", err)
	}
	rowMap := make(map[string]dashboard.ProfitTrendRow, len(rows))
	for _, row := range rows {
		rowMap[row.Day] = row
	}
	if math.Abs(rowMap["2026-08-11"].PaymentFee-3) > 0.000001 {
		t.Fatalf("payment-day fee = %v, want 3", rowMap["2026-08-11"].PaymentFee)
	}
	if math.Abs(rowMap["2026-08-12"].PaymentFee+3) > 0.000001 {
		t.Fatalf("refund-day fee = %v, want -3", rowMap["2026-08-12"].PaymentFee)
	}

	refundOnly, err := repo.GetProfitOverview(refundAt.Add(-time.Hour), refundAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("get refund-only overview failed: %v", err)
	}
	if math.Abs(refundOnly.PaymentFee+3) > 0.000001 {
		t.Fatalf("refund-only payment fee = %v, want -3", refundOnly.PaymentFee)
	}
}

func TestGetProfitOverviewDeductsInWindowRefundForOutOfWindowOrder(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	now := time.Now().UTC().Truncate(time.Second)
	startAt := now.AddDate(0, 0, -7)
	endAt := now.Add(time.Hour)

	category := createDashboardCategory(t, db, "dashboard-profit-period-refund-category")
	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "dashboard-profit-period-refund-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "周期退款测试商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	outsideOrder := &orderdomain.Order{
		OrderNo:        "DJ-PROFIT-OUTSIDE-ORDER",
		UserID:         1,
		Status:         constants.OrderStatusRefunded,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:      startAt.Add(-24 * time.Hour),
		UpdatedAt:      startAt.Add(-24 * time.Hour),
	}
	if err := db.Create(outsideOrder).Error; err != nil {
		t.Fatalf("create outside order failed: %v", err)
	}
	if err := db.Create(&orderdomain.OrderItem{
		OrderID:         outsideOrder.ID,
		ProductID:       product.ID,
		TitleJSON:       jsonmap.JSON{"zh-CN": "周期退款测试商品"},
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(100)),
		CostPrice:       money.FromDecimal(decimal.NewFromInt(40)),
		Quantity:        1,
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(100)),
		CouponDiscount:  money.FromDecimal(decimal.Zero),
		FulfillmentType: constants.FulfillmentTypeManual,
		CreatedAt:       startAt.Add(-24 * time.Hour),
		UpdatedAt:       startAt.Add(-24 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("create outside order item failed: %v", err)
	}

	inWindowOrder := &orderdomain.Order{
		OrderNo:        "DJ-PROFIT-IN-WINDOW",
		UserID:         1,
		Status:         constants.OrderStatusCompleted,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(60)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(60)),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(inWindowOrder).Error; err != nil {
		t.Fatalf("create in-window order failed: %v", err)
	}
	if err := db.Create(&orderdomain.OrderItem{
		OrderID:         inWindowOrder.ID,
		ProductID:       product.ID,
		TitleJSON:       jsonmap.JSON{"zh-CN": "周期退款测试商品"},
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(60)),
		CostPrice:       money.FromDecimal(decimal.NewFromInt(20)),
		Quantity:        1,
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(60)),
		CouponDiscount:  money.FromDecimal(decimal.Zero),
		FulfillmentType: constants.FulfillmentTypeManual,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("create in-window order item failed: %v", err)
	}

	if err := db.Create(&orderdomain.OrderRefundRecord{
		UserID:    1,
		OrderID:   outsideOrder.ID,
		Type:      constants.OrderRefundTypeManual,
		Amount:    money.FromDecimal(decimal.NewFromInt(50)),
		Currency:  "CNY",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create refund record failed: %v", err)
	}

	result, err := repo.GetProfitOverview(startAt, endAt)
	if err != nil {
		t.Fatalf("get profit overview failed: %v", err)
	}
	if math.Abs(result.TotalRevenue-10) > 0.000001 {
		t.Fatalf("total revenue want 10 got %.2f", result.TotalRevenue)
	}
	if math.Abs(result.TotalCost-20) > 0.000001 {
		t.Fatalf("total cost want 20 got %.2f", result.TotalCost)
	}
	if math.Abs(result.RefundedCost-20) > 0.000001 {
		t.Fatalf("refunded cost want 20 got %.2f", result.RefundedCost)
	}
}

func TestGetProfitTrendsIncludesRefundOnlyDayInWindow(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	startAt := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	endAt := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	day1 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 2, 11, 0, 0, 0, time.UTC)

	category := createDashboardCategory(t, db, "dashboard-profit-refund-only-day-category")
	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "dashboard-profit-refund-only-day-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "退款单日测试商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	inWindowOrder := &orderdomain.Order{
		OrderNo:        "DJ-PROFIT-TREND-IN-WINDOW",
		UserID:         1,
		Status:         constants.OrderStatusCompleted,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(80)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(80)),
		CreatedAt:      day1,
		UpdatedAt:      day1,
	}
	if err := db.Create(inWindowOrder).Error; err != nil {
		t.Fatalf("create in-window order failed: %v", err)
	}
	if err := db.Create(&orderdomain.OrderItem{
		OrderID:         inWindowOrder.ID,
		ProductID:       product.ID,
		TitleJSON:       jsonmap.JSON{"zh-CN": "退款单日测试商品"},
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(80)),
		CostPrice:       money.FromDecimal(decimal.NewFromInt(30)),
		Quantity:        1,
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(80)),
		CouponDiscount:  money.FromDecimal(decimal.Zero),
		FulfillmentType: constants.FulfillmentTypeManual,
		CreatedAt:       day1,
		UpdatedAt:       day1,
	}).Error; err != nil {
		t.Fatalf("create in-window order item failed: %v", err)
	}

	outsideOrder := &orderdomain.Order{
		OrderNo:        "DJ-PROFIT-TREND-OUTSIDE-ORDER",
		UserID:         1,
		Status:         constants.OrderStatusRefunded,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:      startAt.Add(-48 * time.Hour),
		UpdatedAt:      startAt.Add(-48 * time.Hour),
	}
	if err := db.Create(outsideOrder).Error; err != nil {
		t.Fatalf("create outside order failed: %v", err)
	}
	if err := db.Create(&orderdomain.OrderItem{
		OrderID:         outsideOrder.ID,
		ProductID:       product.ID,
		TitleJSON:       jsonmap.JSON{"zh-CN": "退款单日测试商品"},
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(100)),
		CostPrice:       money.FromDecimal(decimal.NewFromInt(40)),
		Quantity:        1,
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(100)),
		CouponDiscount:  money.FromDecimal(decimal.Zero),
		FulfillmentType: constants.FulfillmentTypeManual,
		CreatedAt:       startAt.Add(-48 * time.Hour),
		UpdatedAt:       startAt.Add(-48 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("create outside order item failed: %v", err)
	}

	if err := db.Create(&orderdomain.OrderRefundRecord{
		UserID:    1,
		OrderID:   outsideOrder.ID,
		Type:      constants.OrderRefundTypeManual,
		Amount:    money.FromDecimal(decimal.NewFromInt(30)),
		Currency:  "CNY",
		CreatedAt: day2,
		UpdatedAt: day2,
	}).Error; err != nil {
		t.Fatalf("create refund record failed: %v", err)
	}

	rows, err := repo.GetProfitTrends(startAt, endAt)
	if err != nil {
		t.Fatalf("get profit trends failed: %v", err)
	}
	rowMap := make(map[string]dashboard.ProfitTrendRow, len(rows))
	for _, row := range rows {
		rowMap[row.Day] = row
	}
	if math.Abs(rowMap["2026-03-01"].Revenue-80) > 0.000001 || math.Abs(rowMap["2026-03-01"].Cost-30) > 0.000001 {
		t.Fatalf("unexpected 2026-03-01 row: %+v", rowMap["2026-03-01"])
	}
	if math.Abs(rowMap["2026-03-02"].Revenue-(-30)) > 0.000001 || math.Abs(rowMap["2026-03-02"].Cost-0) > 0.000001 || math.Abs(rowMap["2026-03-02"].RefundedCost-12) > 0.000001 {
		t.Fatalf("unexpected 2026-03-02 row: %+v", rowMap["2026-03-02"])
	}
}

func TestProfitMetricsIncludeZeroCostRevenueAndCalculateRefundedCost(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)

	category := createDashboardCategory(t, db, "dashboard-zero-cost-refund-category")
	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "dashboard-zero-cost-refund-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "零成本退款商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(49)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	orders := make([]*orderdomain.Order, 0, 4)
	for index := 0; index < 3; index++ {
		orders = append(orders, createDashboardProfitOrderWithItem(
			t,
			db,
			product,
			fmt.Sprintf("DJ-PROFIT-COST-ONE-%d", index+1),
			constants.OrderStatusRefunded,
			1,
			1,
			"成本一元商品",
			now,
		))
	}
	orders = append(orders, createDashboardProfitOrderWithItem(
		t,
		db,
		product,
		"DJ-PROFIT-ZERO-COST",
		constants.OrderStatusRefunded,
		49,
		0,
		"零成本商品",
		now,
	))

	for _, order := range orders {
		if err := db.Create(&orderdomain.OrderRefundRecord{
			UserID:    1,
			OrderID:   order.ID,
			Type:      constants.OrderRefundTypeManual,
			Amount:    order.TotalAmount,
			Currency:  "CNY",
			CreatedAt: now,
			UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create refund record failed: %v", err)
		}
	}

	startAt := now.Add(-time.Hour)
	endAt := now.Add(time.Hour)
	overview, err := repo.GetProfitOverview(startAt, endAt)
	if err != nil {
		t.Fatalf("get profit overview failed: %v", err)
	}
	if math.Abs(overview.TotalRevenue) > 0.000001 || math.Abs(overview.TotalCost-3) > 0.000001 || math.Abs(overview.RefundedCost-3) > 0.000001 {
		t.Fatalf("unexpected zero-cost refund overview: %+v", overview)
	}

	trends, err := repo.GetProfitTrends(startAt, endAt)
	if err != nil {
		t.Fatalf("get profit trends failed: %v", err)
	}
	if len(trends) != 1 {
		t.Fatalf("profit trend rows want 1 got %d", len(trends))
	}
	if math.Abs(trends[0].Revenue) > 0.000001 || math.Abs(trends[0].Cost-3) > 0.000001 || math.Abs(trends[0].RefundedCost-3) > 0.000001 {
		t.Fatalf("unexpected zero-cost refund trend: %+v", trends[0])
	}
}

func TestRefundedCostUsesParentOrderChildrenCostBasis(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	now := time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)

	category := createDashboardCategory(t, db, "dashboard-parent-refund-cost-category")
	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "dashboard-parent-refund-cost-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "父订单退款成本商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	parent := &orderdomain.Order{
		OrderNo:        "DJ-PROFIT-PARENT-REFUND",
		UserID:         1,
		Status:         constants.OrderStatusPartiallyRefunded,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent order failed: %v", err)
	}
	children := []*orderdomain.Order{
		createDashboardProfitOrderWithItem(t, db, product, "DJ-PROFIT-PARENT-CHILD-1", constants.OrderStatusPartiallyRefunded, 40, 10, "子订单一", now),
		createDashboardProfitOrderWithItem(t, db, product, "DJ-PROFIT-PARENT-CHILD-2", constants.OrderStatusPartiallyRefunded, 60, 30, "子订单二", now),
	}
	for _, child := range children {
		if err := db.Model(&orderdomain.Order{}).Where("id = ?", child.ID).Update("parent_id", parent.ID).Error; err != nil {
			t.Fatalf("attach child order failed: %v", err)
		}
	}
	if err := db.Create(&orderdomain.OrderRefundRecord{
		UserID:    1,
		OrderID:   parent.ID,
		Type:      constants.OrderRefundTypeManual,
		Amount:    money.FromDecimal(decimal.NewFromInt(50)),
		Currency:  "CNY",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create parent refund record failed: %v", err)
	}

	result, err := repo.GetProfitOverview(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("get parent refund profit overview failed: %v", err)
	}
	if math.Abs(result.TotalRevenue-50) > 0.000001 || math.Abs(result.TotalCost-40) > 0.000001 || math.Abs(result.RefundedCost-20) > 0.000001 {
		t.Fatalf("unexpected parent refund metrics: %+v", result)
	}
}

func TestProfitStatisticsReverseRechargeFeeWithoutChangingRevenueOrProductCost(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	base := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	refundAt := base.Add(24 * time.Hour)
	if err := db.Create(&paymentdomain.Payment{OrderID: 0, ProviderType: constants.PaymentProviderOfficial, ChannelType: constants.PaymentChannelTypeAlipay, InteractionMode: constants.PaymentInteractionRedirect, Amount: money.FromDecimal(decimal.NewFromInt(100)), FeeAmount: money.FromDecimal(decimal.RequireFromString("3.01")), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed, Currency: "CNY", Status: constants.PaymentStatusSuccess, CreatedAt: base, UpdatedAt: base}).Error; err != nil {
		t.Fatal(err)
	}
	rechargeID := uint(42)
	deletedAt := refundAt
	records := []walletdomain.Transaction{
		{UserID: 1, RechargeOrderID: &rechargeID, Type: constants.WalletTxnTypeAdminRefund, Reference: "recharge-fee", Amount: money.FromDecimal(decimal.NewFromInt(40)), PaymentFeeRefunded: true, PaymentFeeRefundedAmount: money.FromDecimal(decimal.RequireFromString("1.20")), CreatedAt: refundAt, UpdatedAt: refundAt},
		{UserID: 1, RechargeOrderID: &rechargeID, Type: constants.WalletTxnTypeAdminRefund, Reference: "unchecked-fee", PaymentFeeRefunded: false, PaymentFeeRefundedAmount: money.FromDecimal(decimal.NewFromInt(99)), CreatedAt: refundAt, UpdatedAt: refundAt},
		{UserID: 1, Type: constants.WalletTxnTypeAdminRefund, Reference: "unrelated-refund", PaymentFeeRefunded: true, PaymentFeeRefundedAmount: money.FromDecimal(decimal.NewFromInt(98)), CreatedAt: refundAt, UpdatedAt: refundAt},
		{UserID: 1, RechargeOrderID: &rechargeID, Type: constants.WalletTxnTypeAdminRefund, Reference: "deleted-fee", PaymentFeeRefunded: true, PaymentFeeRefundedAmount: money.FromDecimal(decimal.NewFromInt(97)), CreatedAt: refundAt, UpdatedAt: refundAt, DeletedAt: &deletedAt},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	overview, err := repo.GetProfitOverview(base.Add(-time.Hour), refundAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(overview.PaymentFee-1.81) > 0.000001 || overview.TotalRevenue != 0 || overview.TotalCost != 0 || overview.RefundedCost != 0 {
		t.Fatalf("recharge changed product profit: %+v", overview)
	}
	refundOnly, err := repo.GetProfitOverview(refundAt.Add(-time.Hour), refundAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(refundOnly.PaymentFee+1.20) > 0.000001 {
		t.Fatalf("refund-day fee: %+v", refundOnly)
	}
	rows, err := repo.GetProfitTrends(base.Add(-time.Hour), refundAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Day != "2026-11-11" || rows[1].Day != "2026-11-12" || math.Abs(rows[0].PaymentFee-3.01) > 0.000001 || math.Abs(rows[1].PaymentFee+1.20) > 0.000001 {
		t.Fatalf("fee trends: %+v", rows)
	}
}
