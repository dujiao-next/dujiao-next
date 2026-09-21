package transport_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderhttp "github.com/dujiao-next/internal/modules/order/transport/http"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/gin-gonic/gin"
)

type browserOrderQueryStub struct{ hash string }

func (s *browserOrderQueryStub) ListOrdersByGuestForTenant(reseller.TenantContext, string, string, int, int) ([]orderdomain.Order, int64, error) {
	return nil, 0, nil
}
func (s *browserOrderQueryStub) GetOrderByGuestOrderNoForTenant(reseller.TenantContext, string, string, string) (*orderdomain.Order, error) {
	return nil, nil
}
func (s *browserOrderQueryStub) GetAnyOrderByGuestOrderNoForTenant(reseller.TenantContext, string, string, string) (*orderdomain.Order, error) {
	return nil, nil
}
func (s *browserOrderQueryStub) CancelGuestOrder(order *orderdomain.Order) (*orderdomain.Order, error) {
	return order, nil
}
func (s *browserOrderQueryStub) ListOrdersByBrowserTokenForTenant(_ reseller.TenantContext, hash string, _, _ int) ([]orderdomain.Order, int64, error) {
	s.hash = hash
	return nil, 0, nil
}

func TestBrowserOrdersWithoutCookieReturnsEmptySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &browserOrderQueryStub{}
	r := gin.New()
	orderhttp.RegisterGuestReadRoutes(r, orderhttp.NewGuestHandler(stub, nil, nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/orders/browser", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
	}
}

func TestBrowserOrdersHashesValidCookieBeforeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &browserOrderQueryStub{}
	r := gin.New()
	orderhttp.RegisterGuestReadRoutes(r, orderhttp.NewGuestHandler(stub, nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/orders/browser", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-dujiao_browser_orders", Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || len(stub.hash) != 64 {
		t.Fatalf("cookie was not hashed: %d %q %s", w.Code, stub.hash, w.Body.String())
	}
}
