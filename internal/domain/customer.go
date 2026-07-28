package domain

import (
	"context"
	"time"
)

// CustomerAccountStatus is the lifecycle state of a CustomerAccount.
type CustomerAccountStatus string

const (
	CustomerAccountActive  CustomerAccountStatus = "active"
	CustomerAccountBlocked CustomerAccountStatus = "blocked"
	CustomerAccountDeleted CustomerAccountStatus = "deleted"
)

// CustomerAccount is the global, phone-verified identity of a mobile-app
// customer — created only after a successful OTP verification
// (internal/service/customerauth.go), never implicitly by the 1C accrual
// webhook (see Client.CustomerAccountID). It is not scoped to any one
// store: one CustomerAccount can be linked to a Client in every store the
// phone has ever transacted at.
type CustomerAccount struct {
	ID              int64
	Phone           string
	PhoneVerifiedAt time.Time
	Status          CustomerAccountStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CustomerAccountRepository persists and retrieves CustomerAccount rows.
type CustomerAccountRepository interface {
	// GetByPhone returns domain.ErrNotFound if no account with that phone
	// exists.
	GetByPhone(ctx context.Context, phone string) (*CustomerAccount, error)
	GetByID(ctx context.Context, id int64) (*CustomerAccount, error)
	// Create returns domain.ErrConflict if phone is already registered.
	Create(ctx context.Context, account *CustomerAccount) error
}

// CustomerSession is one issued refresh token. Only RefreshTokenHash
// (SHA-256 of the opaque token) is stored, never the token itself — see
// internal/auth. RevokedAt non-nil means the token can no longer be used to
// refresh; ReplacedByID non-nil (set alongside RevokedAt on rotation) points
// at the session that replaced it, forming a chain used for reuse
// detection: presenting an already-revoked token is treated as compromise
// and revokes every session for that CustomerAccountID.
type CustomerSession struct {
	ID                int64
	CustomerAccountID int64
	RefreshTokenHash  string
	ReplacedByID      *int64
	IssuedAt          time.Time
	ExpiresAt         time.Time
	RevokedAt         *time.Time
	UserAgent         string
	IP                string
}

// CustomerSessionRepository persists and retrieves CustomerSession rows.
type CustomerSessionRepository interface {
	Create(ctx context.Context, session *CustomerSession) error
	// GetByRefreshTokenHash returns domain.ErrNotFound if no session has
	// that hash.
	GetByRefreshTokenHash(ctx context.Context, hash string) (*CustomerSession, error)
	// Rotate atomically marks sessionID revoked (RevokedAt = now,
	// ReplacedByID = replacedByID) — the write side of refresh-token
	// rotation, paired with Create of the new session.
	Rotate(ctx context.Context, sessionID, replacedByID int64) error
	// Revoke marks sessionID revoked without a replacement (plain logout).
	Revoke(ctx context.Context, sessionID int64) error
	// RevokeAllForAccount revokes every non-revoked session for
	// customerAccountID — logout-all, and the response to detected refresh-
	// token reuse (see CustomerSession's doc comment).
	RevokeAllForAccount(ctx context.Context, customerAccountID int64) error
}

// CustomerConsent is an immutable record of a customer granting consent to
// process personal data, captured once per grant (152-FZ). Never updated in
// place: a changed consent is a new row.
type CustomerConsent struct {
	ID                int64
	CustomerAccountID int64
	DocumentVersion   string
	GrantedAt         time.Time
	Source            string
	IP                string
	UserAgent         string
}

// CustomerConsentRepository persists CustomerConsent rows.
type CustomerConsentRepository interface {
	Create(ctx context.Context, consent *CustomerConsent) error
}
