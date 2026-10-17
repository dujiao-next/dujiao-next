package contract

import (
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

type ListFilter struct {
	IsActive *bool
	Page     int
	PageSize int
}

type LevelRepository interface {
	GetByID(id uint) (*memberleveldomain.MemberLevel, error)
	GetBySlug(slug string) (*memberleveldomain.MemberLevel, error)
	GetDefault() (*memberleveldomain.MemberLevel, error)
	GetActiveBySortOrder(sortOrder int, excludeID uint) (*memberleveldomain.MemberLevel, error)
	ListAllActive() ([]memberleveldomain.MemberLevel, error)
	Create(level *memberleveldomain.MemberLevel) error
	Update(level *memberleveldomain.MemberLevel) error
	Delete(id uint) error
	List(filter ListFilter) ([]memberleveldomain.MemberLevel, int64, error)
	ClearDefault(excludeID uint) error
}

type PriceRepository interface {
	GetByID(id uint) (*memberleveldomain.MemberLevelPrice, error)
	GetByLevelAndProductAndSKU(levelID, productID, skuID uint) (*memberleveldomain.MemberLevelPrice, error)
	ListByProduct(productID uint) ([]memberleveldomain.MemberLevelPrice, error)
	ListByLevelAndProducts(levelID uint, productIDs []uint) ([]memberleveldomain.MemberLevelPrice, error)
	BatchUpsert(prices []memberleveldomain.MemberLevelPrice) error
	Delete(id uint) error
	DeleteByProduct(productID uint) error
}

// UserPriceRepository 持久化用户专属商品价（一口价）。
type UserPriceRepository interface {
	GetByUserAndProductAndSKU(userID, productID, skuID uint) (*memberleveldomain.UserProductPrice, error)
	ListByUser(userID uint, page, pageSize int) ([]memberleveldomain.UserProductPrice, int64, error)
	BatchUpsert(prices []memberleveldomain.UserProductPrice) error
	Delete(id uint) error
}

// UserRepository is the member-level consumer's minimal user persistence port.
type UserRepository interface {
	GetByID(id uint) (*userdomain.User, error)
	Update(user *userdomain.User) error
	IncrementTotalRecharged(userID uint, amount decimal.Decimal) error
	IncrementTotalSpent(userID uint, amount decimal.Decimal) error
	UpdateMemberLevelIfCurrent(userID, currentLevelID, nextLevelID uint) (int64, error)
	AssignDefaultMemberLevel(defaultLevelID uint) (int64, error)
}

// ProductSummary 是专属价展示所需的商品摘要。
type ProductSummary struct {
	Slug     string
	Title    jsonmap.JSON
	IsActive bool
}

// SKUSummary 是专属价展示所需的 SKU 摘要。
type SKUSummary struct {
	SKUCode    string
	SpecValues jsonmap.JSON
}

// ProductDirectory 按 ID 批量解析商品与 SKU 摘要，仅用于用户专属价的前台展示。
// 已删除的商品或 SKU 不会出现在结果中。
type ProductDirectory interface {
	ResolveProducts(ids []uint) (map[uint]ProductSummary, error)
	ResolveSKUs(ids []uint) (map[uint]SKUSummary, error)
}
