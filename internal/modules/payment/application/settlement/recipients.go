package settlement

import (
	"context"
	"errors"
	nc "github.com/dujiao-next/internal/modules/notification/contract"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"sort"
)

type RecipientStore interface {
	PrepareRecipients(context.Context, pd.PaymentEffect, []string) error
	AdmitRecipient(context.Context, pd.PaymentEffect, string) (bool, error)
	FinishRecipient(context.Context, pd.PaymentEffect, string, bool) error
}

// RecipientBatch retains confirmed destinations across parent-effect retries.
// Remote success followed by local ACK loss can still duplicate that destination.
func RecipientBatch(store RecipientStore, work pd.PaymentEffect) nc.DeliverBatch {
	return func(ctx context.Context, deliveries []nc.RecipientDelivery) error {
		keys := make([]string, 0, len(deliveries))
		byKey := map[string]nc.RecipientDelivery{}
		for _, d := range deliveries {
			if len(d.Key) != 64 || d.Send == nil {
				return pc.ErrEffectPermanent
			}
			if _, ok := byKey[d.Key]; !ok {
				keys = append(keys, d.Key)
				byKey[d.Key] = d
			}
		}
		sort.Strings(keys)
		if err := store.PrepareRecipients(ctx, work, keys); err != nil {
			return err
		}
		var failures []error
		for _, key := range keys {
			if err := ctx.Err(); err != nil {
				return errors.Join(append(failures, err)...)
			}
			send, err := store.AdmitRecipient(ctx, work, key)
			if err != nil {
				return errors.Join(append(failures, err)...)
			}
			if !send {
				continue
			}
			err = byKey[key].Send(ctx)
			// An ACK needs durable SQL even when the IO consumed its context.
			// Do not detach: failure leaves it retryable, never falsely complete.
			if ackErr := store.FinishRecipient(ctx, work, key, err == nil); ackErr != nil {
				return errors.Join(err, ackErr)
			}
			if err != nil {
				failures = append(failures, err)
			}
		}
		return errors.Join(failures...)
	}
}
