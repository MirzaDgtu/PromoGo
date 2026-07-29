// Package ratelimit provides a Redis-backed, distributed rate limiter and
// an HTTP middleware built on it. Counters live in Redis (never
// process-local memory) so limits hold across every replica of the API,
// and are checked/incremented atomically via a single Lua script so
// concurrent requests can never observe or admit more than the configured
// limit.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// checkAndIncrScript atomically increments the request counter for a
// window and, on the first request of a new window, sets its expiry — a
// single round trip so a crash between "increment" and "set TTL" can never
// leave a key permanently stuck with no expiry. Returns {count, ttl_ms}.
const checkAndIncrScript = `
local n = redis.call("INCR", KEYS[1])
local ttl = redis.call("PTTL", KEYS[1])
if ttl < 0 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
return {n, ttl}
`

// Limiter checks and increments fixed-window request counters in Redis.
type Limiter struct {
	rdb    *redis.Client
	script *redis.Script
}

// New constructs a Limiter backed by rdb. rdb must not be nil.
func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb, script: redis.NewScript(checkAndIncrScript)}
}

// Allow increments the counter for key within window and reports whether
// this request is within limit. On a limiter backend error it returns a
// non-nil error — callers must fail closed (reject the request, typically
// with 503) rather than silently admitting it, since a Redis outage is not
// evidence that traffic is safe to let through unlimited.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error) {
	res, err := l.script.Run(ctx, l.rdb, []string{key}, window.Milliseconds()).Result()
	if err != nil {
		return false, 0, fmt.Errorf("rate limit check: %w", err)
	}

	vals, ok := res.([]interface{})
	if !ok || len(vals) != 2 {
		return false, 0, fmt.Errorf("rate limit check: unexpected script result %#v", res)
	}
	count, ok1 := vals[0].(int64)
	ttlMs, ok2 := vals[1].(int64)
	if !ok1 || !ok2 {
		return false, 0, fmt.Errorf("rate limit check: unexpected script result types %#v", vals)
	}

	if count > int64(limit) {
		return false, time.Duration(ttlMs) * time.Millisecond, nil
	}
	return true, 0, nil
}
