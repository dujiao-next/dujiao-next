package integrationtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	userauthgormstore "github.com/dujiao-next/internal/modules/identity/userauth/infrastructure/gormstore"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	giftcardapp "github.com/dujiao-next/internal/modules/giftcard/application"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	giftcardgormstore "github.com/dujiao-next/internal/modules/giftcard/infrastructure/gormstore"
	giftcardtransport "github.com/dujiao-next/internal/modules/giftcard/transport/http"
	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	emailverificationstore "github.com/dujiao-next/internal/modules/identity/emailverification/infrastructure/gormstore"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	externalidentitystore "github.com/dujiao-next/internal/modules/identity/externalidentity/infrastructure/gormstore"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	giftcardredeemgormuow "github.com/dujiao-next/internal/workflows/giftcardredeem/infrastructure/gormuow"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type channelGiftCardTestResponse struct {
	StatusCode int                    `json:"status_code"`
	Msg        string                 `json:"msg"`
	Data       map[string]interface{} `json:"data"`
	ErrorCode  string                 `json:"error_code"`
}

type giftCardChannelUserProvisioner struct {
	auth *userauthapp.Service
}

func (p giftCardChannelUserProvisioner) ProvisionUserID(channelUserID string) (uint, error) {
	user, _, _, err := p.auth.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{
		ChannelUserID: channelUserID,
	})
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, userauthapp.ErrNotFound
	}
	return user.ID, nil
}

func setupChannelGiftCardHandlerTest(t *testing.T) (*gorm.DB, *httptest.Server) {
	t.Helper()

	dsn := fmt.Sprintf("file:channel_wallet_gift_card_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&externalidentitydomain.Identity{},
		&emailverificationdomain.Code{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&settingsstore.SettingRecord{},
		&giftcarddomain.GiftCardBatch{},
		&giftcarddomain.GiftCard{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	gormdb.DB = db

	userRepo := userstore.New(db)
	identityRepo := externalidentitystore.New(db)
	emailVerifyRepo := emailverificationstore.New(db)
	walletStore := walletgormstore.New(db)
	settingRepo := settingsstore.New(db)
	giftCardRepo := giftcardgormstore.New(db)

	settingSvc := settingsapp.NewService(settingRepo)
	walletSvc := walletapp.NewService(walletapp.Options{
		Repository:   walletStore,
		Transactions: walletStore,
	})
	giftCardSvc := giftcardapp.NewService(giftcardapp.Options{
		Repo:     giftCardRepo,
		Users:    userRepo,
		Currency: giftCardTestCurrencyProvider{settings: settingSvc},
		Redeemer: giftcardredeemgormuow.New(giftCardRepo, walletSvc),
	})
	userAuthSvc := userauthapp.NewService(&config.Config{}, userRepo, identityRepo, emailVerifyRepo, nil, nil, nil)
	userAuthSvc.SetAuthUnitOfWork(userauthgormstore.New(db))

	handler := giftcardtransport.NewChannelHandler(giftCardSvc, giftCardChannelUserProvisioner{auth: userAuthSvc})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	giftcardtransport.RegisterChannelRoutes(router.Group("/api/v1/channel"), handler)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return db, server
}

func seedChannelGiftCard(t *testing.T, db *gorm.DB, card giftcarddomain.GiftCard) giftcarddomain.GiftCard {
	t.Helper()
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create gift card failed: %v", err)
	}
	return card
}

func postChannelGiftCardRedeem(t *testing.T, server *httptest.Server, body map[string]interface{}) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}
	resp, err := http.Post(server.URL+"/api/v1/channel/wallet/gift-card/redeem", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post redeem gift card failed: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func decodeChannelGiftCardResponse(t *testing.T, resp *http.Response) channelGiftCardTestResponse {
	t.Helper()
	var payload channelGiftCardTestResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	return payload
}

func TestRedeemGiftCardChannelHandlerSuccess(t *testing.T) {
	db, server := setupChannelGiftCardHandlerTest(t)
	card := seedChannelGiftCard(t, db, giftcarddomain.GiftCard{
		Name:      "Telegram 礼品卡",
		Code:      "GC-CHANNEL-SUCCESS-001",
		Amount:    money.FromDecimal(decimal.RequireFromString("88.80")),
		Currency:  "CNY",
		Status:    giftcarddomain.GiftCardStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	resp := postChannelGiftCardRedeem(t, server, map[string]interface{}{
		"telegram_user_id": "998877",
		"code":             card.Code,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected http status 200, got %d", resp.StatusCode)
	}

	payload := decodeChannelGiftCardResponse(t, resp)
	if payload.StatusCode != 0 {
		t.Fatalf("expected status_code=0, got %d", payload.StatusCode)
	}
	if payload.Msg != "success" {
		t.Fatalf("expected msg=success, got %s", payload.Msg)
	}

	giftCardData, ok := payload.Data["gift_card"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected gift_card map, got %T", payload.Data["gift_card"])
	}
	if giftCardData["code"] != card.Code {
		t.Fatalf("expected gift_card.code=%s, got %v", card.Code, giftCardData["code"])
	}
	if giftCardData["status"] != giftcarddomain.GiftCardStatusRedeemed {
		t.Fatalf("expected gift_card.status=redeemed, got %v", giftCardData["status"])
	}

	walletData, ok := payload.Data["wallet"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected wallet map, got %T", payload.Data["wallet"])
	}
	if walletData["balance"] != "88.80" {
		t.Fatalf("expected wallet.balance=88.80, got %v", walletData["balance"])
	}

	txnData, ok := payload.Data["transaction"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected transaction map, got %T", payload.Data["transaction"])
	}
	if txnData["type"] != constants.WalletTxnTypeGiftCard {
		t.Fatalf("expected transaction.type=%s, got %v", constants.WalletTxnTypeGiftCard, txnData["type"])
	}
	if payload.Data["wallet_delta"] != "88.80" {
		t.Fatalf("expected wallet_delta=88.80, got %v", payload.Data["wallet_delta"])
	}

	var identity externalidentitydomain.Identity
	if err := db.Where("provider = ? AND provider_user_id = ?", constants.UserOAuthProviderTelegram, "998877").First(&identity).Error; err != nil {
		t.Fatalf("expected provisioned telegram identity: %v", err)
	}

	var account walletdomain.Account
	if err := db.Where("user_id = ?", identity.UserID).First(&account).Error; err != nil {
		t.Fatalf("expected wallet account: %v", err)
	}
	if account.Balance.String() != "88.80" {
		t.Fatalf("expected stored wallet balance=88.80, got %s", account.Balance.String())
	}

	var refreshedCard giftcarddomain.GiftCard
	if err := db.First(&refreshedCard, card.ID).Error; err != nil {
		t.Fatalf("reload gift card failed: %v", err)
	}
	if refreshedCard.Status != giftcarddomain.GiftCardStatusRedeemed {
		t.Fatalf("expected stored gift card status redeemed, got %s", refreshedCard.Status)
	}
	if refreshedCard.RedeemedUserID == nil || *refreshedCard.RedeemedUserID != identity.UserID {
		t.Fatalf("expected redeemed_user_id=%d, got %+v", identity.UserID, refreshedCard.RedeemedUserID)
	}
}

func TestRedeemGiftCardChannelHandlerReturnsMappedRedeemedError(t *testing.T) {
	db, server := setupChannelGiftCardHandlerTest(t)
	redeemedUserID := uint(321)
	redeemedAt := time.Now().Add(-10 * time.Minute)
	seedChannelGiftCard(t, db, giftcarddomain.GiftCard{
		Name:           "已兑换礼品卡",
		Code:           "GC-CHANNEL-REDEEMED-001",
		Amount:         money.FromDecimal(decimal.RequireFromString("50.00")),
		Currency:       "CNY",
		Status:         giftcarddomain.GiftCardStatusRedeemed,
		RedeemedAt:     &redeemedAt,
		RedeemedUserID: &redeemedUserID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})

	resp := postChannelGiftCardRedeem(t, server, map[string]interface{}{
		"channel_user_id": "556677",
		"code":            "GC-CHANNEL-REDEEMED-001",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected http status 400, got %d", resp.StatusCode)
	}

	payload := decodeChannelGiftCardResponse(t, resp)
	if payload.StatusCode != 400 {
		t.Fatalf("expected status_code=400, got %d", payload.StatusCode)
	}
	if payload.ErrorCode != "gift_card_redeemed" {
		t.Fatalf("expected error_code=gift_card_redeemed, got %s", payload.ErrorCode)
	}

	var identity externalidentitydomain.Identity
	if err := db.Where("provider = ? AND provider_user_id = ?", constants.UserOAuthProviderTelegram, "556677").First(&identity).Error; err != nil {
		t.Fatalf("expected provisioned telegram identity on failure path: %v", err)
	}

	var walletCount int64
	if err := db.Model(&walletdomain.Account{}).Where("user_id = ?", identity.UserID).Count(&walletCount).Error; err != nil {
		t.Fatalf("count wallet accounts failed: %v", err)
	}
	if walletCount != 0 {
		t.Fatalf("expected no wallet account created on redeemed error, got %d", walletCount)
	}
}
