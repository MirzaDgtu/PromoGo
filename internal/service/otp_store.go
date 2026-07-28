package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		if err := s.rdb.Expire(ctx, key, window).Err(); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// checkCooldown returns errOTPCooldown if a resend was already sent to
// phone within ResendCooldown.
func (s *otpStore) checkCooldown(ctx context.Context, phone string) error {
	exists, err := s.rdb.Exists(ctx, phoneKey("otp:cooldown", phone)).Result()
	if err != nil {
		return fmt.Errorf("check otp cooldown: %w", err)
	}
	if exists > 0 {
		return errOTPCooldown
	}
	return nil
}

// store persists code's hash for phone, resets the attempt counter, and
// sets the resend cooldown. Replaces any prior unverified challenge for the
// same phone (a fresh OTP request invalidates an older, unverified one).
func (s *otpStore) store(ctx context.Context, phone, code string) error {
	key := phoneKey("otp:code", phone)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{"hash": auth.HashOTP(code), "attempts": 0})
	pipe.Expire(ctx, key, s.cfg.CodeTTL)
	pipe.Set(ctx, phoneKey("otp:cooldown", phone), "1", s.cfg.ResendCooldown)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store otp challenge: %w", err)
	}
	return nil
}

// verify checks code against the stored challenge for phone. On a wrong
// code it increments the attempt counter and returns errOTPNotFound (same
// error as no-challenge-exists, so a caller can't distinguish "wrong code"
// from "no challenge" by response shape); once MaxAttempts is exceeded the
// challenge is deleted and errOTPLocked is returned. On a correct code the
// challenge is deleted (single use) and verify succeeds.
func (s *otpStore) verify(ctx context.Context, phone, code string) error {
	key := phoneKey("otp:code", phone)

	vals, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("load otp challenge: %w", err)
	}
	if len(vals) == 0 {
		return errOTPNotFound
	}

	attempts, _ := strconv.Atoi(vals["attempts"])
	if attempts >= s.cfg.MaxAttempts {
		s.rdb.Del(ctx, key)
		return errOTPLocked
	}

	if !auth.VerifyOTP(code, vals["hash"]) {
		newAttempts := attempts + 1
		if newAttempts >= s.cfg.MaxAttempts {
			s.rdb.Del(ctx, key)
			return errOTPLocked
		}
		s.rdb.HSet(ctx, key, "attempts", newAttempts)
		return errOTPNotFound
	}

	s.rdb.Del(ctx, key)
	return nil
}
