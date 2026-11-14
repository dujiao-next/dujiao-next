package contract

import (
	"testing"
	"time"
)

func TestOrderRefundWindowBoundary(t *testing.T) {
	paid := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)
	zero := time.Time{}
	for _, tc := range []struct {
		name    string
		created time.Time
		paid    *time.Time
		days    int
		now     time.Time
		want    bool
	}{
		{"at_deadline", paid.AddDate(0, 0, -10), &paid, 1, paid.AddDate(0, 0, 1), false},
		{"after_deadline", paid, &paid, 1, paid.AddDate(0, 0, 1).Add(time.Nanosecond), true},
		{"before_deadline", paid, &paid, 1, paid.Add(time.Hour), false},
		{"created_fallback", paid, nil, 1, paid.AddDate(0, 0, 2), true},
		{"zero_paid_fallback", paid, &zero, 1, paid.AddDate(0, 0, 2), true},
		{"missing_dates", zero, nil, 1, paid, false},
		{"unlimited", paid, &paid, 0, paid.AddDate(10, 0, 0), false},
		{"negative_uses_default", paid, &paid, -1, paid.AddDate(0, 0, 31), true},
		{"maximum_clamped", paid, &paid, 5000, paid.AddDate(0, 0, 3651), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsOrderRefundWindowExpired(tc.created, tc.paid, tc.days, tc.now); got != tc.want {
				t.Fatalf("expired = %v, want %v", got, tc.want)
			}
		})
	}
}
