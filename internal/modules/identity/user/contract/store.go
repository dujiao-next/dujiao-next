package usercontract

import (
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
)

// ListFilter describes the administration user-directory query.
type ListFilter struct {
	Page          int
	PageSize      int
	UserID        uint
	InviteCodeID  uint
	Keyword       string
	Status        string
	CreatedFrom   *time.Time
	CreatedTo     *time.Time
	LastLoginFrom *time.Time
	LastLoginTo   *time.Time
	SortBy        string
	SortOrder     string
}

// Store is the persistence port for user accounts and their authentication state.
type Store interface {
	GetByEmail(string) (*userdomain.User, error)
	GetByID(uint) (*userdomain.User, error)
	ListByIDs([]uint) ([]userdomain.User, error)
	Create(*userdomain.User) error
	Update(*userdomain.User) error
	List(ListFilter) ([]userdomain.User, int64, error)
	BatchUpdateStatus([]uint, string) error
	UpdateTOTPPending(uint, string, time.Time) error
	UpdateTOTPEnabled(uint, string, time.Time, string) error
	UpdateRecoveryCodes(uint, string) error
	ClearTOTP(uint) error
}
