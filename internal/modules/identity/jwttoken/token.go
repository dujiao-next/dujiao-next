package jwttoken

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TypeAccess             = "access"
	TypeTwoFactorChallenge = "2fa_challenge"
)

// ErrSessionExpired 表示会话已达绝对生命周期上限，续期被拒绝，必须重新登录。
var ErrSessionExpired = errors.New("session lifetime exceeded")

// IsAccessType accepts an empty type for tokens issued before typ was added.
func IsAccessType(tokenType string) bool {
	return tokenType == "" || tokenType == TypeAccess
}

// NewHS256Parser rejects algorithm-confusion attempts before claims parsing.
func NewHS256Parser() *jwt.Parser {
	return jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
}
