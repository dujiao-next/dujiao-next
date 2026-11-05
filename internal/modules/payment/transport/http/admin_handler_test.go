package paymenthttp

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type fakeAdminPaymentQuery struct {
	payments []paymentdomain.Payment
	owner    map[uint]uint // paymentID -> userID
}

func (f *fakeAdminPaymentQuery) ListPayments(filter AdminPaymentListFilter) ([]paymentdomain.Payment, int64, error) {
	out := make([]paymentdomain.Payment, 0, len(f.payments))
	for _, payment := range f.payments {
		if filter.UserID != 0 && f.owner[payment.ID] != filter.UserID {
			continue
		}
		if filter.OrderID != 0 && payment.OrderID != filter.OrderID {
			continue
		}
		if filter.ChannelID != 0 && payment.ChannelID != filter.ChannelID {
			continue
		}
		out = append(out, payment)
	}
	total := int64(len(out))
	start := (filter.Page - 1) * filter.PageSize
	if start < 0 {
		start = 0
	}
	if filter.PageSize <= 0 {
		return out, total, nil
	}
	if start >= len(out) {
		return nil, total, nil
	}
	end := start + filter.PageSize
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], total, nil
}

func (f *fakeAdminPaymentQuery) GetPayment(id uint) (*paymentdomain.Payment, error) {
	for i := range f.payments {
		if f.payments[i].ID == id {
			payment := f.payments[i]
			return &payment, nil
		}
	}
	return nil, ErrPaymentNotFound
}

type fakeAdminChannelLookup struct {
	names map[uint]string
}

func (f fakeAdminChannelLookup) ListByIDs(ids []uint) ([]paymentdomain.PaymentChannel, error) {
	out := make([]paymentdomain.PaymentChannel, 0, len(ids))
	for _, id := range ids {
		out = append(out, paymentdomain.PaymentChannel{ID: id, Name: f.names[id]})
	}
	return out, nil
}

type fakeAdminOrderLookup struct {
	orderNos map[uint]string
}

type fakeAdminChannelCatalog struct {
	channel            *paymentdomain.PaymentChannel
	securityTestResult *paymentcontract.GatewaySecurityTestResult
	securityTestErr    error
}

func (f *fakeAdminChannelCatalog) ValidateChannel(*paymentdomain.PaymentChannel) error { return nil }
func (f *fakeAdminChannelCatalog) GetChannel(uint) (*paymentdomain.PaymentChannel, error) {
	copied := *f.channel
	copied.ConfigJSON = jsonmap.JSON{}
	for key, value := range f.channel.ConfigJSON {
		copied.ConfigJSON[key] = value
	}
	return &copied, nil
}
func (f *fakeAdminChannelCatalog) ListChannels(AdminChannelListFilter) ([]paymentdomain.PaymentChannel, int64, error) {
	copied, _ := f.GetChannel(f.channel.ID)
	return []paymentdomain.PaymentChannel{*copied}, 1, nil
}
func (f *fakeAdminChannelCatalog) Create(channel *paymentdomain.PaymentChannel) error {
	f.channel = channel
	return nil
}
func (f *fakeAdminChannelCatalog) Update(channel *paymentdomain.PaymentChannel) error {
	f.channel = channel
	return nil
}
func (f *fakeAdminChannelCatalog) Delete(uint) error { return nil }
func (f *fakeAdminChannelCatalog) TestChannelSecurity(context.Context, uint) (*paymentcontract.GatewaySecurityTestResult, error) {
	return f.securityTestResult, f.securityTestErr
}

func TestWechatPayPublicKeySecurityTestReturnsSafeResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := &fakeAdminChannelCatalog{
		channel: &paymentdomain.PaymentChannel{ID: 1},
		securityTestResult: &paymentcontract.GatewaySecurityTestResult{
			VerificationMode:         "wechatpay_public_key",
			ResponseSerial:           "PUB_KEY_ID_3000000001",
			RequestSignatureAccepted: true,
			ResponseSignatureValid:   true,
			EchoMessageMatched:       true,
		},
	}
	handler := NewAdminChannelHandler(catalog)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/payment-channels/1/wechatpay-public-key-test", nil)

	handler.TestWechatPayPublicKey(ctx)

	body := recorder.Body.String()
	if !strings.Contains(body, `"response_serial":"PUB_KEY_ID_3000000001"`) ||
		!strings.Contains(body, `"response_signature_valid":true`) ||
		!strings.Contains(body, `"echo_message_matched":true`) {
		t.Fatalf("unexpected security test response: %s", body)
	}
}

func TestGetPaymentChannelRedactsSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := &fakeAdminChannelCatalog{channel: &paymentdomain.PaymentChannel{
		ID: 1,
		ConfigJSON: jsonmap.JSON{
			"api_base_url":   "https://pay.example",
			"api_key_id":     "key-id",
			"api_secret":     "api-secret-value",
			"webhook_secret": "webhook-secret-value",
			"token":          "usdt",
		},
	}}
	handler := NewAdminChannelHandler(catalog)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/payment-channels/1", nil)

	handler.GetPaymentChannel(ctx)

	body := recorder.Body.String()
	if strings.Contains(body, "api-secret-value") || strings.Contains(body, "webhook-secret-value") {
		t.Fatalf("payment channel response leaked a secret: %s", body)
	}
	if !strings.Contains(body, redactedPaymentConfigValue) {
		t.Fatalf("payment channel response should expose only a redaction marker: %s", body)
	}
	if !strings.Contains(body, "https://pay.example") || !strings.Contains(body, "key-id") {
		t.Fatalf("non-sensitive config should remain editable: %s", body)
	}
	if !strings.Contains(body, `"token":"usdt"`) {
		t.Fatalf("coin token selector is not a credential and must remain editable: %s", body)
	}
}

func TestUpdatePaymentChannelPreservesRedactedSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := &fakeAdminChannelCatalog{channel: &paymentdomain.PaymentChannel{
		ID:           1,
		Name:         "DujiaoPay",
		ProviderType: "dujiaopay",
		ChannelType:  "dujiaopay",
		ConfigJSON: jsonmap.JSON{
			"api_base_url":   "https://old.example",
			"api_secret":     "api-secret-value",
			"webhook_secret": "webhook-secret-value",
		},
	}}
	handler := NewAdminChannelHandler(catalog)
	payload, err := json.Marshal(map[string]interface{}{
		"name": "Updated",
		"config_json": map[string]interface{}{
			"api_base_url":   "https://new.example",
			"api_secret":     redactedPaymentConfigValue,
			"webhook_secret": "",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodPut, "/admin/payment-channels/1", strings.NewReader(string(payload)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.UpdatePaymentChannel(ctx)

	if got := catalog.channel.ConfigJSON["api_secret"]; got != "api-secret-value" {
		t.Fatalf("api_secret should be preserved, got %#v", got)
	}
	if got := catalog.channel.ConfigJSON["webhook_secret"]; got != "webhook-secret-value" {
		t.Fatalf("webhook_secret should be preserved, got %#v", got)
	}
	if got := catalog.channel.ConfigJSON["api_base_url"]; got != "https://new.example" {
		t.Fatalf("non-sensitive config should update, got %#v", got)
	}
}

func TestMergePaymentChannelConfigExplicitNullClearsSecret(t *testing.T) {
	merged := mergePaymentChannelConfig(
		jsonmap.JSON{
			"api_base_url":   "https://old.example",
			"api_secret":     "api-secret-value",
			"webhook_secret": "webhook-secret-value",
		},
		map[string]interface{}{
			"api_base_url":   "https://new.example",
			"api_secret":     nil,
			"webhook_secret": redactedPaymentConfigValue,
		},
	)
	if _, exists := merged["api_secret"]; exists {
		t.Fatalf("explicit null should remove api_secret, got %#v", merged["api_secret"])
	}
	if got := merged["webhook_secret"]; got != "webhook-secret-value" {
		t.Fatalf("redacted webhook_secret should be preserved, got %#v", got)
	}
	if got := merged["api_base_url"]; got != "https://new.example" {
		t.Fatalf("non-sensitive config should update, got %#v", got)
	}
}

func (f fakeAdminOrderLookup) GetByIDs(ids []uint) ([]orderdomain.Order, error) {
	out := make([]orderdomain.Order, 0, len(ids))
	for _, id := range ids {
		out = append(out, orderdomain.Order{ID: id, OrderNo: f.orderNos[id]})
	}
	return out, nil
}

type fakeAdminRechargeLookup struct {
	orders []walletdomain.RechargeOrder
}

func (f fakeAdminRechargeLookup) GetRechargeOrdersByPaymentIDs(paymentIDs []uint) ([]walletdomain.RechargeOrder, error) {
	wanted := make(map[uint]struct{}, len(paymentIDs))
	for _, id := range paymentIDs {
		wanted[id] = struct{}{}
	}
	out := make([]walletdomain.RechargeOrder, 0)
	for _, order := range f.orders {
		if _, ok := wanted[order.PaymentID]; ok {
			out = append(out, order)
		}
	}
	return out, nil
}

type adminPaymentFixture struct {
	User1ID              uint
	User2ID              uint
	OrderPaymentID       uint
	OrderNo              string
	RechargePaymentUser1 uint
	RechargePaymentUser2 uint
	RechargeNoUser1      string
	RechargeNoUser2      string
}

func setupAdminPaymentHandlerTest(t *testing.T) (*AdminHandler, adminPaymentFixture) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC().Truncate(time.Second)

	fixture := adminPaymentFixture{
		User1ID:              1,
		User2ID:              2,
		OrderPaymentID:       11,
		OrderNo:              "DJADMINORDER001",
		RechargePaymentUser1: 21,
		RechargePaymentUser2: 22,
		RechargeNoUser1:      "DJADMINRECHARGE001",
		RechargeNoUser2:      "DJADMINRECHARGE002",
	}

	payments := []paymentdomain.Payment{
		{
			ID:              fixture.OrderPaymentID,
			OrderID:         101,
			ChannelID:       1,
			ProviderType:    constants.PaymentProviderOfficial,
			ChannelType:     constants.PaymentChannelTypeAlipay,
			InteractionMode: constants.PaymentInteractionQR,
			Amount:          money.FromDecimal(decimal.NewFromInt(100)),
			Currency:        "CNY",
			Status:          constants.PaymentStatusPending,
			ProviderRef:     "order_payment_ref_user1",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              fixture.RechargePaymentUser1,
			OrderID:         0,
			ChannelID:       1,
			ProviderType:    constants.PaymentProviderOfficial,
			ChannelType:     constants.PaymentChannelTypeAlipay,
			InteractionMode: constants.PaymentInteractionQR,
			Amount:          money.FromDecimal(decimal.NewFromInt(50)),
			Currency:        "CNY",
			Status:          constants.PaymentStatusPending,
			ProviderRef:     "recharge_payment_ref_user1",
			CreatedAt:       now.Add(time.Second),
			UpdatedAt:       now.Add(time.Second),
		},
		{
			ID:              fixture.RechargePaymentUser2,
			OrderID:         0,
			ChannelID:       2,
			ProviderType:    constants.PaymentProviderOfficial,
			ChannelType:     constants.PaymentChannelTypeWechat,
			InteractionMode: constants.PaymentInteractionQR,
			Amount:          money.FromDecimal(decimal.NewFromInt(60)),
			Currency:        "CNY",
			Status:          constants.PaymentStatusPending,
			ProviderRef:     "recharge_payment_ref_user2",
			CreatedAt:       now.Add(2 * time.Second),
			UpdatedAt:       now.Add(2 * time.Second),
		},
	}

	h := NewAdminHandler(
		&fakeAdminPaymentQuery{
			payments: payments,
			owner: map[uint]uint{
				fixture.OrderPaymentID:       fixture.User1ID,
				fixture.RechargePaymentUser1: fixture.User1ID,
				fixture.RechargePaymentUser2: fixture.User2ID,
			},
		},
		fakeAdminChannelLookup{names: map[uint]string{1: "alipay", 2: "wechat"}},
		fakeAdminOrderLookup{orderNos: map[uint]string{101: fixture.OrderNo}},
		fakeAdminRechargeLookup{orders: []walletdomain.RechargeOrder{
			{PaymentID: fixture.RechargePaymentUser1, RechargeNo: fixture.RechargeNoUser1, UserID: fixture.User1ID, Status: constants.WalletRechargeStatusPending},
			{PaymentID: fixture.RechargePaymentUser2, RechargeNo: fixture.RechargeNoUser2, UserID: fixture.User2ID, Status: constants.WalletRechargeStatusPending},
		}},
		noopAdminPaymentConfirmer{},
	)
	return h, fixture
}

func TestBuildAdminPaymentFilterInvalidOrderID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/payments?order_id=bad", nil)

	_, err := buildAdminPaymentFilter(c, 1, 20)
	if err == nil {
		t.Fatalf("expected invalid order_id error")
	}
}

func TestGetAdminPaymentsFiltersByUserID(t *testing.T) {
	h, fixture := setupAdminPaymentHandlerTest(t)
	query := h.payments.(*fakeAdminPaymentQuery)
	for i := range query.payments {
		if query.payments[i].ID == fixture.OrderPaymentID {
			query.payments[i].ProviderPayload = jsonmap.JSON{"raw_secret": "must-not-leak"}
			query.payments[i].PayURL = "https://checkout.example/private-token"
			query.payments[i].QRCode = "private-qr-payload"
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	url := fmt.Sprintf("/admin/payments?user_id=%d&page=1&page_size=20", fixture.User1ID)
	c.Request = httptest.NewRequest(http.MethodGet, url, nil)

	h.GetAdminPayments(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}

	var resp struct {
		StatusCode int                      `json:"status_code"`
		Pagination responsePaginationAssert `json:"pagination"`
		Data       []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.StatusCode != 0 {
		t.Fatalf("status_code want 0 got %d", resp.StatusCode)
	}
	if resp.Pagination.Total != 2 {
		t.Fatalf("pagination total want 2 got %d", resp.Pagination.Total)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("data len want 2 got %d", len(resp.Data))
	}

	gotIDs := map[uint]struct{}{}
	for _, row := range resp.Data {
		idRaw, ok := row["id"].(float64)
		if !ok {
			t.Fatalf("row id missing or invalid: %+v", row)
		}
		id := uint(idRaw)
		gotIDs[id] = struct{}{}
		if id == fixture.OrderPaymentID && row["order_no"] != fixture.OrderNo {
			t.Fatalf("order payment order_no want %q got %+v", fixture.OrderNo, row["order_no"])
		}
	}
	if _, ok := gotIDs[fixture.OrderPaymentID]; !ok {
		t.Fatalf("missing order payment id %d", fixture.OrderPaymentID)
	}
	if _, ok := gotIDs[fixture.RechargePaymentUser1]; !ok {
		t.Fatalf("missing user1 recharge payment id %d", fixture.RechargePaymentUser1)
	}
	if _, ok := gotIDs[fixture.RechargePaymentUser2]; ok {
		t.Fatalf("unexpected user2 recharge payment id %d", fixture.RechargePaymentUser2)
	}
	body := w.Body.String()
	for _, secret := range []string{"must-not-leak", "private-token", "private-qr-payload"} {
		if strings.Contains(body, secret) {
			t.Fatalf("admin payment response leaked %q: %s", secret, body)
		}
	}
}

func TestExportAdminPaymentsByUserID(t *testing.T) {
	h, fixture := setupAdminPaymentHandlerTest(t)
	query := h.payments.(*fakeAdminPaymentQuery)
	for i := range query.payments {
		if query.payments[i].ID == fixture.OrderPaymentID {
			query.payments[i].ProviderPayload = jsonmap.JSON{"display_channel_type": "usdt.arbitrum"}
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	url := fmt.Sprintf("/admin/payments/export?user_id=%d", fixture.User1ID)
	c.Request = httptest.NewRequest(http.MethodGet, url, nil)

	h.ExportAdminPayments(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}
	if contentType := strings.TrimSpace(w.Header().Get("Content-Type")); !strings.HasPrefix(contentType, "text/csv") {
		t.Fatalf("content-type should be csv, got %s", contentType)
	}

	records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv failed: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("csv rows want 3 got %d", len(records))
	}
	header := strings.Join(records[0], ",")
	if header != "id,order_id,recharge_no,recharge_status,recharge_user_id,channel_id,provider_type,channel_type,display_channel_type,status,amount,currency,created_at,paid_at,expired_at,provider_ref" {
		t.Fatalf("csv header mismatch, got %s", header)
	}

	foundRechargeNo := false
	foundDisplayChannelType := false
	for _, row := range records[1:] {
		if len(row) < 9 {
			t.Fatalf("csv row columns too short: %+v", row)
		}
		if row[0] == strconv.FormatUint(uint64(fixture.OrderPaymentID), 10) && row[8] == "usdt.arbitrum" {
			foundDisplayChannelType = true
		}
		if row[2] == fixture.RechargeNoUser1 {
			foundRechargeNo = true
		}
		if row[2] == fixture.RechargeNoUser2 {
			t.Fatalf("csv should not include user2 recharge data")
		}
	}
	if !foundRechargeNo {
		t.Fatalf("csv missing user1 recharge row")
	}
	if !foundDisplayChannelType {
		t.Fatalf("csv missing order payment display_channel_type")
	}
}

type responsePaginationAssert struct {
	Page      int   `json:"page"`
	PageSize  int   `json:"page_size"`
	Total     int64 `json:"total"`
	TotalPage int64 `json:"total_page"`
}

func TestParseAdminPaymentQueryUint(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/payments?user_id=12", nil)

	parsed, err := ginutil.ParseQueryUint(c.Query("user_id"), true)
	if err != nil {
		t.Fatalf("parse user_id failed: %v", err)
	}
	if parsed != 12 {
		t.Fatalf("parsed user_id want 12 got %d", parsed)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/payments?user_id=0", nil)
	_, err = ginutil.ParseQueryUint(c.Query("user_id"), true)
	if err == nil {
		t.Fatalf("expected parse error for user_id=0")
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/payments", nil)
	parsed, err = ginutil.ParseQueryUint(c.Query("user_id"), true)
	if err != nil {
		t.Fatalf("unexpected error for empty query: %v", err)
	}
	if parsed != 0 {
		t.Fatalf("parsed empty user_id want 0 got %d", parsed)
	}
}

func TestWriteAdminPaymentCSVRows(t *testing.T) {
	h, fixture := setupAdminPaymentHandlerTest(t)

	payments, _, err := h.payments.ListPayments(AdminPaymentListFilter{
		Page:     1,
		PageSize: 50,
		UserID:   fixture.User1ID,
	})
	if err != nil {
		t.Fatalf("list payments failed: %v", err)
	}

	builder := &strings.Builder{}
	writer := csv.NewWriter(builder)
	if err := h.writeAdminPaymentCSVRows(writer, payments); err != nil {
		t.Fatalf("write csv rows failed: %v", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush csv rows failed: %v", err)
	}

	rows, err := csv.NewReader(strings.NewReader(builder.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv rows failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("csv row count want 2 got %d", len(rows))
	}

	foundRecharge := false
	for _, row := range rows {
		if len(row) < 3 {
			t.Fatalf("row columns too short: %+v", row)
		}
		if row[2] == fixture.RechargeNoUser1 {
			foundRecharge = true
		}
	}
	if !foundRecharge {
		t.Fatalf("csv rows should include user1 recharge info")
	}
}

func TestGetAdminPaymentsBadQueryReturnsBadRequestCode(t *testing.T) {
	h, _ := setupAdminPaymentHandlerTest(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/payments?channel_id=abc", nil)

	h.GetAdminPayments(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}
	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status_code want 400 got %d", resp.StatusCode)
	}
}

type noopAdminPaymentConfirmer struct{}

func (noopAdminPaymentConfirmer) ManualConfirmPayment(ManualConfirmPaymentInput) (*paymentdomain.Payment, error) {
	return nil, nil
}
