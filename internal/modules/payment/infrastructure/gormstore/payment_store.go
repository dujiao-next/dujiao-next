package gormstore

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/persistence/gormutil"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store 是支付记录的 GORM 实现。
type Store struct {
	db                    *gorm.DB
	guestCredentialSecret string
}

// New 创建支付 Store。
func New(db *gorm.DB, guestCredentialSecret string) *Store {
	secret := strings.TrimSpace(guestCredentialSecret)
	if secret == "" {
		panic("payment store: guest credential secret is required")
	}
	return &Store{db: db, guestCredentialSecret: secret}
}

// Create 创建支付记录
func (r *Store) Create(payment *paymentdomain.Payment) error {
	return r.db.Create(payment).Error
}

// Update 更新支付记录
func (r *Store) Update(payment *paymentdomain.Payment) error {
	return r.db.Save(payment).Error
}

// GetByID 根据 ID 获取支付记录
func (r *Store) GetByID(id uint) (*paymentdomain.Payment, error) {
	var payment paymentdomain.Payment
	if err := r.db.Where("deleted_at IS NULL").First(&payment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &payment, nil
}

// GetByIDs 根据 ID 列表获取支付记录
func (r *Store) GetByIDs(ids []uint) ([]paymentdomain.Payment, error) {
	if len(ids) == 0 {
		return []paymentdomain.Payment{}, nil
	}
	var payments []paymentdomain.Payment
	if err := r.db.Where("id IN ? AND deleted_at IS NULL", ids).Find(&payments).Error; err != nil {
		return nil, err
	}
	return payments, nil
}

// GetByGatewayOrderNo 根据网关订单号获取支付记录
func (r *Store) GetByGatewayOrderNo(gatewayOrderNo string) (*paymentdomain.Payment, error) {
	gatewayOrderNo = strings.TrimSpace(gatewayOrderNo)
	if gatewayOrderNo == "" {
		return nil, nil
	}
	var payment paymentdomain.Payment
	result := r.db.Where("gateway_order_no = ? AND deleted_at IS NULL", gatewayOrderNo).Order("id desc").Limit(1).Find(&payment)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &payment, nil
}

// GetLatestByProviderRef 根据第三方流水号获取最新支付记录
func (r *Store) GetLatestByProviderRef(providerRef string) (*paymentdomain.Payment, error) {
	providerRef = strings.TrimSpace(providerRef)
	if providerRef == "" {
		return nil, nil
	}
	var payment paymentdomain.Payment
	result := r.db.Where("provider_ref = ? AND deleted_at IS NULL", providerRef).Order("id desc").Limit(1).Find(&payment)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &payment, nil
}

// ListByOrderID 获取订单支付记录
func (r *Store) ListByOrderID(orderID uint) ([]paymentdomain.Payment, error) {
	var payments []paymentdomain.Payment
	if err := r.db.Where("order_id = ? AND deleted_at IS NULL", orderID).Order("id desc").Find(&payments).Error; err != nil {
		return nil, err
	}
	return payments, nil
}

// GetLatestPendingByOrder 获取订单最新待支付记录
func (r *Store) GetLatestPendingByOrder(orderID uint, now time.Time) (*paymentdomain.Payment, error) {
	var payment paymentdomain.Payment
	result := r.db.
		Select("payments.*, payment_channels.name AS channel_name").
		Joins("LEFT JOIN payment_channels ON payment_channels.id = payments.channel_id AND payment_channels.deleted_at IS NULL").
		Where("payments.deleted_at IS NULL AND payments.order_id = ? AND payments.status IN ? AND payments.superseded_at IS NULL AND payments.fee_policy IN ? AND (payments.expired_at IS NULL OR payments.expired_at > ?) AND ((payments.pay_url IS NOT NULL AND payments.pay_url <> '') OR (payments.qr_code IS NOT NULL AND payments.qr_code <> '') OR (payments.requires_chain_verification = true AND payments.expired_at > ?))",
			orderID,
			[]string{constants.PaymentStatusInitiated, constants.PaymentStatusPending},
			[]string{constants.PaymentFeePolicyNone, constants.PaymentFeePolicyMerchantAbsorbed, constants.PaymentFeePolicyCustomerSurcharge},
			now, now,
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
func (r *Store) GetLatestPendingByOrderChannel(orderID uint, channelID uint, now time.Time) (*paymentdomain.Payment, error) {
	var payment paymentdomain.Payment
	result := r.db.Where("deleted_at IS NULL AND order_id = ? AND channel_id = ? AND status IN ? AND superseded_at IS NULL AND (expired_at IS NULL OR expired_at > ?) AND ((pay_url IS NOT NULL AND pay_url <> '') OR (qr_code IS NOT NULL AND qr_code <> '') OR (requires_chain_verification = true AND provider_ref <> '' AND expired_at > ?))",
		orderID,
		channelID,
		[]string{constants.PaymentStatusInitiated, constants.PaymentStatusPending},
		now, now,
	).Order("id desc").Limit(1).Find(&payment)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &payment, nil
}

// SupersedePendingByOrderID 将同一订单的旧支付链接替换为指定的新支付记录。
func (r *Store) SupersedePendingByOrderID(orderID, replacementPaymentID uint, supersededAt time.Time) (int64, error) {
	if orderID == 0 || replacementPaymentID == 0 {
		return 0, nil
	}
	result := r.db.Model(&paymentdomain.Payment{}).
		Where("deleted_at IS NULL AND order_id = ? AND id <> ? AND status IN ?", orderID, replacementPaymentID, []string{constants.PaymentStatusInitiated, constants.PaymentStatusPending}).
		Updates(map[string]interface{}{
			"status":                   constants.PaymentStatusExpired,
			"expired_at":               supersededAt,
			"superseded_at":            supersededAt,
			"superseded_by_payment_id": replacementPaymentID,
			"updated_at":               supersededAt,
		})
	return result.RowsAffected, result.Error
}

// ExpirePendingByOrderIDs 将指定订单的未完成支付记录标记为过期。
func (r *Store) ExpirePendingByOrderIDs(orderIDs []uint, expiredAt time.Time) (int64, error) {
	if len(orderIDs) == 0 {
		return 0, nil
	}
	result := r.db.Model(&paymentdomain.Payment{}).
		Where("deleted_at IS NULL AND order_id IN ? AND status IN ?", orderIDs, []string{constants.PaymentStatusInitiated, constants.PaymentStatusPending}).
		Updates(map[string]interface{}{
			"status":     constants.PaymentStatusExpired,
			"expired_at": expiredAt,
			"updated_at": expiredAt,
		})
	return result.RowsAffected, result.Error
}

// ListAdmin 管理端支付列表
func (r *Store) ListAdmin(filter paymentcontract.ListFilter) ([]paymentdomain.Payment, int64, error) {
	query := r.db.Model(&paymentdomain.Payment{}).Where("payments.deleted_at IS NULL")

	if filter.UserID != 0 {
		query = query.
			Joins("LEFT JOIN orders ON orders.id = payments.order_id AND orders.deleted_at IS NULL").
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
			"payments.fee_policy",
			"payments.exception_code",
			"payments.currency",
			"payments.status",
			"payments.provider_ref",
			"payments.created_at",
			"payments.updated_at",
			"payments.paid_at",
			"payments.expired_at",
			"payments.superseded_at",
			"payments.superseded_by_payment_id",
			"payments.callback_at",
		)
	}

	var total int64
	if !filter.SkipCount {
		if err := query.Count(&total).Error; err != nil {
			return nil, 0, err
		}
	}

	query = gormutil.ApplyPagination(query, filter.Page, filter.PageSize)

	var payments []paymentdomain.Payment
	if err := query.Order("payments.id desc").Find(&payments).Error; err != nil {
		return nil, 0, err
	}
	if filter.Lightweight {
		if err := r.fillPaymentDisplayChannelTypes(payments); err != nil {
			return nil, 0, err
		}
	}
	return payments, total, nil
}

// fillPaymentDisplayChannelTypes 为 lightweight 支付列表补充展示用渠道类型。
func (r *Store) fillPaymentDisplayChannelTypes(payments []paymentdomain.Payment) error {
	if len(payments) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(payments))
	indexByID := make(map[uint]int, len(payments))
	for idx := range payments {
		ids = append(ids, payments[idx].ID)
		indexByID[payments[idx].ID] = idx
	}
	type displayRow struct {
		ID                 uint
		DisplayChannelType string
	}
	var rows []displayRow
	displayChannelTypeExpr := gormutil.JSONTextExpr(r.db, "provider_payload", "display_channel_type")
	if err := r.db.Model(&paymentdomain.Payment{}).
		Select("id", displayChannelTypeExpr+" AS display_channel_type").
		Where("id IN ? AND deleted_at IS NULL", ids).
		Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		idx, ok := indexByID[row.ID]
		if !ok {
			continue
		}
		payments[idx].DisplayChannelType = strings.TrimSpace(row.DisplayChannelType)
	}
	return nil
}

// GetByIDForUpdate 事务中加行锁读取支付单,不存在返回 (nil, nil)。
func (r *Store) GetByIDForUpdate(id uint) (*paymentdomain.Payment, error) {
	if id == 0 {
		return nil, nil
	}
	var payment paymentdomain.Payment
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("deleted_at IS NULL").First(&payment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &payment, nil
}
