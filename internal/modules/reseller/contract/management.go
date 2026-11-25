package contract

import (
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// ProfileListFilter 管理端分销商资料过滤条件。
type ProfileListFilter struct {
	Page             int
	PageSize         int
	UserID           uint
	Status           string
	SettlementStatus string
	Keyword          string
	CreatedFrom      *time.Time
	CreatedTo        *time.Time
}

// DomainListFilter 管理端分销商域名过滤条件。
type DomainListFilter struct {
	Page               int
	PageSize           int
	ResellerID         uint
	UserID             uint
	Domain             string
	Type               string
	Status             string
	VerificationStatus string
	Keyword            string
	CreatedFrom        *time.Time
	CreatedTo          *time.Time
}

// SiteConfigListFilter 分销站点配置列表过滤条件。
type SiteConfigListFilter struct {
	Page        int
	PageSize    int
	ResellerID  uint
	Keyword     string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

// DomainLookupRepository 是租户解析所需的最小域名查询端口。
type DomainLookupRepository interface {
	FindActiveVerifiedDomain(host string) (*resellerdomain.Domain, error)
}

// SiteConfigRepository 是站点配置用例所需的最小持久化端口。
type SiteConfigRepository interface {
	GetProfileByUserID(userID uint) (*resellerdomain.Profile, error)
	GetProfileByID(id uint) (*resellerdomain.Profile, error)
	UpsertSiteConfig(config resellerdomain.SiteConfig) (*resellerdomain.SiteConfig, error)
	GetSiteConfigByResellerID(resellerID uint) (*resellerdomain.SiteConfig, error)
	DeleteSiteConfigByResellerID(resellerID uint) error
}

// ManagementStore 是入驻/审批/域名管理用例所需的持久化与事务端口。
// WithinTransaction 不得向用例暴露具体数据库连接类型。
type ManagementStore interface {
	WithinManagementTransaction(fn func(store ManagementStore) error) error
	GetProfileByUserID(userID uint) (*resellerdomain.Profile, error)
	GetProfileByID(id uint) (*resellerdomain.Profile, error)
	CreateProfile(profile *resellerdomain.Profile) error
	UpdateProfile(profile *resellerdomain.Profile) error
	UpdateActiveProfileDefaultMarkup(userID uint, markup money.Amount) (bool, error)
	ListDomainsByResellerID(resellerID uint) ([]resellerdomain.Domain, error)
	UpsertDomain(domain resellerdomain.Domain) (*resellerdomain.Domain, error)
	GetDomainByID(id uint) (*resellerdomain.Domain, error)
	GetDomainByIDForUpdate(id uint) (*resellerdomain.Domain, error)
	UpdateDomain(domain *resellerdomain.Domain) error
	FindDomainByHost(host string) (*resellerdomain.Domain, error)
}

// ProductSettingListFilter 用户侧分销商品配置列表过滤条件。
type ProductSettingListFilter struct {
	Page       int
	PageSize   int
	ResellerID uint
	CategoryID uint
	Keyword    string
	Configured string
	Listed     string
	OnlyActive bool
}

// ProductSettingAdminListFilter 后台分销商品配置列表过滤条件。
type ProductSettingAdminListFilter struct {
	Page        int
	PageSize    int
	ResellerID  uint
	UserID      uint
	ProductID   uint
	Keyword     string
	PricingMode string
	Configured  string
	Listed      string
}

// ProductSettingProductRow 商品及其分销配置。
type ProductSettingProductRow struct {
	Product  productdomain.Product
	Settings []resellerdomain.ProductSetting
}

// ProductSettingSummary 分销商品配置汇总。
type ProductSettingSummary struct {
	ConfiguredProducts int64
	HiddenProducts     int64
	SKUOverrides       int64
	PricingOverrides   int64
}

// ProductSettingStore 是分销商品配置用例所需的持久化与事务端口。
// WithinTransaction 不得向用例暴露具体数据库连接类型。
type ProductSettingStore interface {
	WithinProductSettingTransaction(fn func(store ProductSettingStore) error) error
	GetProfileByUserID(userID uint) (*resellerdomain.Profile, error)
	GetProfileByID(id uint) (*resellerdomain.Profile, error)
	ListProductsWithSettings(filter ProductSettingListFilter) ([]ProductSettingProductRow, int64, error)
	GetProductWithSettings(resellerID, productID uint) (*ProductSettingProductRow, error)
	UpsertSetting(setting resellerdomain.ProductSetting) (*resellerdomain.ProductSetting, error)
	DeleteSetting(resellerID, productID, skuID uint) error
	ListAdminSettings(filter ProductSettingAdminListFilter) ([]resellerdomain.ProductSetting, int64, error)
	SummarizeByResellerID(resellerID uint) (ProductSettingSummary, error)
}
