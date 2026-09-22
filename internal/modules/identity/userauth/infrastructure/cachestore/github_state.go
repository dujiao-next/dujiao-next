package cachestore

import (
	"context"
	"fmt"
	"time"

	"github.com/dujiao-next/internal/cache"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
)

type GitHubStateStore struct{}

func NewGitHubStateStore() *GitHubStateStore { return &GitHubStateStore{} }
func (*GitHubStateStore) PutGitHubIntent(ctx context.Context, state string, intent userauthapp.GitHubOAuthIntent, ttl time.Duration) error {
	return cache.SetJSONRequired(ctx, fmt.Sprintf("user_auth:github_state:%s", state), intent, ttl)
}
func (*GitHubStateStore) TakeGitHubIntent(ctx context.Context, state string) (*userauthapp.GitHubOAuthIntent, error) {
	var intent userauthapp.GitHubOAuthIntent
	found, err := cache.GetDelJSONRequired(ctx, fmt.Sprintf("user_auth:github_state:%s", state), &intent)
	if err != nil || !found {
		return nil, err
	}
	return &intent, nil
}
