package transport_test

import (
	"os"
	"strings"
	"testing"
)

func TestGuestPendingOrdersExposeCredentialProtectedCancelRoute(t *testing.T) {
	routes, err := os.ReadFile("../../transport/http/routes.go")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := os.ReadFile("../../transport/http/guest_handler.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(routes), `guest.POST("/orders/:order_no/cancel", handler.CancelGuestOrder)`) {
		t.Fatal("guest cancel route is missing")
	}
	for _, expected := range []string{"func (h *GuestHandler) CancelGuestOrder", "h.orders.GetOrderByGuestOrderNoForTenant", "h.orders.CancelGuestOrder"} {
		if !strings.Contains(string(handler), expected) {
			t.Fatalf("guest cancel handler is missing %q", expected)
		}
	}
}
