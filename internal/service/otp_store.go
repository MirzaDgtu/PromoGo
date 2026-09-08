package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
)

// OTP challenge state lives in Redis only, never Postgres: a challenge
// that's never verified should simply expire with no durable trace. This
// differs from customer_sessions (Postgres), which need durable revocation
// state — see CustomerAuthService's doc comment.
var (
	errOTPRateLimited = errors.New("otp rate limited")
	errOTPCooldown    = errors.New("otp resend cooldown active")
	errOTPNotFound    = errors.New("otp challenge not found or expired")
	errOTPLocked      = errors.New("otp attempts exhausted")
)

// OTPConfig bounds OTP request/verify behavior. All durations/counts are
// required; there are no built-in defaults, since silently permissive
// values here would undermine the anti-SMS-pumping and brute-force
// protections this exists for.
type OTPConfig struct {
	CodeTTL             time.Duration
	ResendCooldown      time.Duration
	MaxAttempts         int
	RateLimitWindow     time.Duration
	MaxRequestsPerPhone int
	MaxRequestsPerIP    int
}

// incrWithExpiryScript atomically increments KEYS[1] and, only on the
// first increment (n == 1), sets its expiry to ARGV[1] seconds — as one
// round trip, so a crash or network partition between the increment and
// the expire can never leave a rate-limit counter permanently without a
// TTL.
var incrWithExpiryScript = redis.NewScript(`
	local n = redis.call('INCR', KEYS[1])
	if n == 1 then
		redis.call('EXPIRE', KEYS[1], ARGV[1])
	end
	return n
`)

// verifyOTPScript atomically checks a submitted code's hash against the
// stored challenge and applies the resulting state change — the single-use
// delete on success, or the attempt-counter increment (with its own
// lockout-on-exhaustion delete) on failure — in one round trip. Because
// Redis executes each Lua script to completion before starting the next
// command (including another EVALSHA of this same script), two concurrent
// verify attempts against the same challenge can never both observe it
// present: whichever runs first deletes it (on a correct code) or
// increments it (on a wrong one) before the second one's HMGET ever runs,
// closing both the double-consumption race and the lost-attempt-increment
// race that separate HGETALL/HSET/DEL round trips left open.
//
// KEYS[1] = otp:code:<phone hash>
// ARGV[1] = SHA-256 hex hash of the submitted code (see auth.HashOTP)
// ARGV[2] = MaxAttempts
// returns: "ok" | "notfound" | "wrong" | "locked"
var verifyOTPScript = redis.NewScript(`
	local vals = redis.call('HMGET', KEYS[1], 'hash', 'attempts')
	local storedHash = vals[1]
	if not storedHash then
		return "notfound"
	end
	local attempts = tonumber(vals[2]) or 0
	local maxAttempts = tonumber(ARGV[2])
	if attempts >= maxAttempts then
		redis.call('DEL', KEYS[1])
		return "locked"
	end
	if storedHash == ARGV[1] then
		redis.call('DEL', KEYS[1])
		return "ok"
	end
	local newAttempts = attempts + 1
	if newAttempts >= maxAttempts then
		redis.call('DEL', KEYS[1])
		return "locked"
	end
	redis.call('HINCRBY', KEYS[1], 'attempts', 1)
	return "wrong"
`)

// otpStore implements OTP challenge storage over Redis: rate limiting by
// phone and IP, resend cooldown, bounded verify attempts, and hashed code
// storage (crypto/rand-generated codes hashed via internal/auth before
// they ever reach Redis).
type otpStore struct {
	rdb *redis.Client
	cfg OTPConfig
}

func newOTPStore(rdb *redis.Client, cfg OTPConfig) *otpStore {
	return &otpStore{rdb: rdb, cfg: cfg}
}

// phoneKey derives a Redis key from phone without storing the phone number
// itself in the key space (visible to anyone with Redis access).
func phoneKey(prefix, phone string) string {
	return prefix + ":" + auth.HashOpaqueToken(phone)
}

func ipKey(prefix, ip string) string {
	return prefix + ":" + auth.HashOpaqueToken(ip)
}

// checkRateLimit increments per-phone and per-IP request counters for the
// current window and returns errOTPRateLimited if either exceeds its
// configured max. Both counters are incremented even if the phone limit
// alone would already reject, so a caller can't probe the IP limit
// separately by varying the phone.
func (s *otpStore) checkRateLimit(ctx context.Context, phone, ip string) error {
	phoneCount, err := s.incrWithWindow(ctx, phoneKey("otp:rl:phone", phone), s.cfg.RateLimitWindow)
	if err != nil {
		return fmt.Errorf("check phone rate limit: %w", err)
	}
	ipCount, err := s.incrWithWindow(ctx, ipKey("otp:rl:ip", ip), s.cfg.RateLimitWindow)
	if err != nil {
		return fmt.Errorf("check ip rate limit: %w", err)
	}
	if phoneCount > int64(s.cfg.MaxRequestsPerPhone) || ipCount > int64(s.cfg.MaxRequestsPerIP) {
		return errOTPRateLimited
	}
	return nil
}

func (s *otpStore) incrWithWindow(ctx context.Context, key string, window time.Duration) (int64, error) {
	n, err := incrWithExpiryScript.Run(ctx, s.rdb, []string{key}, int64(window.Seconds())).Int64()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// acquireCooldown atomically claims the resend cooldown for phone —
// SET NX means a concurrent RequestOTP for the same phone can never both
// observe no cooldown and proceed; exactly one wins the race, the other
// gets errOTPCooldown. Callers that fail after acquiring it (e.g. SMS send
// failure) should releaseCooldown so a genuinely undelivered code doesn't
// lock the user out for the full cooldown window.
func (s *otpStore) acquireCooldown(ctx context.Context, phone string) error {
	ok, err := s.rdb.SetNX(ctx, phoneKey("otp:cooldown", phone), "1", s.cfg.ResendCooldown).Result()
	if err != nil {
		return fmt.Errorf("acquire otp cooldown: %w", err)
	}
	if !ok {
		return errOTPCooldown
	}
	return nil
}

// releaseCooldown undoes acquireCooldown after a failure that means no
// code was actually delivered, so the caller isn't penalized with a full
// cooldown wait for a request that never reached the user.
func (s *otpStore) releaseCooldown(ctx context.Context, phone string) error {
	if err := s.rdb.Del(ctx, phoneKey("otp:cooldown", phone)).Err(); err != nil {
		return fmt.Errorf("release otp cooldown: %w", err)
	}
	return nil
}

// store persists code's hash for phone and resets the attempt counter.
// Replaces any prior unverified challenge for the same phone (a fresh OTP
// request invalidates an older, unverified one). The resend cooldown is
// acquired separately and earlier (see acquireCooldown) so it gates the
// whole request atomically rather than racing with this write.
func (s *otpStore) store(ctx context.Context, phone, code string) error {
	key := phoneKey("otp:code", phone)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{"hash": auth.HashOTP(code), "attempts": 0})
	pipe.Expire(ctx, key, s.cfg.CodeTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store otp challenge: %w", err)
	}
	return nil
}

// verify checks code against the stored challenge for phone, atomically
// (see verifyOTPScript). A wrong code returns errOTPNotFound (same error
// as no-challenge-exists, so a caller can't distinguish "wrong code" from
// "no challenge" by response shape); once MaxAttempts is exceeded the
// challenge is deleted and errOTPLocked is returned. On a correct code the
// challenge is deleted (single use) and verify succeeds.
func (s *otpStore) verify(ctx context.Context, phone, code string) error {
	key := phoneKey("otp:code", phone)

	result, err := verifyOTPScript.Run(ctx, s.rdb, []string{key}, auth.HashOTP(code), s.cfg.MaxAttempts).Text()
	if err != nil {
		return fmt.Errorf("verify otp challenge: %w", err)
	}

	switch result {
	case "ok":
		return nil
	case "locked":
		return errOTPLocked
	case "notfound", "wrong":
		return errOTPNotFound
	default:
		return fmt.Errorf("verify otp challenge: unexpected script result %q", result)
	}
}
