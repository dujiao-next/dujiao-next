package contract

import (
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/modules/affiliate/domain"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/shopspring/decimal"
)

// OrderReader 是 Affiliate 计算订单佣金所需的最小订单读取端口。
type OrderReader interface {
	GetByID(id uint) (*orderdomain.Order, error)
}

// ProductReader 是 Affiliate 计算可返利金额所需的最小商品读取端口。
type ProductReader interface {
	ListByIDs(ids []uint) ([]productdomain.Product, error)
}

// SKUReader 是 Affiliate 解析 SKU 级返利比例所需的最小规格读取端口。
type SKUReader interface {
	ListByIDs(ids []uint) ([]productdomain.ProductSKU, error)
	ListByProduct(productID uint, onlyActive bool) ([]productdomain.ProductSKU, error)
}

// SettingsReader 是 Affiliate 所需的动态设置读取端口。
type SettingsReader interface {
	GetAffiliateSetting() (settingsintegration.AffiliateSetting, error)
}

type ProfileListFilter struct {
	Page     int
	PageSize int
	UserID   uint
	Status   string
	Code     string
	Keyword  string
}

type CommissionListFilter struct {
	Page               int
	PageSize           int
	AffiliateProfileID uint
	OrderID            uint
	OrderNo            string
	Status             string
	Keyword            string
	CreatedFrom        *time.Time
	CreatedTo          *time.Time
}

type WithdrawListFilter struct {
	Page               int
	PageSize           int
	AffiliateProfileID uint
	Status             string
	Keyword            string
	CreatedFrom        *time.Time
	CreatedTo          *time.Time
}

type ProfileStatsAggregate struct {
	ClickCount          int64
	ValidOrderCount     int64
	PendingCommission   decimal.Decimal
	AvailableCommission decimal.Decimal
	WithdrawnCommission decimal.Decimal
}

type Store interface {
	WithinTransaction(fn func(Store) error) error

	GetProfileByID(id uint) (*domain.Profile, error)
	UpdateProfileStatus(id uint, status string, updatedAt time.Time) error
	BatchUpdateProfileStatus(ids []uint, status string, updatedAt time.Time) (int64, error)
	GetProfileByUserID(userID uint) (*domain.Profile, error)
	GetProfileByCode(code string) (*domain.Profile, error)
	CreateProfile(profile *domain.Profile) error
	ListProfiles(filter ProfileListFilter) ([]domain.Profile, int64, error)

	CreateClick(click *domain.Click) error
	HasRecentClick(profileID uint, visitorKey, landingPath string, since time.Time) (bool, error)
	GetLatestActiveProfileByVisitorKey(visitorKey string, since time.Time) (*domain.Profile, error)
	CountClicksByProfile(profileID uint) (int64, error)

	GetCommissionByOrderAndProfile(orderID, profileID uint, commissionType string) (*domain.Commission, error)
	CreateCommission(commission *domain.Commission) error
	UpdateCommission(commission *domain.Commission) error
	ListCommissions(filter CommissionListFilter) ([]domain.Commission, int64, error)
	ListCommissionsByOrder(orderID uint, statuses []string) ([]domain.Commission, error)
	ListCommissionsByOrderForUpdate(orderID uint, statuses []string) ([]domain.Commission, error)
	ListCommissionsByWithdrawIDForUpdate(withdrawID uint) ([]domain.Commission, error)
	MarkPendingCommissionsAvailable(before, now time.Time) (int64, error)
	CountValidOrdersByProfile(profileID uint) (int64, error)
	SumCommissionByProfile(profileID uint, statuses []string, unboundOnly bool) (decimal.Decimal, error)
	ListAvailableCommissionsForUpdate(profileID uint) ([]domain.Commission, error)
	BatchUpdateCommissions(ids []uint, updates map[string]interface{}) error

	CreateWithdraw(request *domain.WithdrawRequest) error
	UpdateWithdraw(request *domain.WithdrawRequest) error
	GetWithdrawByID(id uint) (*domain.WithdrawRequest, error)
	GetWithdrawByIDForUpdate(id uint) (*domain.WithdrawRequest, error)
	ListWithdraws(filter WithdrawListFilter) ([]domain.WithdrawRequest, int64, error)
	GetProfileStatsBatch(profileIDs []uint) (map[uint]ProfileStatsAggregate, error)
}
