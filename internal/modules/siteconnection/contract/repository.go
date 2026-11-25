package contract

import siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

// ListFilter 是连接列表查询条件。
type ListFilter struct {
	Status   string
	Page     int
	PageSize int
}

// Repository 是连接持久化端口。
type Repository interface {
	GetByID(id uint) (*siteconnectiondomain.Connection, error)
	GetByApiKey(apiKey string) (*siteconnectiondomain.Connection, error)
	Create(conn *siteconnectiondomain.Connection) error
	Update(conn *siteconnectiondomain.Connection) error
	Delete(id uint) error
	List(filter ListFilter) ([]siteconnectiondomain.Connection, int64, error)
	ListActive() ([]siteconnectiondomain.Connection, error)
}

type BalanceRepository interface {
	ListBalanceSnapshots(ids []uint) ([]siteconnectiondomain.BalanceSnapshot, error)
	SaveBalanceCheck(snapshot *siteconnectiondomain.BalanceSnapshot) error
	SaveBalanceThreshold(connectionID uint, amount, currency string) error
}
