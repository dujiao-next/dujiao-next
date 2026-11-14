package gormstore

import (
	"errors"
	"time"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	memberlevelapp "github.com/dujiao-next/internal/modules/memberlevel/application"
	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
)

// UserStore persists only the user fields owned by member-level progression.
type UserStore struct {
	db *gorm.DB
}

func NewUserStore(db *gorm.DB) *UserStore {
	return &UserStore{db: db}
}

func (r *UserStore) GetByID(id uint) (*userdomain.User, error) {
	var user userdomain.User
	if err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserStore) Update(user *userdomain.User) error {
	if user == nil || user.ID == 0 {
		return nil
	}
	return r.db.Model(&userdomain.User{}).
		Where("id = ? AND deleted_at IS NULL", user.ID).
		Select("*").
		Omit("id", "deleted_at").
		Updates(user).Error
}

func (r *UserStore) IncrementTotalRecharged(userID uint, amount decimal.Decimal) error {
	return r.incrementMoneyColumn(userID, "total_recharged", amount)
}

// DecrementTotalRecharged 原子扣回累计充值，历史累计不足时归零，避免出现负数。
func (r *UserStore) DecrementTotalRecharged(userID uint, amount decimal.Decimal) error {
	return DecrementTotalRecharged(r.db, userID, amount)
}

// DecrementTotalSpent 原子扣回累计消费，历史累计不足时归零。
func (r *UserStore) DecrementTotalSpent(userID uint, amount decimal.Decimal) error {
	return DecrementTotalSpent(r.db, userID, amount)
}

// OnOrderRefunded 在调用方的退款事务内扣回累计消费并检查等级回退。
func OnOrderRefunded(tx *gorm.DB, userID uint, amount decimal.Decimal) error {
	service := memberlevelapp.NewService(NewLevelStore(tx), NewPriceStore(tx), NewUserStore(tx), NewUserPriceStore(tx))
	return service.OnOrderRefunded(userID, amount)
}

// OnRechargeRefunded 在调用方的退款事务内扣回累计充值并检查等级回退。
func OnRechargeRefunded(tx *gorm.DB, userID uint, amount decimal.Decimal) error {
	service := memberlevelapp.NewService(NewLevelStore(tx), NewPriceStore(tx), NewUserStore(tx), NewUserPriceStore(tx))
	return service.OnRechargeRefunded(userID, amount)
}

// DecrementTotalRecharged 使用调用方的事务连接扣回累计充值，累计不足时归零。
func DecrementTotalRecharged(tx *gorm.DB, userID uint, amount decimal.Decimal) error {
	return decrementMoneyColumn(tx, userID, "total_recharged", amount)
}

// DecrementTotalSpent 使用调用方的事务连接扣回累计消费，累计不足时归零。
func DecrementTotalSpent(tx *gorm.DB, userID uint, amount decimal.Decimal) error {
	return decrementMoneyColumn(tx, userID, "total_spent", amount)
}

// decrementMoneyColumn 扣减指定累计字段，字段名仅由内部固定调用提供。
func decrementMoneyColumn(tx *gorm.DB, userID uint, column string, amount decimal.Decimal) error {
	amount = amount.Round(2)
	if userID == 0 || !amount.IsPositive() {
		return nil
	}
	value := money.FromDecimal(amount)
	result := tx.Model(&userdomain.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Updates(map[string]interface{}{
			column:       gorm.Expr("CASE WHEN "+column+" >= ? THEN "+column+" - ? ELSE 0 END", value, value),
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return memberlevelcontract.ErrUserNotFound
	}
	return nil
}

func (r *UserStore) IncrementTotalSpent(userID uint, amount decimal.Decimal) error {
	return r.incrementMoneyColumn(userID, "total_spent", amount)
}

func (r *UserStore) incrementMoneyColumn(userID uint, column string, amount decimal.Decimal) error {
	if userID == 0 {
		return nil
	}
	amount = amount.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	return r.db.Model(&userdomain.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Updates(map[string]interface{}{
			column:       gorm.Expr(column+" + ?", money.FromDecimal(amount)),
			"updated_at": time.Now(),
		}).Error
}

func (r *UserStore) UpdateMemberLevelIfCurrent(userID, currentLevelID, nextLevelID uint) (int64, error) {
	if userID == 0 || currentLevelID == nextLevelID {
		return 0, nil
	}
	result := r.db.Model(&userdomain.User{}).
		Where("id = ? AND member_level_id = ? AND deleted_at IS NULL", userID, currentLevelID).
		Updates(map[string]interface{}{
			"member_level_id": nextLevelID,
			"updated_at":      time.Now(),
		})
	return result.RowsAffected, result.Error
}

func (r *UserStore) AssignDefaultMemberLevel(defaultLevelID uint) (int64, error) {
	if defaultLevelID == 0 {
		return 0, nil
	}
	result := r.db.Model(&userdomain.User{}).
		Where("(member_level_id = 0 OR member_level_id IS NULL) AND deleted_at IS NULL").
		Updates(map[string]interface{}{
			"member_level_id": defaultLevelID,
			"updated_at":      time.Now(),
		})
	return result.RowsAffected, result.Error
}

var _ memberlevelcontract.UserRepository = (*UserStore)(nil)
