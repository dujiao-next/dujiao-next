// Package candidates receives discovery evidence. It has no settlement port.
package candidates

import (
	"context"
	"errors"
	"fmt"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
)

type IntentRecoverer interface{ RecoverFixedPaymentIntents(context.Context) error }

type Service struct {
	Channels paymentcontract.ChannelStore
	Registry paymentcontract.GatewayRegistry
	Inbox    paymentcontract.CandidateInbox
	Recovery IntentRecoverer
}

// Run executes one bounded polling cycle. The next scheduled cycle discovers
// unfinished work from SQL and the unacknowledged gateway inbox, not Redis state.
func (s *Service) Run(ctx context.Context) error {
	if s == nil || s.Channels == nil || s.Registry == nil || s.Inbox == nil {
		return errors.New("candidate receiver unavailable")
	}
	subscriptions, ok := s.Inbox.(paymentcontract.CandidateSubscriptionStore)
	if !ok {
		return errors.New("candidate subscription store unavailable")
	}
	var failures []error
	// Register current channels; previously registered identities survive deletion.
	for page := 1; ; page++ {
		channels, total, err := s.Channels.List(paymentcontract.ChannelListFilter{ProviderType: "epusdt", Page: page, PageSize: 50})
		if err != nil {
			return err
		}
		for _, ch := range channels {
			p, ok := s.Registry.Lookup(ch.ProviderType, ch.ChannelType)
			if !ok {
				continue
			}
			receiver, ok := p.(paymentcontract.GatewayCandidateReceiver)
			if !ok {
				continue
			}
			source, enabled, err := receiver.CandidateSource(ch.ConfigJSON)
			if err != nil {
				failures = append(failures, fmt.Errorf("candidate channel %d configuration invalid", ch.ID))
				continue
			}
			if enabled {
				if err := subscriptions.SaveCandidateSubscription(ctx, source, ch.ChannelType, ch.ConfigJSON); err != nil {
					return err
				}
			}
		}
		if len(channels) == 0 || int64(page*50) >= total {
			break
		}
	}
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		rows, err := subscriptions.ListCandidateSubscriptions(ctx, page, 50)
		if err != nil {
			return err
		}
		for _, row := range rows {
			p, ok := s.Registry.Lookup("epusdt", row.ChannelType)
			if !ok {
				failures = append(failures, errors.New("historical candidate provider unavailable"))
				continue
			}
			receiver, ok := p.(paymentcontract.GatewayCandidateReceiver)
			if !ok {
				return errors.New("candidate protocol unavailable")
			}
			source, enabled, err := receiver.CandidateSource(row.ConfigJSON)
			if err != nil || !enabled || source.Key() != row.SourceKey {
				failures = append(failures, errors.New("historical subscription invalid"))
				continue
			}
			pollCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
			batch, err := receiver.ListCandidates(pollCtx, row.ConfigJSON)
			if err == nil && len(batch) > 0 {
				err = s.Inbox.CommitCandidates(pollCtx, source, batch)
				if err == nil {
					ids := make([]string, len(batch))
					for i := range batch {
						ids[i] = batch[i].ID
					}
					err = receiver.AcknowledgeCandidates(pollCtx, row.ConfigJSON, ids)
				}
			}
			healthy := false
			if observer, ok := p.(paymentcontract.GatewayScanObserver); ok && err == nil {
				var healthErr error
				healthy, healthErr = observer.ScanHealthy(pollCtx, row.ConfigJSON)
				if healthErr != nil {
					err = healthErr
				}
			}
			cancel()
			if recordErr := subscriptions.RecordCandidatePoll(ctx, row.SourceKey, err == nil, healthy, time.Now()); recordErr != nil {
				failures = append(failures, recordErr)
			}
			if err != nil {
				failures = append(failures, errors.New("candidate subscription delivery incomplete"))
			}
		}
		if len(rows) < 50 {
			break
		}
	}
	if s.Recovery != nil {
		if err := s.Recovery.RecoverFixedPaymentIntents(ctx); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
