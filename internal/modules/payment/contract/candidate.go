package contract

import (
	"context"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

type GatewayCandidateReceiver interface {
	CandidateSource(cfg jsonmap.JSON) (paymentdomain.CandidateSource, bool, error)
	ListCandidates(context.Context, jsonmap.JSON) ([]paymentdomain.TransferCandidate, error)
	AcknowledgeCandidates(context.Context, jsonmap.JSON, []string) error
}

type CandidateInbox interface {
	// CommitCandidates must atomically persist the complete batch before returning.
	CommitCandidates(context.Context, paymentdomain.CandidateSource, []paymentdomain.TransferCandidate) error
}

// IntentRecoveryStore is optional for unrelated providers and test stores.
type IntentRecoveryStore interface {
	ListIntentRecoveries(time.Time, int) ([]paymentdomain.Payment, error)
	ClaimIntentRecovery(uint, time.Time, time.Time) (bool, error)
}

type CandidateSubscriptionStore interface {
	SaveCandidateSubscription(context.Context, paymentdomain.CandidateSource, string, jsonmap.JSON) error
	ListCandidateSubscriptions(context.Context, int, int) ([]paymentdomain.CandidateSubscription, error)
	RecordCandidatePoll(context.Context, string, bool, bool, time.Time) error
}

type GatewayScanObserver interface {
	ScanHealthy(context.Context, jsonmap.JSON) (bool, error)
}
