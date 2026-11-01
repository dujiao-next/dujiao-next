package contract

import fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"

// Store 是交付记录持久化端口。
type Store interface {
	Create(fulfillment *fulfillmentdomain.Fulfillment) error
	GetByOrderID(orderID uint) (*fulfillmentdomain.Fulfillment, error)
	FindByOrderIDForUpdate(orderID uint) (*fulfillmentdomain.Fulfillment, bool, error)
}

// PaymentDeliveryGuard is implemented by the transaction-bound SQL adapter.
// It returns whether payment's durable outbox owns delivery notifications.
type PaymentDeliveryGuard interface {
	LockPaymentDelivery(rootOrderID uint, required bool) (bool, error)
}
