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

// issue mints a new QR payload for customerAccountID, or returns
// errQRIssueCooldown (with the remaining TTL) if one was already issued
// within cfg.IssueCooldown.
func (s *qrStore) issue(ctx context.Context, customerAccountID int64) (payload string, expiresAt time.Time, retryAfter time.Duration, err error) {
	cooldownKey := qrIssueCooldownKey(customerAccountID)
	ttl, err := s.rdb.TTL(ctx, cooldownKey).Result()
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("check qr issue cooldown: %w", err)
	}
	if ttl > 0 {
		return "", time.Time{}, ttl, errQRIssueCooldown
	}

	token, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("generate qr token: %w", err)
	}

	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, qrTokenKey(hash), strconv.FormatInt(customerAccountID, 10), s.cfg.TTL)
	pipe.Set(ctx, cooldownKey, "1", s.cfg.IssueCooldown)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", time.Time{}, 0, fmt.Errorf("store qr token: %w", err)
	}

	return qrPayloadVersion + token, time.Now().Add(s.cfg.TTL), 0, nil
}

// checkConsumeCooldown enforces a minimum interval between resolve-QR
// attempts from the same store principal — independent of the token's own
// one-time-use property, this bounds how fast a caller can try candidate
// tokens (DEC-011).
func (s *qrStore) checkConsumeCooldown(ctx context.Context, principal string) (retryAfter time.Duration, err error) {
	if s.cfg.ConsumeCooldown <= 0 {
		return 0, nil
	}
	key := qrConsumeCooldownKey(principal)
	ttl, err := s.rdb.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("check qr consume cooldown: %w", err)
	}
	if ttl > 0 {
		return ttl, errQRConsumeCooldown
	}
	if err := s.rdb.Set(ctx, key, "1", s.cfg.ConsumeCooldown).Err(); err != nil {
		return 0, fmt.Errorf("set qr consume cooldown: %w", err)
	}
	return 0, nil
}

// consume atomically retrieves and deletes the CustomerAccountID for
// payload, or returns errQRGone. A single Redis GETDEL is what makes this
// safe under concurrent resolve attempts against the same token — exactly
// one caller observes the value, every other sees a miss (see qr.go's
// ResolveQR for why a miss can't and shouldn't distinguish expired from
// already-consumed from never-issued).
func (s *qrStore) consume(ctx context.Context, payload string) (customerAccountID int64, err error) {
	token, ok := parseQRPayload(payload)
	if !ok {
		return 0, errQRMalformed
	}

	hash := auth.HashOpaqueToken(token)
	val, err := s.rdb.GetDel(ctx, qrTokenKey(hash)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, errQRGone
	}
	if err != nil {
		return 0, fmt.Errorf("consume qr token: %w", err)
	}

	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("consume qr token: corrupt stored value: %w", err)
	}
	return id, nil
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
