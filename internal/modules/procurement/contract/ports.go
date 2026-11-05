package contract

import (
	"time"

	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
)

type Repository interface {
	GetByID(id uint) (*procurementdomain.Order, error)
	GetByLocalOrderID(localOrderID uint) (*procurementdomain.Order, error)
	GetByLocalOrderNo(localOrderNo string) (*procurementdomain.Order, error)
	Create(order *procurementdomain.Order) error
	// UpdateStatus 仅当当前状态仍为 fromStatus 时写入（比较后再写），返回是否写入；
	// 回调、轮询、提交可能并发处理同一采购单，未写入说明已被其它路径推进。
	UpdateStatus(id uint, fromStatus, toStatus string, updates map[string]interface{}) (bool, error)
	List(filter ListFilter) ([]procurementdomain.Order, int64, error)
	StatsByStatus(filter ListFilter) (map[string]int64, error)
	ListByConnectionAndTimeRange(connectionID uint, start, end time.Time) ([]procurementdomain.Order, error)
}

type OrderRepository interface {
	GetByID(id uint) (*procurementdomain.LocalOrder, error)
	GetByIDs(ids []uint) ([]procurementdomain.LocalOrder, error)
	UpdateStatus(id uint, status string, updates map[string]interface{}) error
}

type ProductMappingReader interface {
	FindConnectionID(productID uint) (connectionID uint, found bool, err error)
}

type SKUMappingReader interface {
	FindUpstreamSKUID(skuID uint) (upstreamSKUID uint, found bool, err error)
}

type ConnectionProvider interface {
	Open(connectionID uint) (UpstreamConnection, error)
}

type Enqueuer interface {
	EnqueueSubmit(procurementOrderID uint, delay time.Duration) error
	EnqueuePoll(procurementOrderID uint, delay time.Duration) error
}

type OrderLifecycle interface {
	CreateUpstreamFulfillment(orderID uint, fulfillment *Fulfillment, now time.Time) error
	SyncParentStatus(parentID uint, now time.Time) (string, error)
	EnqueueStatusEmail(orderID uint, status string) (skipped bool, err error)
}

type DownstreamCallbackEnqueuer interface {
	EnqueueCallback(orderID uint)
}

type BotFulfillmentNotifier interface {
	NotifyBotOrderFulfilled(userID, orderID uint)
}

type FailureNotifier interface {
	NotifyFailure(order *procurementdomain.Order, message string) error
}
