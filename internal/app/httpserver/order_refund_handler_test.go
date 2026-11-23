package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	affiliateapp "github.com/dujiao-next/internal/modules/affiliate/application"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	affiliategormstore "github.com/dujiao-next/internal/modules/affiliate/infrastructure/gormstore"
	orderrefund "github.com/dujiao-next/internal/modules/order/application/refund"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"

	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/app/container"
	orderwiring "github.com/dujiao-next/internal/bootstrap/order"
	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	ordertransport "github.com/dujiao-next/internal/modules/order/transport/http"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type adminOrderRefundFixture struct {
	MemberUserID   uint
	MemberOrderID  uint
	GuestOrderID   uint
	ManualRefundID uint
	WalletRefundID uint
}

func setupAdminOrderRefundHandlerTest(t *testing.T) (*ordertransport.AdminRefundHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:admin_order_refund_handler_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&externalidentitydomain.Identity{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&orderdomain.OrderRefundRecord{},
		&paymentdomain.Payment{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Commission{},
		&affiliatedomain.WithdrawRequest{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	orderRepo := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	userRepo := userstore.New(db)
	affiliateSvc := affiliateapp.NewService(affiliategormstore.New(db), nil, nil, nil, nil, nil)
	paymentRepo := paymentgormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	orderRefundService := orderrefund.New(orderRepo, userRepo, affiliateSvc, nil, nil, paymentRepo)

	return orderwiring.NewAdminRefundHandler(&container.Container{
		OrderStore:         orderRepo,
		PaymentStore:       paymentRepo,
		OrderRefundService: orderRefundService,
	}), db
}

func seedAdminOrderRefundData(t *testing.T, db *gorm.DB) adminOrderRefundFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)

	member := &userdomain.User{
		Email:        "refund-member@example.com",
		DisplayName:  "Refund Member",
		PasswordHash: "hash",
		Status:       constants.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("create member user failed: %v", err)
	}
	if err := db.Create(&externalidentitydomain.Identity{
		UserID:         member.ID,
		Provider:       "telegram",
		ProviderUserID: "700001",
		Username:       "refund_member_tg",
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("create oauth identity failed: %v", err)
	}

	memberOrder := &orderdomain.Order{
		OrderNo:          "DJ-ADMIN-REFUND-ORDER-1",
		UserID:           member.ID,
		Status:           constants.OrderStatusPartiallyRefunded,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount:   money.FromDecimal(decimal.Zero),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(100)),
		RefundedAmount:   money.FromDecimal(decimal.NewFromInt(10)),
		CreatedAt:        now.Add(-2 * time.Hour),
		UpdatedAt:        now.Add(-2 * time.Hour),
	}
	guestOrder := &orderdomain.Order{
		OrderNo:          "DJ-ADMIN-REFUND-ORDER-2",
		UserID:           0,
		GuestEmail:       "refund-guest@example.com",
		GuestLocale:      constants.LocaleZhCN,
		Status:           constants.OrderStatusPartiallyRefunded,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(80)),
		DiscountAmount:   money.FromDecimal(decimal.Zero),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(80)),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(80)),
		RefundedAmount:   money.FromDecimal(decimal.NewFromInt(20)),
		CreatedAt:        now.Add(-90 * time.Minute),
		UpdatedAt:        now.Add(-90 * time.Minute),
	}
	for _, order := range []*orderdomain.Order{memberOrder, guestOrder} {
		if err := db.Create(order).Error; err != nil {
			t.Fatalf("create order failed: %v", err)
		}
	}
	if err := db.Create(&paymentdomain.Payment{
		OrderID: memberOrder.ID, ProviderType: constants.PaymentProviderOfficial,
		ChannelType: constants.PaymentChannelTypeAlipay, InteractionMode: constants.PaymentInteractionRedirect,
		Amount: money.FromDecimal(decimal.NewFromInt(100)), FeeAmount: money.FromDecimal(decimal.RequireFromString("3.00")), FeePolicy: constants.PaymentFeePolicyMerchantAbsorbed,
		Currency: "CNY", Status: constants.PaymentStatusSuccess, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	items := []orderdomain.OrderItem{
		{
			OrderID:         memberOrder.ID,
			ProductID:       1,
			TitleJSON:       jsonmap.JSON{"zh-CN": "会员退款测试商品"},
			UnitPrice:       money.FromDecimal(decimal.NewFromInt(100)),
			CostPrice:       money.FromDecimal(decimal.NewFromInt(50)),
			Quantity:        1,
			TotalPrice:      money.FromDecimal(decimal.NewFromInt(100)),
			CouponDiscount:  money.FromDecimal(decimal.Zero),
			FulfillmentType: constants.FulfillmentTypeManual,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			OrderID:         guestOrder.ID,
			ProductID:       2,
			TitleJSON:       jsonmap.JSON{"zh-CN": "游客退款测试商品"},
			UnitPrice:       money.FromDecimal(decimal.NewFromInt(80)),
			CostPrice:       money.FromDecimal(decimal.NewFromInt(40)),
			Quantity:        1,
			TotalPrice:      money.FromDecimal(decimal.NewFromInt(80)),
			CouponDiscount:  money.FromDecimal(decimal.Zero),
			FulfillmentType: constants.FulfillmentTypeManual,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
	for idx := range items {
		if err := db.Create(&items[idx]).Error; err != nil {
			t.Fatalf("create order item failed: %v", err)
		}
	}

	manualRefund := &orderdomain.OrderRefundRecord{
		UserID:     member.ID,
		OrderID:    memberOrder.ID,
		Type:       constants.OrderRefundTypeManual,
		Amount:     money.FromDecimal(decimal.NewFromInt(10)),
		Currency:   "CNY",
		Remark:     "manual refund reason",
		CreatedAt:  now.Add(-40 * time.Minute),
		UpdatedAt:  now.Add(-40 * time.Minute),
		GuestEmail: "",
	}
	walletRefund := &orderdomain.OrderRefundRecord{
		UserID:     0,
		GuestEmail: "refund-guest@example.com",
		OrderID:    guestOrder.ID,
		Type:       constants.OrderRefundTypeWallet,
		Amount:     money.FromDecimal(decimal.NewFromInt(20)),
		Currency:   "CNY",
		Remark:     "wallet refund reason",
		CreatedAt:  now.Add(-20 * time.Minute),
		UpdatedAt:  now.Add(-20 * time.Minute),
	}
	for _, record := range []*orderdomain.OrderRefundRecord{manualRefund, walletRefund} {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("create refund record failed: %v", err)
		}
	}

	return adminOrderRefundFixture{
		MemberUserID:   member.ID,
		MemberOrderID:  memberOrder.ID,
		GuestOrderID:   guestOrder.ID,
		ManualRefundID: manualRefund.ID,
		WalletRefundID: walletRefund.ID,
	}
}

func TestGetAdminOrderRefundsFiltersByUserKeywordAndProduct(t *testing.T) {
	h, db := setupAdminOrderRefundHandlerTest(t)
	fixture := seedAdminOrderRefundData(t, db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/admin/order-refunds?page=1&page_size=20&user_keyword=refund_member_tg&product_keyword=会员退款测试商品",
		nil,
	)

	h.GetAdminOrderRefunds(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}

	var resp struct {
		StatusCode int `json:"status_code"`
		Pagination struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.StatusCode != 0 {
		t.Fatalf("status_code want 0 got %d", resp.StatusCode)
	}
	if resp.Pagination.Total != 1 || len(resp.Data) != 1 {
		t.Fatalf("unexpected list result: total=%d len=%d", resp.Pagination.Total, len(resp.Data))
	}

	row := resp.Data[0]
	if uint(row["id"].(float64)) != fixture.ManualRefundID {
		t.Fatalf("unexpected refund id: %+v", row["id"])
	}
	if row["refund_type_label"] != "manual" {
		t.Fatalf("unexpected refund_type_label: %+v", row["refund_type_label"])
	}
	if row["order_no"] != "DJ-ADMIN-REFUND-ORDER-1" {
		t.Fatalf("unexpected order_no: %+v", row["order_no"])
	}
	if row["user_email"] != "refund-member@example.com" {
		t.Fatalf("unexpected user_email: %+v", row["user_email"])
	}
	items, ok := row["items"].([]interface{})
	if !ok || len(items) == 0 {
		t.Fatalf("items should not be empty: %+v", row["items"])
	}
}

func TestGetAdminOrderRefundsGuestRowUsesGuestEmail(t *testing.T) {
	h, db := setupAdminOrderRefundHandlerTest(t)
	fixture := seedAdminOrderRefundData(t, db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/admin/order-refunds?page=1&page_size=20&guest_email=refund-guest@example.com",
		nil,
	)

	h.GetAdminOrderRefunds(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}

	var resp struct {
		StatusCode int                      `json:"status_code"`
		Data       []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.StatusCode != 0 {
		t.Fatalf("status_code want 0 got %d", resp.StatusCode)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("unexpected data size: %d", len(resp.Data))
	}

	row := resp.Data[0]
	if uint(row["id"].(float64)) != fixture.WalletRefundID {
		t.Fatalf("unexpected refund id: %+v", row["id"])
	}
	if row["guest_email"] != "refund-guest@example.com" {
		t.Fatalf("unexpected guest_email: %+v", row["guest_email"])
	}
	if row["guest_locale"] != constants.LocaleZhCN {
		t.Fatalf("unexpected guest_locale: %+v", row["guest_locale"])
	}
	if row["user_email"] != nil {
		t.Fatalf("guest refund should not include user_email: %+v", row["user_email"])
	}
	if row["user_display_name"] != nil {
		t.Fatalf("guest refund should not include user_display_name: %+v", row["user_display_name"])
	}
}

func TestGetAdminOrderRefundDetailIncludesOrderIDAndGuestEmail(t *testing.T) {
	h, db := setupAdminOrderRefundHandlerTest(t)
	fixture := seedAdminOrderRefundData(t, db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", fixture.WalletRefundID)}}
	c.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/order-refunds/%d", fixture.WalletRefundID), nil)

	h.GetAdminOrderRefund(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}

	var resp struct {
		StatusCode int                    `json:"status_code"`
		Data       map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.StatusCode != 0 {
		t.Fatalf("status_code want 0 got %d", resp.StatusCode)
	}
	if uint(resp.Data["id"].(float64)) != fixture.WalletRefundID {
		t.Fatalf("unexpected refund id: %+v", resp.Data["id"])
	}
	if resp.Data["refund_type_label"] != "wallet" {
		t.Fatalf("unexpected refund_type_label: %+v", resp.Data["refund_type_label"])
	}
	if uint(resp.Data["order_id"].(float64)) != fixture.GuestOrderID {
		t.Fatalf("unexpected order_id: %+v", resp.Data["order_id"])
	}
	if resp.Data["guest_email"] != "refund-guest@example.com" {
		t.Fatalf("unexpected guest_email: %+v", resp.Data["guest_email"])
	}
	if resp.Data["guest_locale"] != constants.LocaleZhCN {
		t.Fatalf("unexpected guest_locale: %+v", resp.Data["guest_locale"])
	}
	if resp.Data["user_email"] != nil {
		t.Fatalf("guest refund should not include user_email: %+v", resp.Data["user_email"])
	}
	if resp.Data["user_display_name"] != nil {
		t.Fatalf("guest refund should not include user_display_name: %+v", resp.Data["user_display_name"])
	}
	if resp.Data["remark"] != "wallet refund reason" {
		t.Fatalf("unexpected remark: %+v", resp.Data["remark"])
	}
	detailItems, ok := resp.Data["items"].([]interface{})
	if !ok || len(detailItems) == 0 {
		t.Fatalf("items should not be empty: %+v", resp.Data["items"])
	}
}

func TestUpdateAdminOrderRefundPaymentFeeSupportsHistoricalManualRefund(t *testing.T) {
	h, db := setupAdminOrderRefundHandlerTest(t)
	fixture := seedAdminOrderRefundData(t, db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", fixture.ManualRefundID)}}
	c.Request = httptest.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("/admin/order-refunds/%d/payment-fee", fixture.ManualRefundID),
		bytes.NewBufferString(`{"payment_fee_refunded":true}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateAdminOrderRefundPaymentFee(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d: %s", w.Code, w.Body.String())
	}
	var record orderdomain.OrderRefundRecord
	if err := db.First(&record, fixture.ManualRefundID).Error; err != nil {
		t.Fatalf("reload refund record failed: %v", err)
	}
	if !record.PaymentFeeRefunded || !record.PaymentFeeRefundedAmount.Decimal.Equal(decimal.RequireFromString("0.30")) {
		t.Fatalf("unexpected updated payment fee refund: %+v", record)
	}

	var resp struct {
		StatusCode int                    `json:"status_code"`
		Data       map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.StatusCode != 0 {
		t.Fatalf("status_code want 0 got %d", resp.StatusCode)
	}
	if resp.Data["order_no"] != "DJ-ADMIN-REFUND-ORDER-1" {
		t.Fatalf("unexpected order_no: %+v", resp.Data["order_no"])
	}
	if resp.Data["refund_type_label"] != "manual" {
		t.Fatalf("unexpected refund_type_label: %+v", resp.Data["refund_type_label"])
	}
	if resp.Data["user_email"] != "refund-member@example.com" {
		t.Fatalf("unexpected user_email: %+v", resp.Data["user_email"])
	}
	items, ok := resp.Data["items"].([]interface{})
	if !ok || len(items) == 0 {
		t.Fatalf("items should not be empty: %+v", resp.Data["items"])
	}
}
