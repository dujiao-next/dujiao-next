package securityalerts

import (
	"context"
	"errors"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	"time"
)

// Run deliberately ignores the payment freeze. Alerts report observations only.
type Service struct {
	Store  pc.SecurityAlertStore
	Sender pc.SecurityAlertSender
}

func (s *Service) Run(ctx context.Context) error {
	if s == nil || s.Store == nil || s.Sender == nil {
		return errors.New("payment security alerts unavailable")
	}
	if err := s.Store.ObserveSecurity(ctx, time.Now()); err != nil {
		return err
	}
	rows, err := s.Store.LeaseSecurityAlerts(ctx, time.Now())
	if err != nil {
		return err
	}
	var failures []error
	for _, row := range rows {
		callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := s.Sender.Send(callCtx, row)
		cancel()
		if err != nil {
			failures = append(failures, errors.New("payment security alert send failed"))
		}
		if err := s.Store.FinishSecurityAlert(ctx, row, err == nil, time.Now()); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
