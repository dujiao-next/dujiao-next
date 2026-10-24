package channelhttp

import (
	"encoding/json"
	"testing"
	"time"

	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"net/http"
	"net/http/httptest"
	"strings"
)

func TestBuildChannelIdentityResponse(t *testing.T) {
	user := &userdomain.User{
		ID:                    12,
		Email:                 "telegram_12@login.local",
		DisplayName:           "TG Buyer",
		Status:                "active",
		Locale:                "zh-CN",
		PasswordSetupRequired: true,
	}
	identity := &externalidentitydomain.Identity{
		Provider:       "telegram",
		ProviderUserID: "12",
		Username:       "buyer12",
		AvatarURL:      "https://example.com/avatar.png",
	}

	payload := buildChannelIdentityResponse(true, true, user, identity)
	if payload["bound"] != true {
		t.Fatalf("bound flag mismatch: %#v", payload["bound"])
	}
	if payload["created"] != true {
		t.Fatalf("created flag mismatch: %#v", payload["created"])
	}
	identityPayload, ok := payload["identity"].(gin.H)
	if !ok {
		t.Fatalf("identity payload type mismatch: %#v", payload["identity"])
	}
	if identityPayload["provider_user_id"] != "12" {
		t.Fatalf("provider user id mismatch: %#v", identityPayload["provider_user_id"])
	}
	userPayload, ok := payload["user"].(gin.H)
	if !ok {
		t.Fatalf("user payload type mismatch: %#v", payload["user"])
	}
	if userPayload["display_name"] != "TG Buyer" {
		t.Fatalf("display name mismatch: %#v", userPayload["display_name"])
	}
}

func TestBuildChannelOrderPreviewResponseIncludesTelegramFriendlyFields(t *testing.T) {
	resp := buildChannelOrderPreviewResponse(&OrderPreview{
		Currency:                "CNY",
		OriginalAmount:          money.FromDecimal(decimal.RequireFromString("108.00")),
		DiscountAmount:          money.FromDecimal(decimal.RequireFromString("8.00")),
		PromotionDiscountAmount: money.FromDecimal(decimal.RequireFromString("10.00")),
		TotalAmount:             money.FromDecimal(decimal.RequireFromString("90.00")),
		Items: []OrderPreviewItem{{
			ProductID:          12,
			SKUID:              34,
			TitleJSON:          jsonmap.JSON{"zh-CN": "会员订阅"},
			SKUSnapshotJSON:    jsonmap.JSON{"spec_values": jsonmap.JSON{"zh-CN": "季度版"}},
			Quantity:           2,
			OriginalUnitPrice:  money.FromDecimal(decimal.RequireFromString("60.00")),
			UnitPrice:          money.FromDecimal(decimal.RequireFromString("54.00")),
			OriginalTotalPrice: money.FromDecimal(decimal.RequireFromString("120.00")),
			TotalPrice:         money.FromDecimal(decimal.RequireFromString("108.00")),
			CouponDiscount:     money.FromDecimal(decimal.RequireFromString("8.00")),
			PromotionDiscount:  money.FromDecimal(decimal.RequireFromString("10.00")),
			FulfillmentType:    "manual",
		}},
	}, "zh-CN")

	if got := resp["item_count"]; got != 1 {
		t.Fatalf("expected item_count=1, got=%v", got)
	}
	if got := resp["original_amount"]; got != "108.00" {
		t.Fatalf("expected original_amount=108.00, got=%v", got)
	}
	items, ok := resp["items"].([]gin.H)
	if !ok || len(items) != 1 {
		t.Fatalf("expected single preview item, got=%T len=%d", resp["items"], len(items))
	}
	if got := items[0]["coupon_discount"]; got != "8.00" {
		t.Fatalf("expected coupon_discount=8.00, got=%v", got)
	}
	if got := items[0]["original_unit_price"]; got != "60.00" {
		t.Fatalf("expected original_unit_price=60.00, got=%v", got)
	}
	if got := items[0]["original_total_price"]; got != "120.00" {
		t.Fatalf("expected original_total_price=120.00, got=%v", got)
	}
	if got := items[0]["promotion_discount"]; got != "10.00" {
		t.Fatalf("expected promotion_discount=10.00, got=%v", got)
	}
	if got := items[0]["fulfillment_type"]; got != "manual" {
		t.Fatalf("expected fulfillment_type=manual, got=%v", got)
	}
}

func TestNormalizeChannelManualFormSchemaUsesLocaleText(t *testing.T) {
	schema := jsonmap.JSON{"fields": []interface{}{
		map[string]interface{}{
			"key": "account", "type": "text", "required": true,
			"label":       map[string]interface{}{"zh-CN": "充值账号", "en-US": "Account"},
			"placeholder": map[string]interface{}{"zh-CN": "请输入账号", "en-US": "Enter account"},
		},
		map[string]interface{}{
			"key": "server", "type": "radio", "required": false,
			"label": "区服", "options": []interface{}{"亚服", "国际服"},
		},
	}}
	got := normalizeChannelManualFormSchema(schema, "zh-CN", "en-US")
	fields, ok := got["fields"].([]gin.H)
	if !ok || len(fields) != 2 {
		t.Fatalf("expected 2 fields, got=%T len=%d", got["fields"], len(fields))
	}
	if fields[0]["label"] != "充值账号" || fields[0]["placeholder"] != "请输入账号" {
		t.Fatalf("unexpected localized field: %#v", fields[0])
	}
	options, ok := fields[1]["options"].([]string)
	if !ok || len(options) != 2 {
		t.Fatalf("expected options list, got=%T %#v", fields[1]["options"], fields[1]["options"])
	}
}

func TestBuildChannelOrderDetailResponseUsesTotalPaidAmount(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	resp := buildChannelOrderDetailResponse(&orderdomain.Order{
		ID:                      7,
		OrderNo:                 "DJ20260310001",
		Status:                  "paid",
		Currency:                "CNY",
		OriginalAmount:          money.FromDecimal(decimal.RequireFromString("100.00")),
		DiscountAmount:          money.FromDecimal(decimal.RequireFromString("5.00")),
		PromotionDiscountAmount: money.FromDecimal(decimal.RequireFromString("15.00")),
		TotalAmount:             money.FromDecimal(decimal.RequireFromString("80.00")),
		WalletPaidAmount:        money.FromDecimal(decimal.RequireFromString("20.00")),
		OnlinePaidAmount:        money.FromDecimal(decimal.RequireFromString("60.00")),
		RefundedAmount:          money.FromDecimal(decimal.RequireFromString("0.00")),
		ExpiresAt:               &now,
		CreatedAt:               now,
		UpdatedAt:               now,
		PaidAt:                  &now,
		Items: []orderdomain.OrderItem{{
			ProductID:          1,
			SKUID:              2,
			TitleJSON:          jsonmap.JSON{"zh-CN": "测试商品"},
			SKUSnapshotJSON:    jsonmap.JSON{"spec_values": jsonmap.JSON{"zh-CN": "标准版"}},
			Quantity:           1,
			OriginalUnitPrice:  money.FromDecimal(decimal.RequireFromString("100.00")),
			UnitPrice:          money.FromDecimal(decimal.RequireFromString("80.00")),
			OriginalTotalPrice: money.FromDecimal(decimal.RequireFromString("100.00")),
			TotalPrice:         money.FromDecimal(decimal.RequireFromString("80.00")),
			CouponDiscount:     money.FromDecimal(decimal.RequireFromString("5.00")),
			PromotionDiscount:  money.FromDecimal(decimal.RequireFromString("15.00")),
			FulfillmentType:    "manual",
		}},
		Children: []orderdomain.Order{{
			ID:      8,
			OrderNo: "DJ20260310001-01",
			Status:  "completed",
			Fulfillment: &fulfillmentdomain.Fulfillment{
				Status:      "delivered",
				Type:        "auto",
				Payload:     "card-secret-demo",
				DeliveredAt: &now,
			},
		}},
	}, "zh-CN")

	if got := resp["paid_amount"]; got != "80.00" {
		t.Fatalf("expected paid_amount=80.00, got=%v", got)
	}
	if got := resp["wallet_paid_amount"]; got != "20.00" {
		t.Fatalf("expected wallet_paid_amount=20.00, got=%v", got)
	}
	if got := resp["online_paid_amount"]; got != "60.00" {
		t.Fatalf("expected online_paid_amount=60.00, got=%v", got)
	}
	if got := resp["item_count"]; got != 1 {
		t.Fatalf("expected item_count=1, got=%v", got)
	}
	if got := resp["fulfillment_type"]; got != "manual" {
		t.Fatalf("expected fulfillment_type=manual, got=%v", got)
	}
	items, ok := resp["items"].([]gin.H)
	if !ok || len(items) != 1 {
		t.Fatalf("expected single order item, got=%T len=%d", resp["items"], len(items))
	}
	if got := items[0]["coupon_discount"]; got != "5.00" {
		t.Fatalf("expected coupon_discount=5.00, got=%v", got)
	}
	if got := items[0]["original_unit_price"]; got != "100.00" {
		t.Fatalf("expected original_unit_price=100.00, got=%v", got)
	}
	if got := items[0]["original_total_price"]; got != "100.00" {
		t.Fatalf("expected original_total_price=100.00, got=%v", got)
	}
	if got := items[0]["promotion_discount"]; got != "15.00" {
		t.Fatalf("expected promotion_discount=15.00, got=%v", got)
	}
	if got := resp["fulfillment_delivered_at"]; got != nil {
		t.Fatalf("expected parent fulfillment_delivered_at=nil, got=%v", got)
	}
	children, ok := resp["children"].([]gin.H)
	if !ok || len(children) != 1 {
		t.Fatalf("expected single child order, got=%T len=%d", resp["children"], len(children))
	}
	childFulfillment, ok := children[0]["fulfillment"].(gin.H)
	if !ok {
		t.Fatalf("expected child fulfillment payload, got=%T", children[0]["fulfillment"])
	}
	if got := childFulfillment["payload"]; got != "card-secret-demo" {
		t.Fatalf("expected child fulfillment payload=card-secret-demo, got=%v", got)
	}
	if got := childFulfillment["status"]; got != "delivered" {
		t.Fatalf("expected child fulfillment status=delivered, got=%v", got)
	}
}

func TestBuildChannelPaymentResponseIncludesOrderSummary(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 30, 0, 0, time.UTC)
	order := &orderdomain.Order{
		ID:               9,
		OrderNo:          "DJ20260310002",
		TotalAmount:      money.FromDecimal(decimal.RequireFromString("99.00")),
		WalletPaidAmount: money.FromDecimal(decimal.RequireFromString("19.00")),
		OnlinePaidAmount: money.FromDecimal(decimal.RequireFromString("80.00")),
	}
	payment := &paymentdomain.Payment{
		ID:              5,
		OrderID:         order.ID,
		ChannelID:       11,
		Status:          "pending",
		ProviderType:    "alipay",
		ChannelType:     "alipay",
		InteractionMode: "redirect",
		Amount:          money.FromDecimal(decimal.RequireFromString("80.80")),
		FeeRate:         money.FromDecimal(decimal.RequireFromString("1.00")),
		FeeAmount:       money.FromDecimal(decimal.RequireFromString("0.80")),
		Currency:        "CNY",
		PayURL:          "https://pay.example.com",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	resp := buildChannelPaymentResponse(order, payment)
	if got := resp["channel_id"]; got != uint(11) {
		t.Fatalf("expected channel_id=11, got=%v", got)
	}
	if got := resp["fee_amount"]; got != "0.80" {
		t.Fatalf("expected fee_amount=0.80, got=%v", got)
	}
	if got := resp["order_no"]; got != "DJ20260310002" {
		t.Fatalf("expected order_no=DJ20260310002, got=%v", got)
	}
	if got := resp["paid_amount"]; got != "99.00" {
		t.Fatalf("expected paid_amount=99.00, got=%v", got)
	}
}

func TestPreviewOrderRequestBindsAffiliateFields(t *testing.T) {
	raw := []byte(`{
		"channel_user_id":"998877",
		"telegram_user_id":"998877",
		"items":[{"product_id":12,"sku_id":34,"quantity":1,"fulfillment_type":"manual"}],
		"affiliate_code":"AFFTG001",
		"affiliate_visitor_key":"visitor-998877"
	}`)

	var req previewOrderRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal preview order request failed: %v", err)
	}
	if req.AffiliateCode != "AFFTG001" {
		t.Fatalf("expected affiliate code to bind, got=%s", req.AffiliateCode)
	}
	if req.AffiliateKey != "visitor-998877" {
		t.Fatalf("expected affiliate visitor key to bind, got=%s", req.AffiliateKey)
	}
}

func TestCreateOrderRequestBindsAffiliateFields(t *testing.T) {
	raw := []byte(`{
		"channel_user_id":"556677",
		"telegram_user_id":"556677",
		"product_id":12,
		"sku_id":34,
		"quantity":2,
		"affiliate_code":"AFFTG002",
		"affiliate_visitor_key":"visitor-556677"
	}`)

	var req createOrderRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal create order request failed: %v", err)
	}
	if req.AffiliateCode != "AFFTG002" {
		t.Fatalf("expected affiliate code to bind, got=%s", req.AffiliateCode)
	}
	if req.AffiliateKey != "visitor-556677" {
		t.Fatalf("expected affiliate visitor key to bind, got=%s", req.AffiliateKey)
	}
}

func TestBuildChannelPaymentResponse_ProviderModeMatrix(t *testing.T) {
	type wantUSDT struct {
		address string
		amount  string
		chain   string
		tokenID string
	}
	tests := []struct {
		name            string
		providerType    string
		channelType     string
		interactionMode string
		payload         jsonmap.JSON
		wantUSDT        *wantUSDT // nil => no wallet_address/chain_amount keys expected
	}{
		{name: "alipay qr", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypeAlipay, interactionMode: constants.PaymentInteractionQR},
		{name: "alipay wap", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypeAlipay, interactionMode: constants.PaymentInteractionWAP},
		{name: "alipay page", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypeAlipay, interactionMode: constants.PaymentInteractionPage},
		{name: "wechat qr", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypeWechat, interactionMode: constants.PaymentInteractionQR},
		{name: "wechat redirect", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypeWechat, interactionMode: constants.PaymentInteractionRedirect},
		{name: "epay qr", providerType: constants.PaymentProviderEpay, channelType: "", interactionMode: constants.PaymentInteractionQR},
		{name: "epay redirect", providerType: constants.PaymentProviderEpay, channelType: "", interactionMode: constants.PaymentInteractionRedirect},
		{
			name:            "bepusdt qr with usdt payload",
			providerType:    constants.PaymentProviderBepusdt,
			channelType:     "",
			interactionMode: constants.PaymentInteractionQR,
			payload:         jsonmap.JSON{"data": map[string]any{"token": "TBepAddr", "actual_amount": "13.45"}},
			wantUSDT:        &wantUSDT{address: "TBepAddr", amount: "13.45"},
		},
		{name: "bepusdt redirect", providerType: constants.PaymentProviderBepusdt, channelType: "", interactionMode: constants.PaymentInteractionRedirect, payload: jsonmap.JSON{"data": map[string]any{"token": "TBepAddr", "actual_amount": "13.45"}}},
		{
			name:            "epusdt qr with receive_address payload",
			providerType:    constants.PaymentProviderEpusdt,
			channelType:     "",
			interactionMode: constants.PaymentInteractionQR,
			payload:         jsonmap.JSON{"data": map[string]any{"receive_address": "TEpusdtAddr", "actual_amount": "9.99"}},
			wantUSDT:        &wantUSDT{address: "TEpusdtAddr", amount: "9.99"},
		},
		{
			name:            "epusdt qr without receive_address still surfaces chain amount",
			providerType:    constants.PaymentProviderEpusdt,
			channelType:     "",
			interactionMode: constants.PaymentInteractionQR,
			payload:         jsonmap.JSON{"data": map[string]any{"actual_amount": "5.00"}},
			wantUSDT:        &wantUSDT{address: "", amount: "5.00"},
		},
		{name: "epusdt redirect", providerType: constants.PaymentProviderEpusdt, channelType: "", interactionMode: constants.PaymentInteractionRedirect},
		{
			name:            "dujiaopay qr with wallet payload",
			providerType:    constants.PaymentProviderDujiaoPay,
			channelType:     "tron-usdt",
			interactionMode: constants.PaymentInteractionQR,
			payload: jsonmap.JSON{
				"chain":          "tron",
				"token_id":       "tron-usdt",
				"pay_address":    "TDujiaoAddr",
				"payable_amount": "10.0001",
			},
			wantUSDT: &wantUSDT{address: "TDujiaoAddr", amount: "10.0001", chain: "tron", tokenID: "tron-usdt"},
		},
		{name: "okpay qr", providerType: constants.PaymentProviderOkpay, channelType: "", interactionMode: constants.PaymentInteractionQR},
		{name: "okpay redirect", providerType: constants.PaymentProviderOkpay, channelType: "", interactionMode: constants.PaymentInteractionRedirect},
		{name: "stripe redirect", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypeStripe, interactionMode: constants.PaymentInteractionRedirect},
		{name: "paypal redirect", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypePaypal, interactionMode: constants.PaymentInteractionRedirect},
		{name: "tokenpay qr", providerType: constants.PaymentProviderTokenpay, channelType: "", interactionMode: constants.PaymentInteractionQR},
		{name: "tokenpay redirect", providerType: constants.PaymentProviderTokenpay, channelType: "", interactionMode: constants.PaymentInteractionRedirect},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			channelType := tc.channelType
			if channelType == "" {
				channelType = "test-channel-type"
			}
			payment := &paymentdomain.Payment{
				ID:              1,
				OrderID:         2,
				ChannelID:       3,
				ProviderType:    tc.providerType,
				ChannelType:     channelType,
				InteractionMode: tc.interactionMode,
				Status:          constants.PaymentStatusPending,
				Amount:          money.FromDecimal(decimal.RequireFromString("100")),
				FeeRate:         money.Amount{},
				FeeAmount:       money.Amount{},
				Currency:        "CNY",
				PayURL:          "https://pay.example.com/c/abc",
				QRCode:          "https://pay.example.com/c/abc-qr",
				ProviderPayload: tc.payload,
			}
			resp := buildChannelPaymentResponse(nil, payment)
			if got := resp["interaction_mode"]; got != tc.interactionMode {
				t.Errorf("interaction_mode: got %v want %v", got, tc.interactionMode)
			}
			if got := resp["pay_url"]; got != "https://pay.example.com/c/abc" {
				t.Errorf("pay_url: got %v", got)
			}
			if got := resp["qr_code"]; got != "https://pay.example.com/c/abc-qr" {
				t.Errorf("qr_code: got %v", got)
			}
			if tc.wantUSDT == nil {
				if _, ok := resp["wallet_address"]; ok {
					t.Errorf("wallet_address should be absent, got %v", resp["wallet_address"])
				}
				if _, ok := resp["chain_amount"]; ok {
					t.Errorf("chain_amount should be absent, got %v", resp["chain_amount"])
				}
				if _, ok := resp["chain"]; ok {
					t.Errorf("chain should be absent, got %v", resp["chain"])
				}
				if _, ok := resp["token_id"]; ok {
					t.Errorf("token_id should be absent, got %v", resp["token_id"])
				}
				return
			}
			if tc.wantUSDT.address == "" {
				if _, ok := resp["wallet_address"]; ok {
					t.Errorf("wallet_address should be absent, got %v", resp["wallet_address"])
				}
			} else if got := resp["wallet_address"]; got != tc.wantUSDT.address {
				t.Errorf("wallet_address: got %v want %v", got, tc.wantUSDT.address)
			}
			if tc.wantUSDT.amount == "" {
				if _, ok := resp["chain_amount"]; ok {
					t.Errorf("chain_amount should be absent, got %v", resp["chain_amount"])
				}
			} else if got := resp["chain_amount"]; got != tc.wantUSDT.amount {
				t.Errorf("chain_amount: got %v want %v", got, tc.wantUSDT.amount)
			}
			if tc.wantUSDT.chain == "" {
				if _, ok := resp["chain"]; ok {
					t.Errorf("chain should be absent, got %v", resp["chain"])
				}
			} else if got := resp["chain"]; got != tc.wantUSDT.chain {
				t.Errorf("chain: got %v want %v", got, tc.wantUSDT.chain)
			}
			if tc.wantUSDT.tokenID == "" {
				if _, ok := resp["token_id"]; ok {
					t.Errorf("token_id should be absent, got %v", resp["token_id"])
				}
			} else if got := resp["token_id"]; got != tc.wantUSDT.tokenID {
				t.Errorf("token_id: got %v want %v", got, tc.wantUSDT.tokenID)
			}
		})
	}
}

func TestBuildChannelPaymentResponse_USDTQRExposesWalletFields(t *testing.T) {
	payment := &paymentdomain.Payment{
		ID:              42,
		OrderID:         1,
		ChannelID:       2,
		ProviderType:    constants.PaymentProviderBepusdt,
		ChannelType:     "usdt-trc20",
		InteractionMode: constants.PaymentInteractionQR,
		Status:          constants.PaymentStatusPending,
		Amount:          money.FromDecimal(decimal.RequireFromString("100.00")),
		FeeRate:         money.Amount{},
		FeeAmount:       money.Amount{},
		Currency:        "CNY",
		PayURL:          "https://pay.example.com/c/abc",
		QRCode:          "https://pay.example.com/c/abc",
		ProviderPayload: jsonmap.JSON{
			"data": map[string]any{
				"token":         "TXxxxxx",
				"actual_amount": "13.45",
			},
		},
	}
	resp := buildChannelPaymentResponse(nil, payment)
	if got := resp["wallet_address"]; got != "TXxxxxx" {
		t.Fatalf("wallet_address: got %v want TXxxxxx", got)
	}
	if got := resp["chain_amount"]; got != "13.45" {
		t.Fatalf("chain_amount: got %v want 13.45", got)
	}
	if got := resp["interaction_mode"]; got != constants.PaymentInteractionQR {
		t.Fatalf("interaction_mode: got %v want qr", got)
	}
}

// inviteIdentityService 模拟邀请注册模式：只有 GOOD 能开号。
type inviteIdentityService struct {
	IdentityService
	gotCode string
}

func (s *inviteIdentityService) ProvisionTelegramChannelIdentity(input TelegramIdentityInput) (*userdomain.User, *externalidentitydomain.Identity, bool, error) {
	s.gotCode = input.InviteCode
	switch input.InviteCode {
	case "":
		return nil, nil, false, userauthapp.ErrInviteCodeRequired
	case "GOOD":
		return &userdomain.User{ID: 1}, nil, true, nil
	default:
		return nil, nil, false, userauthapp.ErrInviteCodeInvalid
	}
}

// Telegram Bot 依赖这些 error_code 字符串判断是否提示输入邀请码，改动即为对 Bot 的破坏性变更。
func TestProvisionTelegramIdentityInviteErrorCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		inviteCode string
		wantStatus int
		wantCode   string
	}{
		{"", http.StatusForbidden, "invite_code_required"},
		{"BAD", http.StatusBadRequest, "invite_code_invalid"},
		{"GOOD", http.StatusOK, ""},
	}
	for _, tc := range cases {
		service := &inviteIdentityService{}
		handler := &Handler{Dependencies{UserAuthService: service}}
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		body := `{"channel_user_id":"1001","invite_code":"` + tc.inviteCode + `"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/identities/telegram/provision", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ProvisionTelegramIdentity(c)

		var resp struct {
			ErrorCode string `json:"error_code"`
		}
		_ = json.Unmarshal(recorder.Body.Bytes(), &resp)
		if recorder.Code != tc.wantStatus || resp.ErrorCode != tc.wantCode || service.gotCode != tc.inviteCode {
			t.Fatalf("invite_code=%q: status=%d error_code=%q passed=%q body=%s", tc.inviteCode, recorder.Code, resp.ErrorCode, service.gotCode, recorder.Body.String())
		}
	}
	// 注册关闭也要给出可识别的错误码，而不是 internal_error。
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/orders", nil)
	respondChannelIdentityServiceError(c, userauthapp.ErrRegistrationDisabled)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), `"error_code":"registration_disabled"`) {
		t.Fatalf("registration disabled: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
