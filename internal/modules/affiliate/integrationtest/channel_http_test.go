package integrationtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	userauthgormstore "github.com/dujiao-next/internal/modules/identity/userauth/infrastructure/gormstore"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	affiliategormstore "github.com/dujiao-next/internal/modules/affiliate/infrastructure/gormstore"

	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	affiliateapp "github.com/dujiao-next/internal/modules/affiliate/application"
	affiliatetransport "github.com/dujiao-next/internal/modules/affiliate/transport/http"
	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	emailverificationstore "github.com/dujiao-next/internal/modules/identity/emailverification/infrastructure/gormstore"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	externalidentitystore "github.com/dujiao-next/internal/modules/identity/externalidentity/infrastructure/gormstore"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/dujiao-next/internal/platform/database/gormdb"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type channelAffiliateTestResponse struct {
	StatusCode int            `json:"status_code"`
	Msg        string         `json:"msg"`
	Data       map[string]any `json:"data"`
	ErrorCode  string         `json:"error_code"`
}

type channelAffiliateUserAdapter struct {
	auth *userauthapp.Service
}

type channelAffiliateOrderReader struct {
	db *gorm.DB
}

func (r channelAffiliateOrderReader) GetByID(id uint) (*orderdomain.Order, error) {
	var order orderdomain.Order
	err := r.db.Preload("Items").Preload("Children").Preload("Children.Items").First(&order, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (a channelAffiliateUserAdapter) ProvisionUserID(identity affiliatetransport.ChannelIdentity) (uint, error) {
	user, _, _, err := a.auth.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{
		ChannelUserID: identity.ChannelUserID,
		Username:      identity.Username,
		FirstName:     identity.FirstName,
		LastName:      identity.LastName,
		AvatarURL:     identity.AvatarURL,
	})
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, userauthapp.ErrNotFound
	}
	return user.ID, nil
}

func setupChannelAffiliateHandlerTest(t *testing.T) (*gorm.DB, *httptest.Server) {
	t.Helper()

	dsn := fmt.Sprintf("file:channel_affiliate_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&externalidentitydomain.Identity{},
		&emailverificationdomain.Code{},
		&settingsstore.SettingRecord{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Click{},
		&affiliatedomain.Commission{},
		&affiliatedomain.WithdrawRequest{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	gormdb.DB = db

	userRepo := userstore.New(db)
	identityRepo := externalidentitystore.New(db)
	emailVerifyRepo := emailverificationstore.New(db)
	settingRepo := settingsstore.New(db)
	affiliateRepo := affiliategormstore.New(db)

	settingSvc := settingsapp.NewService(settingRepo)
	if _, err := settingSvc.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{
		Enabled:           true,
		CommissionRate:    10,
		ConfirmDays:       7,
		MinWithdrawAmount: 10,
		WithdrawChannels:  []string{"alipay", "wechat", "bank"},
	}); err != nil {
		t.Fatalf("enable affiliate setting failed: %v", err)
	}

	affiliateSvc := affiliateapp.NewService(affiliateRepo, userRepo, channelAffiliateOrderReader{db: db}, nil, settingSvc)
	userAuthSvc := userauthapp.NewService(&config.Config{}, userRepo, identityRepo, emailVerifyRepo, nil, nil, nil)
	userAuthSvc.SetAuthUnitOfWork(userauthgormstore.New(db))
	handler := affiliatetransport.NewChannelHandler(
		affiliateSvc,
		channelAffiliateUserAdapter{auth: userAuthSvc},
		settingSvc,
	)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	affiliatetransport.RegisterChannelRoutes(router.Group("/api/v1/channel"), handler)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return db, server
}

func decodeChannelAffiliateResponse(t *testing.T, resp *http.Response) channelAffiliateTestResponse {
	t.Helper()

	var payload channelAffiliateTestResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	return payload
}

func TestChannelAffiliateOpenAndDashboard(t *testing.T) {
	db, server := setupChannelAffiliateHandlerTest(t)

	raw, err := json.Marshal(map[string]any{
		"telegram_user_id": "998877",
		"username":         "channel_affiliate_user",
	})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	resp, err := http.Post(server.URL+"/api/v1/channel/affiliate/open", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post affiliate open failed: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected http status 200, got %d", resp.StatusCode)
	}

	payload := decodeChannelAffiliateResponse(t, resp)
	if payload.StatusCode != 0 {
		t.Fatalf("expected status_code=0, got %d", payload.StatusCode)
	}
	if payload.Data["code"] == "" {
		t.Fatalf("expected affiliate code in open response, got=%v", payload.Data["code"])
	}

	var identity externalidentitydomain.Identity
	if err := db.Where("provider = ? AND provider_user_id = ?", constants.UserOAuthProviderTelegram, "998877").First(&identity).Error; err != nil {
		t.Fatalf("expected telegram identity to be provisioned: %v", err)
	}

	dashboardResp, err := http.Get(server.URL + "/api/v1/channel/affiliate/dashboard?channel_user_id=998877")
	if err != nil {
		t.Fatalf("get affiliate dashboard failed: %v", err)
	}
	t.Cleanup(func() { _ = dashboardResp.Body.Close() })
	if dashboardResp.StatusCode != http.StatusOK {
		t.Fatalf("expected dashboard http status 200, got %d", dashboardResp.StatusCode)
	}

	dashboardPayload := decodeChannelAffiliateResponse(t, dashboardResp)
	if opened, ok := dashboardPayload.Data["opened"].(bool); !ok || !opened {
		t.Fatalf("expected opened=true, got=%v", dashboardPayload.Data["opened"])
	}
	if dashboardPayload.Data["affiliate_code"] == "" {
		t.Fatalf("expected affiliate_code in dashboard response, got=%v", dashboardPayload.Data["affiliate_code"])
	}
	if dashboardPayload.Data["click_count"] != float64(0) {
		t.Fatalf("expected click_count=0, got=%v", dashboardPayload.Data["click_count"])
	}
	if dashboardPayload.Data["min_withdraw_amount"] != float64(10) {
		t.Fatalf("expected min_withdraw_amount=10, got=%v", dashboardPayload.Data["min_withdraw_amount"])
	}
	withdrawChannels, ok := dashboardPayload.Data["withdraw_channels"].([]any)
	if !ok || len(withdrawChannels) != 3 {
		t.Fatalf("expected 3 withdraw channels, got=%T len=%d", dashboardPayload.Data["withdraw_channels"], len(withdrawChannels))
	}
}

func TestChannelAffiliateListsCommissionAndWithdrawRecords(t *testing.T) {
	db, server := setupChannelAffiliateHandlerTest(t)

	user := userdomain.User{
		Email:     "affiliate-bot@example.com",
		Status:    constants.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	identity := externalidentitydomain.Identity{
		UserID:         user.ID,
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: "556677",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("create identity failed: %v", err)
	}
	profile := affiliatedomain.Profile{
		UserID:        user.ID,
		AffiliateCode: "AFFTEST01",
		Status:        constants.AffiliateProfileStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	order := orderdomain.Order{
		UserID:         user.ID,
		OrderNo:        "DJ-AFF-001",
		Status:         constants.OrderStatusPaid,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.RequireFromString("100.00")),
		TotalAmount:    money.FromDecimal(decimal.RequireFromString("100.00")),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	commission := affiliatedomain.Commission{
		AffiliateProfileID: profile.ID,
		OrderID:            order.ID,
		CommissionType:     constants.AffiliateCommissionTypeOrder,
		BaseAmount:         money.FromDecimal(decimal.RequireFromString("100.00")),
		RatePercent:        money.FromDecimal(decimal.RequireFromString("10.00")),
		CommissionAmount:   money.FromDecimal(decimal.RequireFromString("10.00")),
		Status:             constants.AffiliateCommissionStatusAvailable,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := db.Create(&commission).Error; err != nil {
		t.Fatalf("create commission failed: %v", err)
	}
	withdraw := affiliatedomain.WithdrawRequest{
		AffiliateProfileID: profile.ID,
		Amount:             money.FromDecimal(decimal.RequireFromString("10.00")),
		Channel:            "alipay",
		Account:            "demo@pay.test",
		Status:             constants.AffiliateWithdrawStatusPendingReview,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := db.Create(&withdraw).Error; err != nil {
		t.Fatalf("create withdraw failed: %v", err)
	}

	commissionResp, err := http.Get(server.URL + "/api/v1/channel/affiliate/commissions?channel_user_id=556677")
	if err != nil {
		t.Fatalf("get commissions failed: %v", err)
	}
	t.Cleanup(func() { _ = commissionResp.Body.Close() })
	if commissionResp.StatusCode != http.StatusOK {
		t.Fatalf("expected commissions http status 200, got %d", commissionResp.StatusCode)
	}
	commissionPayload := decodeChannelAffiliateResponse(t, commissionResp)
	items, ok := commissionPayload.Data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 commission item, got=%T len=%d", commissionPayload.Data["items"], len(items))
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected commission item map, got=%T", items[0])
	}
	if item["order_no"] != "DJ-AFF-001" {
		t.Fatalf("expected order_no=DJ-AFF-001, got=%v", item["order_no"])
	}
	if item["commission_amount"] != "10.00" {
		t.Fatalf("expected commission_amount=10.00, got=%v", item["commission_amount"])
	}

	withdrawResp, err := http.Get(server.URL + "/api/v1/channel/affiliate/withdraws?channel_user_id=556677")
	if err != nil {
		t.Fatalf("get withdraws failed: %v", err)
	}
	t.Cleanup(func() { _ = withdrawResp.Body.Close() })
	if withdrawResp.StatusCode != http.StatusOK {
		t.Fatalf("expected withdraws http status 200, got %d", withdrawResp.StatusCode)
	}
	withdrawPayload := decodeChannelAffiliateResponse(t, withdrawResp)
	withdrawItems, ok := withdrawPayload.Data["items"].([]any)
	if !ok || len(withdrawItems) != 1 {
		t.Fatalf("expected 1 withdraw item, got=%T len=%d", withdrawPayload.Data["items"], len(withdrawItems))
	}
	withdrawItem, ok := withdrawItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected withdraw item map, got=%T", withdrawItems[0])
	}
	if withdrawItem["channel"] != "alipay" {
		t.Fatalf("expected withdraw channel=alipay, got=%v", withdrawItem["channel"])
	}
	if withdrawItem["amount"] != "10.00" {
		t.Fatalf("expected withdraw amount=10.00, got=%v", withdrawItem["amount"])
	}
}

func TestChannelAffiliateTrackClick(t *testing.T) {
	db, server := setupChannelAffiliateHandlerTest(t)

	raw, err := json.Marshal(map[string]any{
		"channel_user_id": "445566",
		"affiliate_code":  "AFFCLICK1",
		"visitor_key":     "445566",
		"landing_path":    "/telegram/start",
		"referrer":        "telegram_deep_link",
	})
	if err != nil {
		t.Fatalf("marshal click request failed: %v", err)
	}

	resp, err := http.Post(server.URL+"/api/v1/channel/affiliate/click", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post affiliate click failed: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected click http status 200, got %d", resp.StatusCode)
	}

	payload := decodeChannelAffiliateResponse(t, resp)
	if payload.StatusCode != 0 {
		t.Fatalf("expected status_code=0, got %d", payload.StatusCode)
	}

	user := userdomain.User{
		Email:        "affiliate-click@example.com",
		PasswordHash: "telegram-auto",
		Status:       constants.UserStatusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create unrelated user failed: %v", err)
	}
	profile := affiliatedomain.Profile{
		UserID:        user.ID,
		AffiliateCode: "AFFCLICK1",
		Status:        constants.AffiliateProfileStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := db.Save(&profile).Error; err != nil {
		t.Fatalf("save affiliate profile failed: %v", err)
	}

	raw, err = json.Marshal(map[string]any{
		"channel_user_id": "445566",
		"affiliate_code":  "AFFCLICK1",
		"visitor_key":     "445566",
		"landing_path":    "/telegram/start",
		"referrer":        "telegram_deep_link",
	})
	if err != nil {
		t.Fatalf("marshal second click request failed: %v", err)
	}
	resp2, err := http.Post(server.URL+"/api/v1/channel/affiliate/click", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post second affiliate click failed: %v", err)
	}
	t.Cleanup(func() { _ = resp2.Body.Close() })
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected second click http status 200, got %d", resp2.StatusCode)
	}

	var clickCount int64
	if err := db.Model(&affiliatedomain.Click{}).Where("visitor_key = ?", "445566").Count(&clickCount).Error; err != nil {
		t.Fatalf("count affiliate clicks failed: %v", err)
	}
	if clickCount != 1 {
		t.Fatalf("expected 1 affiliate click, got %d", clickCount)
	}
}

func TestChannelAffiliateApplyWithdraw(t *testing.T) {
	db, server := setupChannelAffiliateHandlerTest(t)

	user := userdomain.User{
		Email:     "affiliate-withdraw@example.com",
		Status:    constants.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	identity := externalidentitydomain.Identity{
		UserID:         user.ID,
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: "667788",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("create identity failed: %v", err)
	}
	profile := affiliatedomain.Profile{
		UserID:        user.ID,
		AffiliateCode: "AFFWD001",
		Status:        constants.AffiliateProfileStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	order := orderdomain.Order{
		UserID:         user.ID,
		OrderNo:        "DJ-AFF-WD-001",
		Status:         constants.OrderStatusPaid,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.RequireFromString("200.00")),
		TotalAmount:    money.FromDecimal(decimal.RequireFromString("200.00")),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	commission := affiliatedomain.Commission{
		AffiliateProfileID: profile.ID,
		OrderID:            order.ID,
		CommissionType:     constants.AffiliateCommissionTypeOrder,
		BaseAmount:         money.FromDecimal(decimal.RequireFromString("200.00")),
		RatePercent:        money.FromDecimal(decimal.RequireFromString("10.00")),
		CommissionAmount:   money.FromDecimal(decimal.RequireFromString("20.00")),
		Status:             constants.AffiliateCommissionStatusAvailable,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := db.Create(&commission).Error; err != nil {
		t.Fatalf("create commission failed: %v", err)
	}

	raw, err := json.Marshal(map[string]any{
		"channel_user_id": "667788",
		"amount":          "12.00",
		"channel":         "alipay",
		"account":         "demo@pay.test",
	})
	if err != nil {
		t.Fatalf("marshal withdraw request failed: %v", err)
	}

	resp, err := http.Post(server.URL+"/api/v1/channel/affiliate/withdraws", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post affiliate withdraw failed: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected withdraw apply http status 200, got %d", resp.StatusCode)
	}

	payload := decodeChannelAffiliateResponse(t, resp)
	if payload.StatusCode != 0 {
		t.Fatalf("expected status_code=0, got %d", payload.StatusCode)
	}
	if payload.Data["status"] != constants.AffiliateWithdrawStatusPendingReview {
		t.Fatalf("expected pending_review status, got=%v", payload.Data["status"])
	}
	if payload.Data["amount"] != "12.00" {
		t.Fatalf("expected amount=12.00, got=%v", payload.Data["amount"])
	}

	var withdrawCount int64
	if err := db.Model(&affiliatedomain.WithdrawRequest{}).Where("affiliate_profile_id = ?", profile.ID).Count(&withdrawCount).Error; err != nil {
		t.Fatalf("count withdraw requests failed: %v", err)
	}
	if withdrawCount != 1 {
		t.Fatalf("expected 1 withdraw request, got %d", withdrawCount)
	}
}
