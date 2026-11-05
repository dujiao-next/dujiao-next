package userstore

import (
	"errors"
	"github.com/dujiao-next/internal/persistence/gormutil"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"gorm.io/gorm"
)

// Store persists user accounts with GORM.
type Store struct {
	db *gorm.DB
}

// New creates a user account store.
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// GetByEmail 根据邮箱获取用户
func (r *Store) GetByEmail(email string) (*userdomain.User, error) {
	var user userdomain.User
	if err := r.db.Where("email = ? AND deleted_at IS NULL", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetByID 根据 ID 获取用户
func (r *Store) GetByID(id uint) (*userdomain.User, error) {
	var user userdomain.User
	if err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// ListByIDs 批量获取用户
func (r *Store) ListByIDs(ids []uint) ([]userdomain.User, error) {
	if len(ids) == 0 {
		return []userdomain.User{}, nil
	}
	var users []userdomain.User
	if err := r.db.Where("id IN ? AND deleted_at IS NULL", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// Create 创建用户
func (r *Store) Create(user *userdomain.User) error {
	return r.db.Create(user).Error
}

// Update 更新用户
func (r *Store) Update(user *userdomain.User) error {
	if user == nil || user.ID == 0 {
		return nil
	}
	return r.db.Model(&userdomain.User{}).
		Where("id = ? AND deleted_at IS NULL", user.ID).
		Select("*").
		Omit("id", "deleted_at").
		Updates(user).Error
}

// List 用户列表
func (r *Store) List(filter usercontract.ListFilter) ([]userdomain.User, int64, error) {
	query := r.db.Model(&userdomain.User{}).Where("users.deleted_at IS NULL")

	if filter.UserID != 0 {
		query = query.Where("users.id = ?", filter.UserID)
	}
	if filter.InviteCodeID != 0 {
		query = query.Where("users.invite_code_id = ?", filter.InviteCodeID)
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

	query = gormutil.ApplyPagination(query, filter.Page, filter.PageSize)
	query = applyUserSort(query, filter.SortBy, filter.SortOrder)

	var users []userdomain.User
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

// BatchUpdateStatus 批量更新用户状态
func (r *Store) BatchUpdateStatus(userIDs []uint, status string) error {
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
	return r.db.Model(&userdomain.User{}).Where("id IN ? AND deleted_at IS NULL", userIDs).Updates(updates).Error
}

// UpdateTOTPPending 写入待绑定 secret 与过期时间
func (r *Store) UpdateTOTPPending(userID uint, encSecret string, expiresAt time.Time) error {
	if userID == 0 {
		return errors.New("invalid user id")
	}
	return r.db.Model(&userdomain.User{}).Where("id = ? AND deleted_at IS NULL", userID).Updates(map[string]interface{}{
		"totp_pending_secret":     encSecret,
		"totp_pending_expires_at": expiresAt,
	}).Error
}

// UpdateTOTPEnabled 完成绑定：迁移 pending → 正式 secret，写入恢复码，清空 pending；
// 同时 TokenVersion++ 强制其他设备的旧 session 失效（提升安全级别等同于改密码）。
func (r *Store) UpdateTOTPEnabled(userID uint, encSecret string, enabledAt time.Time, recoveryCodesJSON string) error {
	if userID == 0 {
		return errors.New("invalid user id")
	}
	return r.db.Model(&userdomain.User{}).Where("id = ? AND deleted_at IS NULL", userID).Updates(map[string]interface{}{
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
func (r *Store) UpdateRecoveryCodes(userID uint, recoveryCodesJSON string) error {
	if userID == 0 {
		return errors.New("invalid user id")
	}
	return r.db.Model(&userdomain.User{}).Where("id = ? AND deleted_at IS NULL", userID).Update("recovery_codes", recoveryCodesJSON).Error
}

// ClearTOTP 清空所有 TOTP 字段，TokenVersion++ 强制下线
func (r *Store) ClearTOTP(userID uint) error {
	if userID == 0 {
		return errors.New("invalid user id")
	}
	now := time.Now()
	return r.db.Model(&userdomain.User{}).Where("id = ? AND deleted_at IS NULL", userID).Updates(map[string]interface{}{
		"totp_secret":             "",
		"totp_enabled_at":         nil,
		"totp_pending_secret":     "",
		"totp_pending_expires_at": nil,
		"recovery_codes":          "",
		"token_version":           gorm.Expr("token_version + 1"),
		"token_invalid_before":    now,
	}).Error
}
