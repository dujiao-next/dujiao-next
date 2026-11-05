package application

import (
	"crypto/rand"
	"errors"
	"math/big"
	"regexp"
	"strings"
	"time"

	invitecodedomain "github.com/dujiao-next/internal/modules/identity/invitecode/domain"
)

var (
	ErrInvalid  = errors.New("invite code invalid input")
	ErrNotFound = errors.New("invite code not found")
	ErrExists   = errors.New("invite code exists")
)

const (
	maxGenerateQuantity = 1000
	generatedCodeLength = 10
	// 去掉 0/O/1/I 等易混淆字符，便于口头或截图分发。
	codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

var customCodePattern = regexp.MustCompile(`^[A-Z0-9_-]{4,32}$`)

// Store 是邀请码管理的数据端口。
type Store interface {
	CreateBatch(codes []invitecodedomain.InviteCode) error
	GetByID(id uint) (*invitecodedomain.InviteCode, error)
	GetByCode(code string) (*invitecodedomain.InviteCode, error)
	List(code, status string, page, pageSize int) ([]invitecodedomain.InviteCode, int64, error)
	Update(code *invitecodedomain.InviteCode) error
	Delete(id uint) error
}

// Service 邀请码后台管理用例。
type Service struct {
	store Store
}

func NewService(store Store) *Service {
	if store == nil {
		panic("invite code service: store is nil")
	}
	return &Service{store: store}
}

// GenerateInput 批量生成参数；Code 非空时只生成这一个自定义码。
type GenerateInput struct {
	Quantity  int
	Code      string
	MaxUses   int
	ExpiresAt *time.Time
	Note      string
	CreatedBy *uint
}

// UpdateInput 可编辑字段，nil 表示不修改。
type UpdateInput struct {
	MaxUses      *int
	ExpiresAt    *time.Time
	ClearExpires bool
	Status       *string
	Note         *string
}

// Generate 批量生成邀请码。
func (s *Service) Generate(input GenerateInput) ([]invitecodedomain.InviteCode, error) {
	custom := invitecodedomain.NormalizeCode(input.Code)
	if custom != "" {
		input.Quantity = 1
		if !customCodePattern.MatchString(custom) {
			return nil, ErrInvalid
		}
		existing, err := s.store.GetByCode(custom)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, ErrExists
		}
	}
	if input.Quantity <= 0 || input.Quantity > maxGenerateQuantity || input.MaxUses < 0 {
		return nil, ErrInvalid
	}
	note := strings.TrimSpace(input.Note)
	if len([]rune(note)) > 255 {
		return nil, ErrInvalid
	}

	now := time.Now()
	codes := make([]invitecodedomain.InviteCode, 0, input.Quantity)
	seen := make(map[string]struct{}, input.Quantity)
	for len(codes) < input.Quantity {
		code := custom
		if code == "" {
			generated, err := randomCode()
			if err != nil {
				return nil, err
			}
			code = generated
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, invitecodedomain.InviteCode{
			Code:      code,
			MaxUses:   input.MaxUses,
			ExpiresAt: input.ExpiresAt,
			Status:    invitecodedomain.StatusActive,
			Note:      note,
			CreatedBy: input.CreatedBy,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	// shortcut: 随机码与库中已有码碰撞（32^10 空间）时整批失败，管理员重试即可。
	if err := s.store.CreateBatch(codes); err != nil {
		return nil, err
	}
	return codes, nil
}

// List 分页查询邀请码。
func (s *Service) List(code, status string, page, pageSize int) ([]invitecodedomain.InviteCode, int64, error) {
	return s.store.List(invitecodedomain.NormalizeCode(code), strings.TrimSpace(status), page, pageSize)
}

// Update 修改邀请码。
func (s *Service) Update(id uint, input UpdateInput) (*invitecodedomain.InviteCode, error) {
	code, err := s.store.GetByID(id)
	if err != nil {
		return nil, err
	}
	if code == nil {
		return nil, ErrNotFound
	}
	if input.MaxUses != nil {
		if *input.MaxUses < 0 {
			return nil, ErrInvalid
		}
		code.MaxUses = *input.MaxUses
	}
	if input.ClearExpires {
		code.ExpiresAt = nil
	} else if input.ExpiresAt != nil {
		code.ExpiresAt = input.ExpiresAt
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		if status != invitecodedomain.StatusActive && status != invitecodedomain.StatusDisabled {
			return nil, ErrInvalid
		}
		code.Status = status
	}
	if input.Note != nil {
		note := strings.TrimSpace(*input.Note)
		if len([]rune(note)) > 255 {
			return nil, ErrInvalid
		}
		code.Note = note
	}
	code.UpdatedAt = time.Now()
	if err := s.store.Update(code); err != nil {
		return nil, err
	}
	return code, nil
}

// Delete 删除邀请码。
func (s *Service) Delete(id uint) error {
	code, err := s.store.GetByID(id)
	if err != nil {
		return err
	}
	if code == nil {
		return ErrNotFound
	}
	return s.store.Delete(id)
}

// IsUsable 只读校验邀请码当前是否可用（不核销），用于发送注册验证码前的预检。
func (s *Service) IsUsable(code string) (bool, error) {
	if s == nil {
		return false, nil
	}
	found, err := s.store.GetByCode(invitecodedomain.NormalizeCode(code))
	if err != nil {
		return false, err
	}
	return found.Usable(time.Now()), nil
}

func randomCode() (string, error) {
	buf := make([]byte, generatedCodeLength)
	max := big.NewInt(int64(len(codeAlphabet)))
	for i := range buf {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		buf[i] = codeAlphabet[n.Int64()]
	}
	return string(buf), nil
}
