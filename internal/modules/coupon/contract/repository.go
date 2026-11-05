package contract

import (
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	"github.com/dujiao-next/internal/shared/money"
)

type ListFilter struct {
	ID         uint
	Code       string
	ScopeRefID uint
	IsActive   *bool
	Page       int
	PageSize   int
}

type UsageListFilter struct {
	UserID   uint
	Page     int
	PageSize int
}

// EligibilityItem 是 Coupon 计算所需的订单项只读快照，避免优惠券域依赖订单持久化模型。
type EligibilityItem struct {
	ProductID         uint
	SKUID             uint
	Quantity          int
	TotalPrice        money.Amount
	WholesaleDiscount money.Amount
}

type Repository interface {
	GetByID(id uint) (*coupondomain.Coupon, error)
	GetByIDForUpdate(id uint) (*coupondomain.Coupon, error)
	GetByCode(code string) (*coupondomain.Coupon, error)
	ListByIDs(ids []uint) ([]coupondomain.Coupon, error)
	Create(coupon *coupondomain.Coupon) error
	Update(coupon *coupondomain.Coupon) error
	Delete(id uint) error
	List(filter ListFilter) ([]coupondomain.Coupon, int64, error)
	IncrementUsedCount(id uint, delta int) error
	DecrementUsedCount(id uint, delta int) error
}

type UsageRepository interface {
	Create(usage *coupondomain.CouponUsage) error
	CountByUser(couponID, userID uint) (int64, error)
	ListByOrderID(orderID uint) ([]coupondomain.CouponUsage, error)
	ListByUser(filter UsageListFilter) ([]coupondomain.CouponUsage, int64, error)
	DeleteByOrderID(orderID uint) error
}

// SKUDirectory 解析规格所属商品，用于校验优惠券规格范围。
type SKUDirectory interface {
	// ResolveSKUProducts 按规格 ID 返回其所属商品 ID；已删除或不存在的规格不在结果中。
	ResolveSKUProducts(ids []uint) (map[uint]uint, error)
}
