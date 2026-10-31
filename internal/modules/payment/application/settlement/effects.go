package settlement

import (
	"context"
	"errors"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	"time"
)

type Effects struct {
	Store   pc.EffectStore
	Handler pc.EffectHandler
}

func (s *Effects) Run(ctx context.Context) error {
	if s == nil || s.Store == nil || s.Handler == nil {
		return errors.New("payment effects unavailable")
	}
	var failures []error
	for n := 0; n < 20; n++ {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		rows, err := s.Store.LeasePaymentEffects(ctx, time.Now())
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		w := rows[0]
		callCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		done, err := s.Handler.HandlePaymentEffect(callCtx, w)
		cancel()
		state, code := "pending", "awaiting_effect"
		if errors.Is(err, pc.ErrEffectPermanent) {
			state, code = "manual", "delivery_rejected"
		} else if err != nil {
			code = "effect_retry"
			failures = append(failures, err)
		} else if done {
			state, code = "done", "effect_completed"
		}
		delay := time.Duration(w.Attempts) * time.Minute
		if delay < time.Minute {
			delay = time.Minute
		}
		if delay > 15*time.Minute {
			delay = 15 * time.Minute
		}
		if err := s.Store.FinishPaymentEffect(ctx, w, state, code, time.Now().Add(delay)); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
