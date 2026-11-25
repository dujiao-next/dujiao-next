package cachestore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net/mail"
	"net/netip"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/redis/go-redis/v9"
)

const (
	registrationEmailWindow        = 10 * time.Minute
	registrationEmailMaxRecipients = 3
	registrationEmailMaxSends      = 5
)

// Each member is a recipient digest and a unique send ID. The sorted set
// stores at most five attempts for a source IP and uses a rolling window.
var reserveRegistrationEmail = redis.NewScript(`
local clock = redis.call('TIME')
local now = tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000)
local window = tonumber(ARGV[3])
local maxRecipients = tonumber(ARGV[4])
local maxSends = tonumber(ARGV[5])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - window)
local attempts = redis.call('ZRANGE', KEYS[1], 0, -1, 'WITHSCORES')
local recipients = {}
local count = 0
for i = 1, #attempts, 2 do
    local recipient = string.sub(attempts[i], 1, 64)
    if recipients[recipient] == nil then
        count = count + 1
    end
    recipients[recipient] = tonumber(attempts[i + 1])
end
local wait = math.max(0, redis.call('PTTL', KEYS[2]))
if #attempts / 2 >= maxSends then
    wait = math.max(wait, tonumber(attempts[2]) + window - now)
end
if count >= maxRecipients and recipients[ARGV[1]] == nil then
    local earliest = now
    for _, lastSend in pairs(recipients) do
        earliest = math.min(earliest, lastSend)
    end
    wait = math.max(wait, earliest + window - now)
end
if wait > 0 then
    return wait
end
redis.call('SET', KEYS[2], ARGV[2], 'PX', ARGV[6])
redis.call('ZADD', KEYS[1], now, ARGV[1] .. ':' .. ARGV[2])
redis.call('PEXPIRE', KEYS[1], window)
return 0
`)

var releaseRegistrationEmail = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
end
return 0
`)

type RegistrationEmailLimiter struct {
	client *redis.Client
	prefix string
}

func NewRegistrationEmailLimiter(client *redis.Client, prefix string) *RegistrationEmailLimiter {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = constants.RedisPrefixDefault
	}
	return &RegistrationEmailLimiter{client: client, prefix: prefix + ":registration_email:"}
}

func (l *RegistrationEmailLimiter) Reserve(ctx context.Context, clientIP, email string, cooldown time.Duration) (time.Duration, func(), error) {
	ip, err := netip.ParseAddr(clientIP)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid registration client address: %w", err)
	}
	address, err := mail.ParseAddress(email)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid registration recipient: %w", err)
	}
	recipient := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(address.Address))))
	ipKey := l.prefix + "ip:" + ip.Unmap().String()
	emailKey := l.prefix + "recipient:" + recipient
	sendID := rand.Text()
	wait, err := reserveRegistrationEmail.Run(ctx, l.client, []string{ipKey, emailKey}, recipient, sendID,
		registrationEmailWindow.Milliseconds(), registrationEmailMaxRecipients, registrationEmailMaxSends, cooldown.Milliseconds()).Int64()
	if err != nil {
		return 0, nil, fmt.Errorf("reserve registration email: %w", err)
	}
	if wait > 0 {
		return time.Duration(wait) * time.Millisecond, nil, nil
	}
	return 0, func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if err := releaseRegistrationEmail.Run(cleanup, l.client, []string{emailKey}, sendID).Err(); err != nil {
			logger.Warnw("registration_email_cooldown_release_failed", "error", err)
		}
	}, nil
}
