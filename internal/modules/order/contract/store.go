package contract

import (
	"time"

	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	couponcontract "github.com/dujiao-next/internal/modules/coupon/contract"
	fulfillmentcontract "github.com/dujiao-next/internal/modules/fulfillment/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"github.com/dujiao-next/internal/shared/money"
)

// Store 是订单应用层所需的完整持久化端口。
// 事务通过 WithinTransaction 暴露模块端口，不向应用层泄漏 ORM 连接。
type Store interface {
	Create(order *orderdomain.Order, items []orderdomain.OrderItem) error
	GetByID(id uint) (*orderdomain.Order, error)
	GetByIDs(ids []uint) ([]orderdomain.Order, error)
	ResolveReceiverEmailByOrderID(orderID uint) (string, error)
	GetByIDAndUser(id uint, userID uint) (*orderdomain.Order, error)
	GetByOrderNoAndUser(orderNo string, userID uint) (*orderdomain.Order, error)
	GetAnyByOrderNoAndUser(orderNo string, userID uint) (*orderdomain.Order, error)
	GetByIDAndGuest(id uint, email, password string) (*orderdomain.Order, error)
	GetByOrderNoAndGuest(orderNo, email, password string) (*orderdomain.Order, error)
	GetAnyByOrderNoAndGuest(orderNo, email, password string) (*orderdomain.Order, error)
	GetByIDAndUserScoped(id uint, userID uint, scope TenantScope) (*orderdomain.Order, error)
	GetByOrderNoAndUserScoped(orderNo string, userID uint, scope TenantScope) (*orderdomain.Order, error)
	GetAnyByOrderNoAndUserScoped(orderNo string, userID uint, scope TenantScope) (*orderdomain.Order, error)
	GetByIDAndGuestScoped(id uint, email, password string, scope TenantScope) (*orderdomain.Order, error)
	GetByOrderNoAndGuestScoped(orderNo, email, password string, scope TenantScope) (*orderdomain.Order, error)
	GetAnyByOrderNoAndGuestScoped(orderNo, email, password string, scope TenantScope) (*orderdomain.Order, error)
	ListChildren(parentID uint) ([]orderdomain.Order, error)
	ListByUser(filter ListFilter) ([]orderdomain.Order, int64, error)
	StatsByUser(filter ListFilter) (map[string]int64, error)
	ListByUserScoped(filter ListFilter, scope TenantScope) ([]orderdomain.Order, int64, error)
	StatsByUserScoped(filter ListFilter, scope TenantScope) (map[string]int64, error)
	ListByGuest(email, password string, page, pageSize int) ([]orderdomain.Order, int64, error)
	ListByGuestScoped(email, password string, page, pageSize int, scope TenantScope) ([]orderdomain.Order, int64, error)
	ListByBrowserTokenScoped(tokenHash string, page, pageSize int, scope TenantScope) ([]orderdomain.Order, int64, error)
	ListAdmin(filter ListFilter) ([]orderdomain.Order, int64, error)
	UpdateStatus(id uint, status string, updates map[string]interface{}) error
	CountOrderItemsByProduct(productID uint) (int64, error)
	CountPendingByUserID(userID uint) (int64, error)
	LockRiskKeys(keys []string) error
	CountPendingGuestByRiskIP(riskIP string) (int64, error)
	CountPendingMemberByRiskIP(riskIP string) (int64, error)
	SumPendingGuestQuantityByRiskIP(riskIP string, productIDs []uint) (map[uint]int64, error)
	UpdateFields(id uint, updates map[string]interface{}) error
	UpdateChildrenStatus(parentID uint, targetStatus string, now time.Time) (int64, error)
	UpdateFieldsWhereWalletPaid(id uint, updates map[string]interface{}) (int64, error)
	GetByIDForUpdate(id uint) (*orderdomain.Order, error)
	GetByIDForUpdateWithChildren(id uint) (*orderdomain.Order, error)

	CreateRefundRecord(record *orderdomain.OrderRefundRecord) error
	GetRefundRecordByID(id uint) (*orderdomain.OrderRefundRecord, error)
	GetRefundRecordByIDForUpdate(id uint) (*orderdomain.OrderRefundRecord, error)
	UpdateRefundRecordPaymentFee(id uint, refunded bool, amount money.Amount, updatedAt time.Time) error
	ListRefundRecordsByOrderIDs(orderIDs []uint) ([]orderdomain.OrderRefundRecord, error)
	ListRefundRecordsAdmin(filter RefundRecordListFilter) ([]orderdomain.OrderRefundRecord, int64, error)

	WithinTransaction(fn func(Transaction) error) error
}

// ResellerOrderStore 是创建订单时写入分销快照所需的最小端口。
type ResellerOrderStore interface {
	CreateOrderSnapshot(snapshot *resellerdomain.OrderSnapshot) error
}

// Transaction 是已打开事务的订单工作单元。
// 每个方法只暴露相应领域端口，保证事务原子性同时隔离 GORM。
type Transaction interface {
	Orders() Store
	Products() productcontract.Repository
	ProductSKUs() productcontract.SKURepository
	CardSecrets() cardsecretcontract.Repository
	Coupons() couponcontract.Repository
	CouponUsages() couponcontract.UsageRepository
	Fulfillments() fulfillmentcontract.Store
	Wallets() walletcontract.Transaction
	Affiliates() affiliatecontract.Store
	ResellerOrders() ResellerOrderStore
	ResellerAccounting() resellercontract.AccountingLedgerStore
	ExpirePendingPaymentsByOrderIDs(orderIDs []uint, expiredAt time.Time) (int64, error)
}
