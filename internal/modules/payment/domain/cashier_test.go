package domain

import (
	"github.com/dujiao-next/internal/constants"
	od "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"testing"
	"time"
)

func splitOrder() od.Order {
	expiry := time.Now().Add(time.Hour)
	o := od.Order{ID: 1, UserID: 7, OrderNo: "root", Currency: "CNY", Status: constants.OrderStatusPendingPayment, ExpiresAt: &expiry, TotalAmount: money.FromDecimal(decimal.NewFromInt(100))}
	for i := uint(2); i <= 3; i++ {
		parent := o.ID
		child := od.Order{ID: i, UserID: 7, OrderNo: "child", ParentID: &parent, Currency: "CNY", Status: o.Status, ExpiresAt: &expiry, TotalAmount: money.FromDecimal(decimal.NewFromInt(50))}
		child.Items = []od.OrderItem{{ID: i, OrderID: i, ProductID: 1, SKUID: 1, Quantity: 1, FulfillmentType: constants.FulfillmentTypeAuto}}
		o.Children = append(o.Children, child)
	}
	return o
}

func TestFixedCashierRequiresConsistentPersistedObligations(t *testing.T) {
	valid := splitOrder()
	if err := ValidateFixedOrder(&valid); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*od.Order){
		"guest":            func(o *od.Order) { o.UserID = 0 },
		"empty_leaf":       func(o *od.Order) { o.Children[0].Items = nil },
		"mixed_root_items": func(o *od.Order) { o.Items = o.Children[0].Items },
		"wrong_parent":     func(o *od.Order) { id := uint(9); o.Children[0].ParentID = &id },
		"wrong_owner":      func(o *od.Order) { o.Children[0].UserID = 9 },
		"wrong_tenant":     func(o *od.Order) { id := uint(9); o.Children[0].ResellerID = &id },
		"wrong_currency":   func(o *od.Order) { o.Children[0].Currency = "USD" },
		"wrong_total":      func(o *od.Order) { o.Children[0].TotalAmount = money.FromDecimal(decimal.NewFromInt(49)) },
		"duplicate_order":  func(o *od.Order) { o.Children[1].ID = o.Children[0].ID },
		"nested":           func(o *od.Order) { o.Children[0].Children = []od.Order{{ID: 4}} },
		"foreign_item":     func(o *od.Order) { o.Children[0].Items[0].OrderID = 9 },
		"duplicate_item":   func(o *od.Order) { o.Children[1].Items[0].ID = o.Children[0].Items[0].ID },
		"upstream":         func(o *od.Order) { o.Children[0].Items[0].FulfillmentType = "upstream" },
		"zero_quantity":    func(o *od.Order) { o.Children[0].Items[0].Quantity = 0 },
		"no_sku":           func(o *od.Order) { o.Children[0].Items[0].SKUID = 0 },
		"paid_child":       func(o *od.Order) { o.Children[0].Status = constants.OrderStatusPaid },
		"expired_child":    func(o *od.Order) { past := time.Now().Add(-time.Second); o.Children[0].ExpiresAt = &past },
		"missing_expiry":   func(o *od.Order) { o.ExpiresAt = nil },
		"refund":           func(o *od.Order) { o.Children[0].RefundedAmount = money.FromDecimal(decimal.NewFromInt(1)) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			o := splitOrder()
			change(&o)
			if ValidateFixedOrder(&o) == nil {
				t.Fatal("ineligible order accepted")
			}
		})
	}
}
