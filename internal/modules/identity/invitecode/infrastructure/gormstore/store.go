package invitecodestore

import (
	"errors"
	"time"

	invitecodedomain "github.com/dujiao-next/internal/modules/identity/invitecode/domain"

	"gorm.io/gorm"
)

// Store persists invite codes with GORM.
type Store struct {
	db *gorm.DB
}

// New creates an invite code store.
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// CreateBatch 在一个事务内写入一批邀请码。
func (s *Store) CreateBatch(codes []invitecodedomain.InviteCode) error {
	if len(codes) == 0 {
		return nil
	}
	return s.db.Create(&codes).Error
}

// GetByID 按主键查询。
func (s *Store) GetByID(id uint) (*invitecodedomain.InviteCode, error) {
	return first(s.db.Where("id = ?", id))
}

// GetByCode 按邀请码查询（code 需已归一化）。
func (s *Store) GetByCode(code string) (*invitecodedomain.InviteCode, error) {
	if code == "" {
		return nil, nil
	}
	return first(s.db.Where("code = ?", code))
}

// List 分页查询。
func (s *Store) List(code, status string, page, pageSize int) ([]invitecodedomain.InviteCode, int64, error) {
	query := s.db.Model(&invitecodedomain.InviteCode{})
	if code != "" {
		query = query.Where("code LIKE ?", "%"+code+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var codes []invitecodedomain.InviteCode
	err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&codes).Error
	return codes, total, err
}

// Update 保存邀请码可编辑字段（不覆盖 used_count，避免与并发核销互相覆盖）。
func (s *Store) Update(code *invitecodedomain.InviteCode) error {
	return s.db.Model(code).Select("max_uses", "expires_at", "status", "note", "updated_at").Updates(code).Error
}

// Delete 删除邀请码；已注册用户保留 invite_code_id 作为历史记录。
func (s *Store) Delete(id uint) error {
	return s.db.Delete(&invitecodedomain.InviteCode{}, id).Error
}

// Consume 在调用方事务内原子核销一次邀请码，条件 UPDATE 保证并发下不超用。
// 返回被核销的邀请码 ID；码不存在或不可用时返回 0。
func Consume(tx *gorm.DB, code string, now time.Time) (uint, error) {
	existing, err := first(tx.Where("code = ?", invitecodedomain.NormalizeCode(code)))
	if err != nil || existing == nil {
		return 0, err
	}
	result := tx.Model(&invitecodedomain.InviteCode{}).
		Where("id = ? AND status = ? AND (max_uses = 0 OR used_count < max_uses) AND (expires_at IS NULL OR expires_at > ?)",
			existing.ID, invitecodedomain.StatusActive, now).
		Updates(map[string]interface{}{"used_count": gorm.Expr("used_count + 1"), "updated_at": now})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, nil
	}
	return existing.ID, nil
}

func first(query *gorm.DB) (*invitecodedomain.InviteCode, error) {
	var code invitecodedomain.InviteCode
	err := query.First(&code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &code, nil
}
