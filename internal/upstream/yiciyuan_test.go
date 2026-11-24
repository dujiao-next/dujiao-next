package upstream

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestYiciyuanSignaturePreservesPHPArrayAndEmptyRules(t *testing.T) {
	values := url.Values{"app_id": {"123"}, "code": {"卡 密+&"}, "num": {"0"}, "empty": {""}, "sku[颜色]": {"黑 色"}, "sign": {"old"}}
	raw := "app_id=123&code=卡 密+&&num=0&sku[颜色]=黑 色&key=private key"
	sum := md5.Sum([]byte(raw))
	if got := signYiciyuan(values, "private+key"); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("PHP signature mismatch: %s", got)
	}
}

func TestYiciyuanSignatureSortsTopLevelArrayKey(t *testing.T) {
	values := url.Values{"sku[颜色]": {"黑"}, "skuZ": {"widget"}}
	sum := md5.Sum([]byte("sku[颜色]=黑&skuZ=widget&key=secret"))
	if got := signYiciyuan(values, "secret"); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("top-level PHP key order mismatch: %s", got)
	}
}

func TestYiciyuanPendingManualSecretIsNotDelivery(t *testing.T) {
	if got := ycyFulfillment(map[string]any{"secret": "客服将在一个工作日内处理"}, false); got != nil {
		t.Fatal("unconfirmed manual text must not complete an order")
	}
	if got := ycyFulfillment(map[string]any{"secret": "CARD", "delivery_status": "0"}, true); got != nil {
		t.Fatal("explicit pending status must override secret")
	}
}

func TestYiciyuanCatalogAndPurchase(t *testing.T) {
	trades := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("app_id") != "42" || r.Form.Get("sign") != signYiciyuan(r.PostForm, "secret") {
			t.Errorf("invalid signature")
		}
		if r.Form.Get("app_key") != "" {
			t.Error("secret must never be sent")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/shared/commodity/queryByRequest":
			fmt.Fprint(w, `{"code":200,"data":{"found":false}}`)
		case "/shared/authentication/connect":
			fmt.Fprint(w, `{"code":200,"data":[]}`)
		case "/shared/commodity/items":
			fmt.Fprint(w, `{"code":200,"data":[{"id":9,"name":"Tools","children":[{"id":7,"name":"Card","code":"ABC7","user_price":"10.00","stock":5,"delivery_way":0}]}]}`)
		case "/shared/commodity/item":
			if r.Form.Get("code") != "ABC7" {
				t.Error("missing product code")
			}
			fmt.Fprint(w, `{"code":200,"data":{"id":7,"name":"Card","code":"ABC7","description":"Description","user_price":"10.00","delivery_way":0,"minimum":1,"config":{"category":{"月卡":10,"年卡":20},"sku":{"线路.普通":0,"线路.高速":2}}}}`)
		case "/shared/commodity/stock":
			fmt.Fprint(w, `{"code":200,"data":{"stock":"5"}}`)
		case "/shared/commodity/valuation":
			amount := 10
			if r.Form.Get("race") == "年卡" {
				amount = 20
			}
			if r.Form.Get("sku[线路]") == "高速" {
				amount += 2
			}
			fmt.Fprintf(w, `{"code":200,"data":{"price":"%d.00"}}`, amount)
		case "/shared/commodity/inventoryState":
			if r.Form.Get("shared_code") != "ABC7" {
				t.Error("wrong inventory code field")
			}
			fmt.Fprint(w, `{"code":200,"data":[]}`)
		case "/shared/commodity/trade":
			trades++
			if r.Form.Get("request_no") != "LOCAL1" || r.Form.Get("shared_code") != "ABC7" || r.Form.Get("sku[线路]") != "普通" {
				t.Error("purchase reference lost")
			}
			fmt.Fprint(w, `{"code":200,"data":{"tradeNo":"ORDER-9007199254740993","amount":"20.00","secret":"CARD-1\nCARD-2"}}`)
		case "/shared/commodity/query":
			if r.Form.Get("tradeNo") != "ORDER-9007199254740993" {
				t.Error("opaque order reference lost")
			}
			fmt.Fprint(w, `{"code":200,"data":{"status":1,"delivery_status":1,"secret":"CARD-1\nCARD-2"}}`)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	a := NewYiciyuanAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, ApiKey: "42", ApiSecret: "secret"}, t.TempDir())
	if _, err := a.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	list, err := a.ListProducts(context.Background(), ListProductsOpts{Page: 1, PageSize: 20})
	if err != nil || list.Total != 1 {
		t.Fatalf("catalog: %#v %v", list, err)
	}
	product, err := a.GetProduct(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(product.SKUs) != 4 || product.CategoryID != 9 {
		t.Fatalf("variants/category: %#v", product)
	}
	var chosen UpstreamSKU
	for _, sku := range product.SKUs {
		if sku.Metadata["race"] == "年卡" && sku.SpecValues["线路"] == "普通" {
			chosen = sku
		}
	}
	if chosen.ID == 0 || chosen.PriceAmount != "20.00" {
		t.Fatalf("wrong variant %#v", chosen)
	}
	result, err := a.CreateOrder(context.Background(), CreateUpstreamOrderReq{SKUID: chosen.ID, Metadata: chosen.Metadata, Quantity: 1, DownstreamOrderNo: "LOCAL1"})
	if err != nil || !result.OK || result.OrderNo != "ORDER-9007199254740993" || result.Fulfillment == nil {
		t.Fatalf("purchase: %#v %v", result, err)
	}
	detail, err := a.GetOrderByReference(context.Background(), result.OrderNo)
	if err != nil || detail.Fulfillment == nil || detail.Fulfillment.Payload != "CARD-1\nCARD-2" {
		t.Fatalf("query: %#v %v", detail, err)
	}
	if trades != 1 {
		t.Fatalf("unexpected trade count %d", trades)
	}
}

func TestYiciyuanRejectsRedirectAndMissingPurchaseReference(t *testing.T) {
	leaked := false
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer foreign.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	a := NewYiciyuanAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, ApiKey: "42", ApiSecret: "secret"}, t.TempDir())
	if _, err := a.Ping(context.Background()); err == nil || leaked {
		t.Fatal("authenticated requests must not follow redirects")
	}
	for _, req := range []CreateUpstreamOrderReq{{Quantity: 1, DownstreamOrderNo: "x"}, {Quantity: 1, Metadata: jsonmap.JSON{"code": "x"}}} {
		if _, err := a.CreateOrder(context.Background(), req); err == nil {
			t.Fatal("incomplete reference must not reach trade")
		}
	}
}

func TestYiciyuanBusinessErrorDoesNotExposeSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "private-secret"})
	}))
	defer server.Close()
	a := NewYiciyuanAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, ApiKey: "42", ApiSecret: "private-secret"}, t.TempDir())
	_, err := a.Ping(context.Background())
	if err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatalf("sensitive error: %v", err)
	}
}

func TestYiciyuanReferencePriceAndLostTradeResponse(t *testing.T) {
	trades := 0
	valuations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shared/commodity/valuation":
			valuations++
			fmt.Fprint(w, `{"code":200,"data":{"price":"10.00"}}`)
		case "/shared/commodity/inventoryState":
			fmt.Fprint(w, `{"code":200,"data":[]}`)
		case "/shared/commodity/trade":
			trades++
			w.WriteHeader(http.StatusBadGateway)
		case "/shared/commodity/queryByRequest":
			_ = r.ParseForm()
			if r.Form.Get("request_no") != "LOCAL3" {
				t.Error("recovery must use the same request number")
			}
			if trades == 0 {
				fmt.Fprint(w, `{"code":200,"data":{"found":false}}`)
				return
			}
			fmt.Fprint(w, `{"code":200,"data":{"found":true,"trade_no":"EXISTING123","status":1,"delivery_status":1,"secret":"CARD"}}`)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
	}))
	defer server.Close()
	a := NewYiciyuanAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, ApiKey: "42", ApiSecret: "secret"}, t.TempDir())
	result, err := a.CreateOrder(context.Background(), CreateUpstreamOrderReq{Quantity: 3, DownstreamOrderNo: "LOCAL3", Metadata: jsonmap.JSON{"code": "ABC", "unit_price": "3.33", "reference_amount": "10.00", "reference_quantity": 3}})
	if err != nil || result == nil || result.OK || !strings.Contains(result.ErrorMessage, "EXISTING123") || !strings.Contains(result.ErrorMessage, "do not repurchase") {
		t.Fatalf("ambiguous debit must retain a reference for manual resolution without inventing cost: %#v %v", result, err)
	}
	if trades != 1 {
		t.Fatalf("rounding must not reject unchanged price or repeat trade: %d", trades)
	}
	result, err = a.CreateOrder(context.Background(), CreateUpstreamOrderReq{Quantity: 3, DownstreamOrderNo: "LOCAL3", Metadata: jsonmap.JSON{"code": "ABC", "unit_price": "3.33"}})
	if err != nil || result.OK || !strings.Contains(result.ErrorMessage, "EXISTING123") || trades != 1 || valuations != 1 {
		t.Fatalf("retry must recover before changed price or depleted stock: %#v %v, trades=%d valuations=%d", result, err, trades, valuations)
	}
}
