package cachestore

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRegistrationEmailLimiter(t *testing.T) {
	socket := os.Getenv("TEST_REGISTRATION_REDIS_SOCKET")
	if socket == "" {
		t.Skip("requires an isolated Redis Unix socket")
	}
	client := redis.NewClient(&redis.Options{Network: "unix", Addr: socket})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(t.Context()).Err(); err != nil {
		t.Fatal(err)
	}
	limiter := NewRegistrationEmailLimiter(client, fmt.Sprintf("test:%d", time.Now().UnixNano()))
	const cooldown = time.Minute

	attempt := func(ip, email string, allowed bool) func() {
		t.Helper()
		wait, release, err := limiter.Reserve(t.Context(), ip, email, cooldown)
		if err != nil || (wait == 0) != allowed || (release != nil) != allowed {
			t.Fatalf("reserve %s: wait=%s allowed=%t err=%v", email, wait, release != nil, err)
		}
		return release
	}

	ip := "192.0.2.1"
	release := attempt(ip, "a@example.test", true)
	attempt("192.0.2.2", "A <A@example.test>", false)
	release()
	attempt(ip, "b@example.test", true)()
	attempt(ip, "c@example.test", true)()
	attempt(ip, "d@example.test", false)
	attempt(ip, "a@example.test", true)()
	attempt(ip, "b@example.test", true)()
	attempt("::ffff:"+ip, "a@example.test", false)
	attempt("192.0.2.2", "d@example.test", true)()

	// Age only the fixture attempts to verify that an expired window reopens.
	now, err := client.Time(t.Context()).Result()
	if err != nil {
		t.Fatal(err)
	}
	key := limiter.prefix + "ip:" + ip
	members, err := client.ZRange(t.Context(), key, 0, -1).Result()
	if err != nil || len(members) != 5 {
		t.Fatalf("fixture window: count=%d err=%v", len(members), err)
	}
	for _, member := range members {
		if err := client.ZAdd(t.Context(), key, redis.Z{Score: float64(now.Add(-registrationEmailWindow).UnixMilli()), Member: member}).Err(); err != nil {
			t.Fatal(err)
		}
	}
	attempt(ip, "e@example.test", true)()

	const concurrentIP = "192.0.2.3"
	var workers sync.WaitGroup
	results := make(chan bool, 12)
	for i := range cap(results) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			email := fmt.Sprintf("concurrent-%d@example.test", i)
			wait, _, err := limiter.Reserve(t.Context(), concurrentIP, email, cooldown)
			if err != nil {
				t.Errorf("concurrent reservation: %v", err)
			}
			results <- err == nil && wait == 0
		}()
	}
	workers.Wait()
	close(results)
	admitted := 0
	for allowed := range results {
		if allowed {
			admitted++
		}
	}
	if admitted != registrationEmailMaxRecipients {
		t.Fatalf("concurrent admission allowed %d recipients", admitted)
	}
}
