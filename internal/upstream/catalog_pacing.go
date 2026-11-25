package upstream

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type catalogPacingKey struct{}

type catalogPacing struct {
	interval time.Duration
	next     time.Time
}

// WithCatalogPacing limits the rate of sequential requests in a catalog scan.
// Other adapter callers keep their existing request behavior.
func WithCatalogPacing(ctx context.Context, interval time.Duration) context.Context {
	return context.WithValue(ctx, catalogPacingKey{}, &catalogPacing{interval: interval})
}

func waitCatalogRequest(ctx context.Context) error {
	pacing, ok := ctx.Value(catalogPacingKey{}).(*catalogPacing)
	if !ok {
		return nil
	}
	if delay := time.Until(pacing.next); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	pacing.next = time.Now().Add(pacing.interval)
	return nil
}

type CatalogRateLimitError struct {
	RetryAt time.Time
}

func (*CatalogRateLimitError) Error() string { return "upstream catalog rate limited (HTTP 429)" }

func catalogRateLimit(ctx context.Context, response *http.Response) *CatalogRateLimitError {
	if response.StatusCode != http.StatusTooManyRequests || ctx.Value(catalogPacingKey{}) == nil {
		return nil
	}
	return &CatalogRateLimitError{RetryAt: catalogRetryAt(response.Header.Get("Retry-After"))}
}

func catalogRetryAt(value string) time.Time {
	header := strings.TrimSpace(value)
	if retryAt, err := http.ParseTime(header); err == nil {
		return retryAt
	}
	if delay, err := time.ParseDuration(header + "s"); err == nil && delay > 0 {
		return time.Now().Add(delay)
	}
	return time.Time{}
}
