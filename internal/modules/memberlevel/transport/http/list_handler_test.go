package memberlevelhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func TestResolveLocalizedJSONPrefersLocaleThenDefault(t *testing.T) {
	m := jsonmap.JSON{
		"en-US": "Gold",
		"zh-CN": "黄金",
	}
	if got := resolveLocalizedJSON(m, "en-US", "zh-CN"); got != "Gold" {
		t.Fatalf("locale prefer: got %q", got)
	}
	if got := resolveLocalizedJSON(m, "ja-JP", "zh-CN"); got != "黄金" {
		t.Fatalf("default prefer: got %q", got)
	}
	if got := resolveLocalizedJSON(jsonmap.JSON{}, "zh-CN", "zh-CN"); got != "" {
		t.Fatalf("empty map: got %q", got)
	}
}

type stubUserPriceReader struct {
	gotUserID uint
	prices    []memberleveldomain.UserProductPrice
}

func (r *stubUserPriceReader) ListUserPrices(userID uint, _, _ int) ([]memberleveldomain.UserProductPrice, int64, error) {
	r.gotUserID = userID
	return r.prices, int64(len(r.prices)), nil
}

// 前台只能读到当前登录用户自己的专属价，且响应不泄露其他字段。
func TestListMyPricesReturnsOnlyCurrentUserPrices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reader := &stubUserPriceReader{prices: []memberleveldomain.UserProductPrice{
		{ID: 1, UserID: 7, ProductID: 18, SKUID: 19, PriceAmount: money.FromDecimal(decimal.NewFromInt(95))},
	}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/me/product-prices", nil)
	c.Set("user_id", uint(7))

	NewUserHandler(reader, nil).ListMyPrices(c)

	if reader.gotUserID != 7 {
		t.Fatalf("reader queried user %d, want 7", reader.gotUserID)
	}
	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	if len(resp.Data) != 1 {
		t.Fatalf("got %d rows, want 1: %s", len(resp.Data), recorder.Body.String())
	}
	row := resp.Data[0]
	if row["product_id"] != float64(18) || row["sku_id"] != float64(19) {
		t.Fatalf("unexpected row: %v", row)
	}
	if _, leaked := row["user_id"]; leaked {
		t.Fatalf("response must not expose user_id: %v", row)
	}
}

func TestListMyPricesRejectsMissingUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reader := &stubUserPriceReader{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/me/product-prices", nil)

	NewUserHandler(reader, nil).ListMyPrices(c)

	if reader.gotUserID != 0 {
		t.Fatalf("reader must not be called without a user, got %d", reader.gotUserID)
	}
}

type stubProductDirectory struct {
	products   map[uint]memberlevelcontract.ProductSummary
	skus       map[uint]memberlevelcontract.SKUSummary
	productErr error
	skuErr     error
	gotSKUIDs  []uint
}

func (d *stubProductDirectory) ResolveProducts(_ []uint) (map[uint]memberlevelcontract.ProductSummary, error) {
	return d.products, d.productErr
}

func (d *stubProductDirectory) ResolveSKUs(ids []uint) (map[uint]memberlevelcontract.SKUSummary, error) {
	d.gotSKUIDs = ids
	return d.skus, d.skuErr
}

func decodePriceRows(t *testing.T, recorder *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	return resp.Data
}

func serveMyPrices(t *testing.T, reader *stubUserPriceReader, dir memberlevelcontract.ProductDirectory) []map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/me/product-prices", nil)
	c.Set("user_id", uint(7))
	NewUserHandler(reader, dir).ListMyPrices(c)
	return decodePriceRows(t, recorder)
}

// 个人中心展示需要商品标题、slug、上架状态与 SKU 规格；商品级专属价（sku_id=0）不带 SKU 信息。
func TestListMyPricesEnrichesProductAndSKUInfo(t *testing.T) {
	reader := &stubUserPriceReader{prices: []memberleveldomain.UserProductPrice{
		{ID: 1, UserID: 7, ProductID: 18, SKUID: 19, PriceAmount: money.FromDecimal(decimal.NewFromInt(95))},
		{ID: 2, UserID: 7, ProductID: 20, SKUID: 0, PriceAmount: money.FromDecimal(decimal.NewFromInt(10))},
	}}
	dir := &stubProductDirectory{
		products: map[uint]memberlevelcontract.ProductSummary{
			18: {Slug: "pro-plan", Title: jsonmap.JSON{"zh-CN": "专业版"}, IsActive: true},
			20: {Slug: "basic", Title: jsonmap.JSON{"zh-CN": "基础版"}, IsActive: false},
		},
		skus: map[uint]memberlevelcontract.SKUSummary{
			19: {SKUCode: "Y1", SpecValues: jsonmap.JSON{"zh-CN": "年付"}},
		},
	}

	rows := serveMyPrices(t, reader, dir)

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %v", len(rows), rows)
	}
	first := rows[0]
	if first["product_slug"] != "pro-plan" || first["product_active"] != true || first["sku_code"] != "Y1" {
		t.Fatalf("first row not enriched: %v", first)
	}
	if title, _ := first["product_title"].(map[string]interface{}); title["zh-CN"] != "专业版" {
		t.Fatalf("first row title: %v", first["product_title"])
	}
	if _, ok := first["sku_spec_values"]; !ok {
		t.Fatalf("first row must carry sku_spec_values: %v", first)
	}
	second := rows[1]
	if second["product_slug"] != "basic" || second["product_active"] != false {
		t.Fatalf("second row not enriched: %v", second)
	}
	if _, ok := second["sku_code"]; ok {
		t.Fatalf("product-level price must not carry sku info: %v", second)
	}
	if len(dir.gotSKUIDs) != 1 || dir.gotSKUIDs[0] != 19 {
		t.Fatalf("only positive sku ids should be resolved, got %v", dir.gotSKUIDs)
	}
}

// 商品或 SKU 已被删除（目录里查不到）时，该行仍返回价格，只是没有展示信息。
func TestListMyPricesKeepsRowWhenProductDeleted(t *testing.T) {
	reader := &stubUserPriceReader{prices: []memberleveldomain.UserProductPrice{
		{ID: 1, UserID: 7, ProductID: 18, SKUID: 19, PriceAmount: money.FromDecimal(decimal.NewFromInt(95))},
	}}

	rows := serveMyPrices(t, reader, &stubProductDirectory{})

	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1: %v", len(rows), rows)
	}
	if rows[0]["product_active"] != false {
		t.Fatalf("missing product must not be reported active: %v", rows[0])
	}
	if _, ok := rows[0]["product_slug"]; ok {
		t.Fatalf("missing product must not carry slug: %v", rows[0])
	}
}

// 目录解析失败不阻断主体返回，前端退化为只有 ID 与价格。
func TestListMyPricesSurvivesDirectoryFailure(t *testing.T) {
	reader := &stubUserPriceReader{prices: []memberleveldomain.UserProductPrice{
		{ID: 1, UserID: 7, ProductID: 18, SKUID: 19, PriceAmount: money.FromDecimal(decimal.NewFromInt(95))},
	}}
	dir := &stubProductDirectory{productErr: errors.New("db down"), skuErr: errors.New("db down")}

	rows := serveMyPrices(t, reader, dir)

	if len(rows) != 1 || rows[0]["product_id"] != float64(18) {
		t.Fatalf("row must survive directory failure: %v", rows)
	}
	if _, ok := rows[0]["product_slug"]; ok {
		t.Fatalf("failed lookup must not carry slug: %v", rows[0])
	}
}
