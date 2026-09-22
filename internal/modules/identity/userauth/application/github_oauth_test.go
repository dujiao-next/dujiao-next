package application

import (
	"context"
	"testing"
	"time"
)

type fakeGitHubStateStore struct {
	values map[string]GitHubOAuthIntent
	ttl    time.Duration
	takes  int
}

func (s *fakeGitHubStateStore) PutGitHubIntent(_ context.Context, state string, intent GitHubOAuthIntent, ttl time.Duration) error {
	s.values[state] = intent
	s.ttl = ttl
	return nil
}
func (s *fakeGitHubStateStore) TakeGitHubIntent(_ context.Context, state string) (*GitHubOAuthIntent, error) {
	s.takes++
	v, ok := s.values[state]
	if !ok {
		return nil, nil
	}
	delete(s.values, state)
	return &v, nil
}

func TestGitHubOAuthStateIsHighEntropyShortLivedAndOneTime(t *testing.T) {
	store := &fakeGitHubStateStore{values: map[string]GitHubOAuthIntent{}}
	s := &Service{gitHubStateStore: store}
	state, err := s.CreateGitHubOAuthIntent(context.Background(), GitHubOAuthIntent{Origin: "https://outlooksell.uno", CreatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if len(state) != 43 || store.ttl != GitHubOAuthIntentTTL {
		t.Fatalf("state=%q ttl=%s", state, store.ttl)
	}
	intent, err := s.ConsumeGitHubOAuthIntent(context.Background(), state)
	if err != nil || intent == nil {
		t.Fatalf("first consume=%+v %v", intent, err)
	}
	intent, err = s.ConsumeGitHubOAuthIntent(context.Background(), state)
	if err == nil || intent != nil {
		t.Fatalf("replay accepted: %+v %v", intent, err)
	}
}

func TestGitHubCallbackURLUsesTrustedOriginAndExactPath(t *testing.T) {
	got, err := BuildGitHubCallbackURL("https://outlooksell.uno")
	if err != nil || got != "https://outlooksell.uno/api/v1/auth/github/callback" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	for _, bad := range []string{"javascript:alert(1)", "https://user:pass@example.com", "https://example.com/path", "https://example.com?x=1"} {
		if _, err := BuildGitHubCallbackURL(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
