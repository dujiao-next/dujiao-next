package emailverificationcontract

import (
	"time"

	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
)

type Store interface {
	Create(*emailverificationdomain.Code) error
	GetLatest(email, purpose string) (*emailverificationdomain.Code, error)
	// MarkVerified 仅标记尚未使用的验证码，返回是否由本次调用完成标记（防止并发重复使用）。
	MarkVerified(id uint, verifiedAt time.Time) (bool, error)
	// ReserveAttempt 原子占用一次尝试机会；已用完 maxAttempts（>0 时）或已验证则返回 false。
	ReserveAttempt(id uint, maxAttempts int) (bool, error)
	// Delete 软删除验证码（如邮件未发出），使其不再占用发送间隔、也不遮挡之前已送达的验证码。
	Delete(id uint) error
}
