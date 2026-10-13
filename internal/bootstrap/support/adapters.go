package support

import (
	"context"
	"github.com/dujiao-next/internal/authz"
	admincontract "github.com/dujiao-next/internal/modules/identity/admin/contract"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	supportapp "github.com/dujiao-next/internal/modules/support/application"
	"github.com/dujiao-next/internal/modules/support/contract"
)

type Orders struct{ Service *orderapp.OrderService }

func (o Orders) Verify(ctx context.Context, a contract.Actor, no, email, password string) (*contract.OrderSummary, error) {
	tenant, ok := resellercontract.TenantFromContext(ctx)
	if !ok || tenant.Unavailable {
		return nil, contract.ErrForbidden
	}
	var order *orderdomain.Order
	var err error
	if a.UserID > 0 {
		order, err = o.Service.GetOrderByUserOrderNoForTenant(tenant, no, a.UserID)
	} else {
		if email == "" || password == "" {
			return nil, contract.ErrForbidden
		}
		order, err = o.Service.GetOrderByGuestOrderNoForTenant(tenant, no, email, password)
	}
	if err != nil || order == nil {
		return nil, contract.ErrForbidden
	}
	s := &contract.OrderSummary{OrderNo: order.OrderNo, Status: order.Status, TotalAmount: order.TotalAmount.Decimal.StringFixed(2), Currency: order.Currency, Items: []contract.OrderItem{}}
	for _, item := range order.Items {
		name := ""
		for _, lang := range []string{"zh-CN", "zh-cn", "zh", "en-US", "en"} {
			if v, ok := item.TitleJSON[lang].(string); ok && v != "" {
				name = v
				break
			}
		}
		if name == "" {
			name = "Product"
		}
		s.Items = append(s.Items, contract.OrderItem{Name: name, Quantity: item.Quantity})
	}
	return s, nil
}

type Agents struct {
	Store admincontract.Store
	Authz *authz.Service
}

func (a Agents) Eligible(ctx context.Context, id uint) bool {
	v, err := a.Store.GetByID(id)
	if err != nil || v == nil || v.DeletedAt != nil {
		return false
	}
	if v.IsSuper {
		return true
	}
	ok, err := a.Authz.EnforceAdmin(id, "/admin/support/conversations", "GET")
	return err == nil && ok
}
func (a Agents) List(ctx context.Context) ([]contract.Agent, error) {
	admins, err := a.Store.List()
	if err != nil {
		return nil, err
	}
	out := []contract.Agent{}
	for _, v := range admins {
		if a.Eligible(ctx, v.ID) {
			out = append(out, contract.Agent{ID: v.ID, Username: v.Username})
		}
	}
	return out, nil
}
func New(store contract.Store, orders *orderapp.OrderService, admins admincontract.Store, az *authz.Service) *supportapp.Service {
	return supportapp.New(store, Orders{orders}, Agents{admins, az})
}
