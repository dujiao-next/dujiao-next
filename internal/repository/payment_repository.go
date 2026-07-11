package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PaymentRepository 支付数据访问接口
type PaymentRepository interface {
	Create(payment *models.Payment) error
	Update(payment *models.Payment) error
	GetByID(id uint) (*models.Payment, error)
	GetByIDs(ids []uint) ([]models.Payment, error)
	GetByGatewayOrderNo(gatewayOrderNo string) (*models.Payment, error)
	GetLatestByProviderRef(providerRef string) (*models.Payment, error)
	ListByOrderID(orderID uint) ([]models.Payment, error)
	GetLatestPendingByOrder(orderID uint, now time.Time) (*models.Payment, error)
	ExpirePendingByOrderIDs(orderIDs []uint, expiredAt time.Time) (int64, error)
	ListAdmin(filter PaymentListFilter) ([]models.Payment, int64, error)
	Transaction(fn func(tx *gorm.DB) error) error
	GetByIDForUpdate(id uint) (*models.Payment, error)
	WithTx(tx *gorm.DB) *GormPaymentRepository
}

// GormPaymentRepository GORM 实现
type GormPaymentRepository struct {
	BaseRepository
}

// NewPaymentRepository 创建支付仓库
func NewPaymentRepository(db *gorm.DB) *GormPaymentRepository {
	return &GormPaymentRepository{BaseRepository: BaseRepository{db: db}}
}

// WithTx 绑定事务
func (r *GormPaymentRepository) WithTx(tx *gorm.DB) *GormPaymentRepository {
	if tx == nil {
		return r
	}
	return &GormPaymentRepository{BaseRepository: BaseRepository{db: tx}}
}

// Create 创建支付记录
func (r *GormPaymentRepository) Create(payment *models.Payment) error {
	return r.db.Create(payment).Error
}

// Update 更新支付记录
func (r *GormPaymentRepository) Update(payment *models.Payment) error {
	return r.db.Save(payment).Error
}

// GetByID 根据 ID 获取支付记录
func (r *GormPaymentRepository) GetByID(id uint) (*models.Payment, error) {
	var payment models.Payment
	if err := r.db.First(&payment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &payment, nil
}

// GetByIDs 根据 ID 列表获取支付记录
func (r *GormPaymentRepository) GetByIDs(ids []uint) ([]models.Payment, error) {
	if len(ids) == 0 {
		return []models.Payment{}, nil
	}
	var payments []models.Payment
	if err := r.db.Where("id IN ?", ids).Find(&payments).Error; err != nil {
		return nil, err
	}
	return payments, nil
}

// GetByGatewayOrderNo 根据网关订单号获取支付记录
func (r *GormPaymentRepository) GetByGatewayOrderNo(gatewayOrderNo string) (*models.Payment, error) {
	gatewayOrderNo = strings.TrimSpace(gatewayOrderNo)
	if gatewayOrderNo == "" {
		return nil, nil
	}
	var payment models.Payment
	result := r.db.Where("gateway_order_no = ?", gatewayOrderNo).Order("id desc").Limit(1).Find(&payment)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &payment, nil
}

// GetLatestByProviderRef 根据第三方流水号获取最新支付记录
func (r *GormPaymentRepository) GetLatestByProviderRef(providerRef string) (*models.Payment, error) {
	providerRef = strings.TrimSpace(providerRef)
	if providerRef == "" {
		return nil, nil
	}
	var payment models.Payment
	result := r.db.Where("provider_ref = ?", providerRef).Order("id desc").Limit(1).Find(&payment)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &payment, nil
}

// ListByOrderID 获取订单支付记录
func (r *GormPaymentRepository) ListByOrderID(orderID uint) ([]models.Payment, error) {
	var payments []models.Payment
	if err := r.db.Where("order_id = ?", orderID).Order("id desc").Find(&payments).Error; err != nil {
		return nil, err
	}
	return payments, nil
}

// GetLatestPendingByOrder 获取订单最新待支付记录
func (r *GormPaymentRepository) GetLatestPendingByOrder(orderID uint, now time.Time) (*models.Payment, error) {
	var payment models.Payment
	result := r.db.
		Select("payments.*, payment_channels.name AS channel_name").
		Joins("LEFT JOIN payment_channels ON payment_channels.id = payments.channel_id AND payment_channels.deleted_at IS NULL").
		Where("payments.order_id = ? AND payments.status IN ? AND (payments.expired_at IS NULL OR payments.expired_at > ?) AND ((payments.pay_url IS NOT NULL AND payments.pay_url <> '') OR (payments.qr_code IS NOT NULL AND payments.qr_code <> '') OR (LOWER(payments.provider_type) = ? AND LOWER(payments.interaction_mode) = ? AND payments.provider_ref IS NOT NULL AND payments.provider_ref <> ''))",
			orderID,
			[]string{constants.PaymentStatusInitiated, constants.PaymentStatusPending},
			now,
			constants.PaymentProviderBepusdt,
			constants.PaymentInteractionQR,
		).Order("payments.id desc").Limit(1).Find(&payment)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &payment, nil
}

// GetLatestPendingByOrderChannel 获取订单+渠道最新待支付记录
func (r *GormPaymentRepository) GetLatestPendingByOrderChannel(orderID uint, channelID uint, now time.Time) (*models.Payment, error) {
	var payment models.Payment
	result := r.db.Where("order_id = ? AND channel_id = ? AND status IN ? AND (expired_at IS NULL OR expired_at > ?) AND ((pay_url IS NOT NULL AND pay_url <> '') OR (qr_code IS NOT NULL AND qr_code <> '') OR (LOWER(provider_type) = ? AND LOWER(interaction_mode) = ? AND provider_ref IS NOT NULL AND provider_ref <> ''))",
		orderID,
		channelID,
		[]string{constants.PaymentStatusInitiated, constants.PaymentStatusPending},
		now,
		constants.PaymentProviderBepusdt,
		constants.PaymentInteractionQR,
	).Order("id desc").Limit(1).Find(&payment)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &payment, nil
}

// ExpirePendingByOrderIDs 将指定订单的未完成支付记录标记为过期。
func (r *GormPaymentRepository) ExpirePendingByOrderIDs(orderIDs []uint, expiredAt time.Time) (int64, error) {
	if len(orderIDs) == 0 {
		return 0, nil
	}
	result := r.db.Model(&models.Payment{}).
		Where("order_id IN ? AND status IN ?", orderIDs, []string{constants.PaymentStatusInitiated, constants.PaymentStatusPending}).
		Updates(map[string]interface{}{
			"status":     constants.PaymentStatusExpired,
			"expired_at": expiredAt,
			"updated_at": expiredAt,
		})
	return result.RowsAffected, result.Error
}

// ListAdmin 管理端支付列表
func (r *GormPaymentRepository) ListAdmin(filter PaymentListFilter) ([]models.Payment, int64, error) {
	query := r.db.Model(&models.Payment{})

	if filter.UserID != 0 {
		query = query.
			Joins("LEFT JOIN orders ON orders.id = payments.order_id").
			Joins("LEFT JOIN wallet_recharge_orders ON wallet_recharge_orders.payment_id = payments.id").
			Where("(orders.user_id = ? OR wallet_recharge_orders.user_id = ?)", filter.UserID, filter.UserID)
	}
	if filter.OrderID != 0 {
		query = query.Where("payments.order_id = ?", filter.OrderID)
	}
	if filter.ChannelID != 0 {
		query = query.Where("payments.channel_id = ?", filter.ChannelID)
	}
	if filter.ProviderType != "" {
		query = query.Where("payments.provider_type = ?", filter.ProviderType)
	}
	if filter.ChannelType != "" {
		query = query.Where("payments.channel_type = ?", filter.ChannelType)
	}
	if filter.Status != "" {
		query = query.Where("payments.status = ?", filter.Status)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("payments.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("payments.created_at <= ?", *filter.CreatedTo)
	}

	if filter.Lightweight {
		query = query.Select(
			"payments.id",
			"payments.order_id",
			"payments.channel_id",
			"payments.provider_type",
			"payments.channel_type",
			"payments.interaction_mode",
			"payments.amount",
			"payments.fee_rate",
			"payments.fee_amount",
			"payments.currency",
			"payments.status",
			"payments.provider_ref",
			"payments.created_at",
			"payments.updated_at",
			"payments.paid_at",
			"payments.expired_at",
			"payments.callback_at",
		)
	}

	var total int64
	if !filter.SkipCount {
		if err := query.Count(&total).Error; err != nil {
			return nil, 0, err
		}
	}

	query = applyPagination(query, filter.Page, filter.PageSize)

	var payments []models.Payment
	if err := query.Order("payments.id desc").Find(&payments).Error; err != nil {
		return nil, 0, err
	}
	return payments, total, nil
}

// GetByIDForUpdate 事务中加行锁读取支付单,不存在返回 (nil, nil)。
func (r *GormPaymentRepository) GetByIDForUpdate(id uint) (*models.Payment, error) {
	if id == 0 {
		return nil, nil
	}
	var payment models.Payment
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&payment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &payment, nil
}
