package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// UserRepository 用户数据访问接口
type UserRepository interface {
	GetByEmail(email string) (*models.User, error)
	GetByID(id uint) (*models.User, error)
	ListByIDs(ids []uint) ([]models.User, error)
	Create(user *models.User) error
	Update(user *models.User) error
	IncrementTotalRecharged(userID uint, amount decimal.Decimal) error
	IncrementTotalSpent(userID uint, amount decimal.Decimal) error
	UpdateMemberLevelIfCurrent(userID, currentLevelID, nextLevelID uint) (int64, error)
	List(filter UserListFilter) ([]models.User, int64, error)
	CountByMemberLevelIDs(levelIDs []uint) (map[uint]int64, error)
	BatchUpdateStatus(userIDs []uint, status string) error
	AssignDefaultMemberLevel(defaultLevelID uint) (int64, error)

	// TOTP 相关
	UpdateTOTPPending(userID uint, encSecret string, expiresAt time.Time) error
	UpdateTOTPEnabled(userID uint, encSecret string, enabledAt time.Time, recoveryCodesJSON string) error
	UpdateRecoveryCodes(userID uint, recoveryCodesJSON string) error
	ClearTOTP(userID uint) error
}

// GormUserRepository GORM 实现
type GormUserRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓库
func NewUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

// GetByEmail 根据邮箱获取用户
func (r *GormUserRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetByID 根据 ID 获取用户
func (r *GormUserRepository) GetByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// ListByIDs 批量获取用户
func (r *GormUserRepository) ListByIDs(ids []uint) ([]models.User, error) {
	if len(ids) == 0 {
		return []models.User{}, nil
	}
	var users []models.User
	if err := r.db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// Create 创建用户
func (r *GormUserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// Update 更新用户
func (r *GormUserRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

// IncrementTotalRecharged 原子累加用户累计充值金额。
func (r *GormUserRepository) IncrementTotalRecharged(userID uint, amount decimal.Decimal) error {
	return r.incrementMoneyColumn(userID, "total_recharged", amount)
}

// IncrementTotalSpent 原子累加用户累计消费金额。
func (r *GormUserRepository) IncrementTotalSpent(userID uint, amount decimal.Decimal) error {
	return r.incrementMoneyColumn(userID, "total_spent", amount)
}

func (r *GormUserRepository) incrementMoneyColumn(userID uint, column string, amount decimal.Decimal) error {
	if userID == 0 {
		return nil
	}
	amount = amount.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	return r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			column:       gorm.Expr(column+" + ?", models.NewMoneyFromDecimal(amount)),
			"updated_at": time.Now(),
		}).Error
}

// UpdateMemberLevelIfCurrent 仅在用户当前等级未被其他流程改变时更新会员等级。
func (r *GormUserRepository) UpdateMemberLevelIfCurrent(userID, currentLevelID, nextLevelID uint) (int64, error) {
	if userID == 0 || currentLevelID == nextLevelID {
		return 0, nil
	}
	result := r.db.Model(&models.User{}).
		Where("id = ? AND member_level_id = ?", userID, currentLevelID).
		Updates(map[string]interface{}{
			"member_level_id": nextLevelID,
			"updated_at":      time.Now(),
		})
	return result.RowsAffected, result.Error
}

// List 用户列表
func (r *GormUserRepository) List(filter UserListFilter) ([]models.User, int64, error) {
	query := r.db.Model(&models.User{})

	if filter.UserID != 0 {
		query = query.Where("users.id = ?", filter.UserID)
	}
	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		query = query.Where(
			"email LIKE ? OR display_name LIKE ? OR EXISTS ("+
				"SELECT 1 FROM user_oauth_identities "+
				"WHERE user_oauth_identities.user_id = users.id "+
				"AND ("+
				"user_oauth_identities.provider LIKE ? OR "+
				"user_oauth_identities.provider_user_id LIKE ? OR "+
				"user_oauth_identities.username LIKE ?"+
				")"+
				")",
			like, like, like, like, like,
		)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.MemberLevelID != nil {
		query = query.Where("member_level_id = ?", *filter.MemberLevelID)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}
	if filter.LastLoginFrom != nil {
		query = query.Where("last_login_at >= ?", *filter.LastLoginFrom)
	}
	if filter.LastLoginTo != nil {
		query = query.Where("last_login_at <= ?", *filter.LastLoginTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = applyPagination(query, filter.Page, filter.PageSize)
	query = applyUserSort(query, filter.SortBy, filter.SortOrder)

	var users []models.User
	if err := query.Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// applyUserSort 按白名单字段排序，并追加 users.id DESC 作为二级排序保证分页稳定；
// 非法字段回退默认 id DESC。
func applyUserSort(query *gorm.DB, sortBy, sortOrder string) *gorm.DB {
	dir := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		dir = "ASC"
	}
	switch strings.TrimSpace(sortBy) {
	case "created_at":
		return query.Order("users.created_at " + dir).Order("users.id DESC")
	case "last_login_at":
		// (last_login_at IS NULL) 让从未登录的用户始终垫底，跨 SQLite/PostgreSQL 行为一致
		return query.Order("users.last_login_at IS NULL").Order("users.last_login_at " + dir).Order("users.id DESC")
	case "wallet_balance":
		// 钱包余额在 wallet_accounts 表，LEFT JOIN + COALESCE(0) 让无账户用户按余额 0 处理
		return query.
			Joins("LEFT JOIN wallet_accounts ON wallet_accounts.user_id = users.id").
			Select("users.*").
			Order("COALESCE(wallet_accounts.balance, 0) " + dir).
			Order("users.id DESC")
	default:
		return query.Order("users.id DESC")
	}
}

// CountByMemberLevelIDs 按等级 ID 批量统计会员人数，返回 levelID → count 映射。
// 查询失败时返回 error（调用方决定是否降级为 0）。
func (r *GormUserRepository) CountByMemberLevelIDs(levelIDs []uint) (map[uint]int64, error) {
	if len(levelIDs) == 0 {
		return map[uint]int64{}, nil
	}
	type row struct {
		MemberLevelID uint  `gorm:"column:member_level_id"`
		Count         int64 `gorm:"column:count"`
	}
	var rows []row
	if err := r.db.Model(&models.User{}).
		Select("member_level_id, COUNT(*) as count").
		Where("member_level_id IN ?", levelIDs).
		Group("member_level_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]int64, len(rows))
	for _, r := range rows {
		result[r.MemberLevelID] = r.Count
	}
	return result, nil
}

// BatchUpdateStatus 批量更新用户状态
func (r *GormUserRepository) BatchUpdateStatus(userIDs []uint, status string) error {
	if len(userIDs) == 0 {
		return nil
	}
	now := time.Now()
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": now,
	}
	if strings.ToLower(strings.TrimSpace(status)) == constants.UserStatusDisabled {
		updates["token_invalid_before"] = now
		updates["token_version"] = gorm.Expr("token_version + 1")
	}
	return r.db.Model(&models.User{}).Where("id IN ?", userIDs).Updates(updates).Error
}

// AssignDefaultMemberLevel 为所有未分配等级(member_level_id=0)的用户批量分配默认等级
func (r *GormUserRepository) AssignDefaultMemberLevel(defaultLevelID uint) (int64, error) {
	if defaultLevelID == 0 {
		return 0, nil
	}
	result := r.db.Model(&models.User{}).
		Where("member_level_id = 0 OR member_level_id IS NULL").
		Updates(map[string]interface{}{
			"member_level_id": defaultLevelID,
			"updated_at":      time.Now(),
		})
	return result.RowsAffected, result.Error
}

// UpdateTOTPPending 写入待绑定 secret 与过期时间
func (r *GormUserRepository) UpdateTOTPPending(userID uint, encSecret string, expiresAt time.Time) error {
	if userID == 0 {
		return errors.New("invalid user id")
	}
	return r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"totp_pending_secret":     encSecret,
		"totp_pending_expires_at": expiresAt,
	}).Error
}

// UpdateTOTPEnabled 完成绑定：迁移 pending → 正式 secret，写入恢复码，清空 pending；
// 同时 TokenVersion++ 强制其他设备的旧 session 失效（提升安全级别等同于改密码）。
func (r *GormUserRepository) UpdateTOTPEnabled(userID uint, encSecret string, enabledAt time.Time, recoveryCodesJSON string) error {
	if userID == 0 {
		return errors.New("invalid user id")
	}
	return r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"totp_secret":             encSecret,
		"totp_enabled_at":         enabledAt,
		"totp_pending_secret":     "",
		"totp_pending_expires_at": nil,
		"recovery_codes":          recoveryCodesJSON,
		"token_version":           gorm.Expr("token_version + 1"),
		"token_invalid_before":    enabledAt,
	}).Error
}

// UpdateRecoveryCodes 替换恢复码 JSON
func (r *GormUserRepository) UpdateRecoveryCodes(userID uint, recoveryCodesJSON string) error {
	if userID == 0 {
		return errors.New("invalid user id")
	}
	return r.db.Model(&models.User{}).Where("id = ?", userID).Update("recovery_codes", recoveryCodesJSON).Error
}

// ClearTOTP 清空所有 TOTP 字段，TokenVersion++ 强制下线
func (r *GormUserRepository) ClearTOTP(userID uint) error {
	if userID == 0 {
		return errors.New("invalid user id")
	}
	now := time.Now()
	return r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"totp_secret":             "",
		"totp_enabled_at":         nil,
		"totp_pending_secret":     "",
		"totp_pending_expires_at": nil,
		"recovery_codes":          "",
		"token_version":           gorm.Expr("token_version + 1"),
		"token_invalid_before":    now,
	}).Error
}
