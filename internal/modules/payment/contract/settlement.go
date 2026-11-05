package contract

import (
	"context"
	"errors"
	"time"

	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
)

var ErrEffectPermanent = errors.New("payment notification needs manual review")

var ErrAllocationConflict = errors.New("payment allocation unavailable or historical ownership incomplete")

type AllocationStore interface {
	ReserveAllocation(paymentdomain.Allocation) error
}

type SettlementTransaction interface {
	Transaction
	Users() usercontract.Store
	// MemberLevelUsers 返回会员等级模块的用户仓储。
	// 上游 v1.5 把 IncrementTotalSpent / AssignDefaultMemberLevel 从 identity 的
	// usercontract.Store 挪到了 memberlevel 自己的契约里，链核验结算在事务内
	// 加消费额、做等级检查都得走这个入口。
	MemberLevelUsers() memberlevelcontract.UserRepository
	Allocation(uint) (*paymentdomain.Allocation, error)
	ConsumeEvent(paymentdomain.EventConsumption) (bool, error)
	AddPaymentEffects([]paymentdomain.PaymentEffect) error
	CompletePaymentEffect(paymentdomain.PaymentEffect) (bool, error)
	LockAutomation(uint64) error
}

type SettlementWork struct {
	AutomationRevision uint64
	Evidence           paymentdomain.CandidateEvidence
	Receipt            paymentdomain.CandidateReceipt
	Payment            paymentdomain.Payment
}

type SettlementStore interface {
	LeaseSettlements(context.Context, time.Time) ([]SettlementWork, error)
	WithSettlement(context.Context, SettlementWork, func(SettlementTransaction, *paymentdomain.Payment, *orderdomain.Order, *paymentdomain.CandidateEvidence, *paymentdomain.CandidateReceipt) error) error
	FinishSettlement(SettlementTransaction, *paymentdomain.CandidateEvidence, string, string, time.Time) error
}

type EffectStore interface {
	LeasePaymentEffects(context.Context, time.Time) ([]paymentdomain.PaymentEffect, error)
	FinishPaymentEffect(context.Context, paymentdomain.PaymentEffect, string, string, time.Time) error
}

type SettlementApplier func(SettlementTransaction, *orderdomain.Order, *paymentdomain.Payment, time.Time) error
type EffectHandler interface {
	HandlePaymentEffect(context.Context, paymentdomain.PaymentEffect) (bool, error)
}
