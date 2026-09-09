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

// QR token state lives in Redis only, like OTP challenges (see
// otp_store.go) — a never-scanned QR should simply expire with no durable
// trace, and DEC-011 requires the token itself never touch Postgres.
var (
	errQRIssueCooldown   = errors.New("qr issue cooldown active")
	errQRConsumeCooldown = errors.New("qr consume cooldown active")
	errQRGone            = errors.New("qr token expired, consumed, or unknown")
)

// qrClaimTTL bounds how long a claimed-but-not-yet-finalized/released QR
// token can stay in limbo — see claim's doc comment. Not user-configurable:
// it only needs to comfortably exceed how long the Postgres work between
// claim and finalize/release can take, not reflect any product-visible
// timing.
const qrClaimTTL = 30 * time.Second

// QRConfig bounds QR issuance/consumption behavior (DEC-011). Like
// AntiFraudConfig, this is a narrow service-local view of
// config.AntiFraudConfig's QR fields — internal/app/app.go maps them in.
type QRConfig struct {
	TTL             time.Duration
	IssueCooldown   time.Duration
	ConsumeCooldown time.Duration
}

// qrStore implements QR token storage over Redis: a versioned opaque
// payload whose hash is the only thing ever persisted, one-time atomic
// consume via GETDEL, and independent issue/consume cooldowns.
type qrStore struct {
	rdb *redis.Client
	cfg QRConfig
}

func newQRStore(rdb *redis.Client, cfg QRConfig) *qrStore {
	return &qrStore{rdb: rdb, cfg: cfg}
}

// qrPayloadVersion prefixes every issued payload so a future format change
// is detectable without breaking already-issued QRs.
const qrPayloadVersion = "v1."

func qrTokenKey(tokenHash string) string {
	return "qr:token:" + tokenHash
}

func qrIssueCooldownKey(customerAccountID int64) string {
	// CustomerAccountID is an internal numeric identifier, not phone/IP-class
	// PII — used raw as Redis key material, matching principalRule's
	// treatment of StaffUserID/CustomerAccountID elsewhere in this codebase
	// (see internal/httpserver/ratelimit_rules.go).
	return "qr:issue-cooldown:" + strconv.FormatInt(customerAccountID, 10)
}

func qrConsumeCooldownKey(principal string) string {
	return "qr:consume-cooldown:" + auth.HashOpaqueToken(principal)
}

func qrClaimKey(tokenHash string) string {
	return "qr:claim:" + tokenHash
}

// issue mints a new QR payload for customerAccountID, or returns
// errQRIssueCooldown (with the remaining TTL) if one was already issued
// within cfg.IssueCooldown. SetNX atomically claims the cooldown key —
// unlike a separate TTL-check-then-SET, two concurrent issue calls for the
// same account can never both observe no cooldown and both mint a token.
func (s *qrStore) issue(ctx context.Context, customerAccountID int64) (payload string, expiresAt time.Time, retryAfter time.Duration, err error) {
	// IssueCooldown<=0 means "no cooldown enforced" (matching
	// checkConsumeCooldown's identical guard for ConsumeCooldown) — skip
	// the cooldown key entirely rather than SetNX-ing a key with no
	// expiration, which would permanently lock out every issuance after
	// the first.
	if s.cfg.IssueCooldown > 0 {
		cooldownKey := qrIssueCooldownKey(customerAccountID)
		ok, err := s.rdb.SetNX(ctx, cooldownKey, "1", s.cfg.IssueCooldown).Result()
		if err != nil {
			return "", time.Time{}, 0, fmt.Errorf("acquire qr issue cooldown: %w", err)
		}
		if !ok {
			ttl, err := s.rdb.TTL(ctx, cooldownKey).Result()
			if err != nil {
				return "", time.Time{}, 0, fmt.Errorf("check qr issue cooldown: %w", err)
			}
			return "", time.Time{}, ttl, errQRIssueCooldown
		}
		defer func() {
			// Release the cooldown we just acquired if issuance didn't
			// actually complete — the caller shouldn't be penalized with a
			// full cooldown wait for a request that produced no token.
			if err != nil {
				s.rdb.Del(ctx, cooldownKey)
			}
		}()
	}

	token, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("generate qr token: %w", err)
	}

	if err := s.rdb.Set(ctx, qrTokenKey(hash), strconv.FormatInt(customerAccountID, 10), s.cfg.TTL).Err(); err != nil {
		return "", time.Time{}, 0, fmt.Errorf("store qr token: %w", err)
	}

	return qrPayloadVersion + token, time.Now().Add(s.cfg.TTL), 0, nil
}

// checkConsumeCooldown enforces a minimum interval between resolve-QR
// attempts from the same principal (see qr.go's resolveQRConsumePrincipal
// for what identifies "same") — independent of the token's own one-time-use
// property, this bounds how fast a caller can try candidate tokens
// (DEC-011). SetNX makes the check-and-acquire atomic: two concurrent
// attempts can never both observe no cooldown and both proceed.
func (s *qrStore) checkConsumeCooldown(ctx context.Context, principal string) (retryAfter time.Duration, err error) {
	if s.cfg.ConsumeCooldown <= 0 {
		return 0, nil
	}
	key := qrConsumeCooldownKey(principal)
	ok, err := s.rdb.SetNX(ctx, key, "1", s.cfg.ConsumeCooldown).Result()
	if err != nil {
		return 0, fmt.Errorf("acquire qr consume cooldown: %w", err)
	}
	if !ok {
		ttl, err := s.rdb.TTL(ctx, key).Result()
		if err != nil {
			return 0, fmt.Errorf("check qr consume cooldown: %w", err)
		}
		return ttl, errQRConsumeCooldown
	}
	return 0, nil
}

// qrClaimScript atomically moves a token from its active key to a
// time-bounded claim key, in one round trip — see claim's doc comment.
// KEYS[1] = qr:token:<hash>, KEYS[2] = qr:claim:<hash>, ARGV[1] = claim TTL
// in milliseconds. Returns the stored value, or false (surfaced by
// go-redis as redis.Nil) if the token key didn't exist.
var qrClaimScript = redis.NewScript(`
	local val = redis.call('GET', KEYS[1])
	if not val then
		return false
	end
	redis.call('DEL', KEYS[1])
	redis.call('SET', KEYS[2], val, 'PX', ARGV[1])
	return val
`)

// qrReleaseScript atomically moves a token back from its claim key to its
// active key, preserving whatever time remains on the claim (falling back
// to ARGV[1] if the claim key had no discoverable TTL). KEYS[1] =
// qr:claim:<hash>, KEYS[2] = qr:token:<hash>, ARGV[1] = fallback TTL in
// milliseconds. Returns the stored value, or false if the claim key was
// already gone (already finalized, or its own TTL already expired) — not
// an error, just nothing to release.
var qrReleaseScript = redis.NewScript(`
	local val = redis.call('GET', KEYS[1])
	if not val then
		return false
	end
	local ttl = redis.call('PTTL', KEYS[1])
	if ttl <= 0 then
		ttl = ARGV[1]
	end
	redis.call('DEL', KEYS[1])
	redis.call('SET', KEYS[2], val, 'PX', ttl)
	return val
`)

// claim begins consuming payload: atomically moves the token from "active"
// to "claimed" (see qrClaimScript) and returns the CustomerAccountID it
// names, or errQRGone if it was never issued, already claimed, expired, or
// already fully consumed. Unlike a plain GETDEL, this does not destroy the
// token outright — the caller must follow up with exactly one of finalize
// (permanently consume it — the resolve succeeded, or failed for a reason
// that will never succeed on retry) or release (return it to "active" —
// the resolve failed for a transient/infrastructure reason, so the same QR
// should remain usable). A claim nobody follows up on (e.g. the process
// crashes) self-heals: the claim key expires after qrClaimTTL and the
// token is simply gone, rather than usable forever.
func (s *qrStore) claim(ctx context.Context, payload string) (customerAccountID int64, tokenHash string, err error) {
	token, ok := parseQRPayload(payload)
	if !ok {
		return 0, "", errQRMalformed
	}
	hash := auth.HashOpaqueToken(token)

	result, err := qrClaimScript.Run(ctx, s.rdb, []string{qrTokenKey(hash), qrClaimKey(hash)}, qrClaimTTL.Milliseconds()).Result()
	if errors.Is(err, redis.Nil) {
		return 0, "", errQRGone
	}
	if err != nil {
		return 0, "", fmt.Errorf("claim qr token: %w", err)
	}

	val, ok := result.(string)
	if !ok {
		return 0, "", fmt.Errorf("claim qr token: unexpected script result type %T", result)
	}
	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("claim qr token: corrupt stored value: %w", err)
	}
	return id, hash, nil
}

// finalize permanently consumes a claimed token — call after a successful
// resolve, or a failure that would never succeed on retry (e.g. the
// account the token names no longer exists). Deleting an already-expired
// claim key is a harmless no-op.
func (s *qrStore) finalize(ctx context.Context, tokenHash string) error {
	if err := s.rdb.Del(ctx, qrClaimKey(tokenHash)).Err(); err != nil {
		return fmt.Errorf("finalize qr token: %w", err)
	}
	return nil
}

// release returns a claimed token to "active" so the same QR can be
// retried — call after a transient/infrastructure failure (a database
// error, not a business-logic rejection) partway through resolving it. A
// no-op if the claim already expired or was already finalized.
func (s *qrStore) release(ctx context.Context, tokenHash string) error {
	_, err := qrReleaseScript.Run(ctx, s.rdb, []string{qrClaimKey(tokenHash), qrTokenKey(tokenHash)}, qrClaimTTL.Milliseconds()).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("release qr token: %w", err)
	}
	return nil
}

// errQRMalformed is returned when payload doesn't even parse as a
// well-formed QR token — distinguished from errQRGone (400 vs 410) because
// it's a pure input-shape check that never touches Redis, so it can't leak
// anything about token state.
var errQRMalformed = errors.New("malformed qr payload")

// parseQRPayload strips and validates the version prefix, returning the raw
// token to hash and look up.
func parseQRPayload(payload string) (token string, ok bool) {
	if len(payload) <= len(qrPayloadVersion) || payload[:len(qrPayloadVersion)] != qrPayloadVersion {
		return "", false
	}
	return payload[len(qrPayloadVersion):], true
}
