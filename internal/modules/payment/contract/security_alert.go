package contract

import (
	"context"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"time"
)

type SecurityAlertStore interface {
	ObserveSecurity(context.Context, time.Time) error
	LeaseSecurityAlerts(context.Context, time.Time) ([]pd.SecurityAlert, error)
	FinishSecurityAlert(context.Context, pd.SecurityAlert, bool, time.Time) error
}
type SecurityAlertSender interface {
	Send(context.Context, pd.SecurityAlert) error
}
