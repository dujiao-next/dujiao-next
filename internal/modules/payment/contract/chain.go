package contract

import (
	"context"
	"errors"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
)

var (
	ErrChainUnavailable   = errors.New("chain RPC unavailable or malformed")
	ErrChainPending       = errors.New("chain transaction not finalized")
	ErrChainMismatch      = errors.New("chain evidence mismatch")
	ErrChainDisagreement  = errors.New("chain sources disagree")
	ErrChainAmbiguous     = errors.New("chain transfer ambiguous")
	ErrChainOutsideWindow = errors.New("chain transfer outside intent window")
)

type ChainVerifier interface {
	Verify(context.Context, paymentdomain.ChainExpectation, string) (*paymentdomain.ChainProof, error)
}

// GatewayChainIntentReader revalidates a stored intent without HTTP, current
// channel configuration, repricing or extending its acceptance window.
type GatewayChainIntentReader interface {
	ReadChainExpectation(paymentdomain.Payment) (paymentdomain.ChainExpectation, error)
}

type ChainCheckStore interface {
	LeaseChainChecks(context.Context, time.Time, int) ([]paymentdomain.ChainCheckWork, error)
	FinishChainCheck(context.Context, paymentdomain.ChainCheckWork, string, string, *paymentdomain.ChainProof, time.Time) error
}
