package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// Errors returned by CustomerAuthService. Handlers map these to HTTP status
// codes; none of them (except ErrInvalidPhone) may vary the response shape
// based on whether a phone number is already registered — see RequestOTP.
var (
	ErrOTPCooldown    = errors.New("otp resend cooldown active")
	ErrOTPRateLimited = errors.New("otp rate limited")
	ErrOTPInvalid     = errors.New("otp code invalid or expired")
	ErrOTPLocked      = errors.New("otp attempts exhausted, request a new code")
	ErrSessionInvalid = errors.New("session invalid, expired, or revoked")
	ErrAccountBlocked = errors.New("customer account blocked")
	ErrInvalidPhone   = auth.ErrInvalidPhone
)

// CustomerAuthTokens is the access/refresh pair returned after a successful
// OTP verify or refresh.
type CustomerAuthTokens struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

// CustomerAuthConfig configures CustomerAuthService. All fields are
// required — see OTPConfig's doc comment on why there are no built-in
// defaults for the anti-abuse settings.
type CustomerAuthConfig struct {
	AccessTokenSecret []byte
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	OTP               OTPConfig
}

// CustomerAuthService implements the mobile-app customer's OTP
// registration/login flow, refresh-token rotation with reuse detection, and
// logout. It never creates a domain.Client itself — that remains the 1C
// accrual webhook's job (see LoyaltyService.resolveOrCreateClient) — it
// only links pre-existing, unlinked Client rows to the newly verified
// CustomerAccount (see VerifyOTP).
//
// OTP challenges live in Redis only (see otp_store.go); refresh tokens are
// persisted in Postgres via CustomerSessionRepository because
// revocation/rotation/replay-detection need durable state a TTL cache
// shouldn't be trusted for.
type CustomerAuthService struct {
	log      *slog.Logger
	accounts domain.CustomerAccountRepository
	sessions domain.CustomerSessionRepository
	consents domain.CustomerConsentRepository
	clients  domain.ClientRepository
	audit    domain.AuditEventRepository
	sms      domain.SMSSender
	otp      *otpStore
	cfg      CustomerAuthConfig
}

// NewCustomerAuthService constructs a CustomerAuthService.
func NewCustomerAuthService(
	log *slog.Logger,
	accounts domain.CustomerAccountRepository,
	sessions domain.CustomerSessionRepository,
	consents domain.CustomerConsentRepository,
	clients domain.ClientRepository,
	audit domain.AuditEventRepository,
	sms domain.SMSSender,
	rdb *redis.Client,
	cfg CustomerAuthConfig,
) *CustomerAuthService {
	return &CustomerAuthService{
		log:      log,
		accounts: accounts,
		sessions: sessions,
		consents: consents,
		clients:  clients,
		audit:    audit,
		sms:      sms,
		otp:      newOTPStore(rdb, cfg.OTP),
		cfg:      cfg,
	}
}

// RequestOTP normalizes phone, applies rate limiting (by phone and by IP)
// and resend cooldown, generates a code via crypto/rand, stores only its
// hash (TTL'd) in Redis, and sends it via SMS. It always succeeds or fails
// the same way regardless of whether phone is already a registered
// CustomerAccount — an unknown phone is not distinguishable from a known
// one in the response, so this endpoint can't be used to enumerate
// accounts.
func (s *CustomerAuthService) RequestOTP(ctx context.Context, rawPhone, ip string) error {
	phone, err := auth.NormalizePhone(rawPhone)
	if err != nil {
		return ErrInvalidPhone
	}

	if err := s.otp.checkCooldown(ctx, phone); err != nil {
		return ErrOTPCooldown
	}
	if err := s.otp.checkRateLimit(ctx, phone, ip); err != nil {
		return ErrOTPRateLimited
	}

	code, err := auth.GenerateOTPCode()
	if err != nil {
		return fmt.Errorf("request otp: generate code: %w", err)
	}

	if err := s.otp.store(ctx, phone, code); err != nil {
		return fmt.Errorf("request otp: store challenge: %w", err)
	}

	if err := s.sms.Send(ctx, phone, fmt.Sprintf("PromoGo: ваш код подтверждения %s", code)); err != nil {
		return fmt.Errorf("request otp: send sms: %w", err)
	}

	s.auditLog(ctx, domain.AuditActorSystem, nil, domain.AuditActionCustomerOTPRequested, nil, ip, "")
	return nil
}

// VerifyOTPRequest is the input to VerifyOTP. ConsentDocumentVersion and
// ConsentSource are empty for a returning customer re-authenticating with a
// document version they already accepted; the HTTP layer only populates
// them on first-time registration (see knowledge/Project Questions.md
// Q-P0-048).
type VerifyOTPRequest struct {
	Phone                  string
	Code                   string
	IP                     string
	UserAgent              string
	ConsentDocumentVersion string
	ConsentSource          string
}

// VerifyOTP checks the OTP code, then finds-or-creates the CustomerAccount
// for phone, links any pre-existing unlinked Client rows to it, records
// consent if provided, and issues a fresh access/refresh token pair.
func (s *CustomerAuthService) VerifyOTP(ctx context.Context, req VerifyOTPRequest) (*CustomerAuthTokens, *domain.CustomerAccount, error) {
	phone, err := auth.NormalizePhone(req.Phone)
	if err != nil {
		return nil, nil, ErrInvalidPhone
	}

	if err := s.otp.verify(ctx, phone, req.Code); err != nil {
		s.auditLog(ctx, domain.AuditActorSystem, nil, domain.AuditActionCustomerOTPFailed, nil, req.IP, req.UserAgent)
		switch {
		case errors.Is(err, errOTPLocked):
			return nil, nil, ErrOTPLocked
		default:
			return nil, nil, ErrOTPInvalid
		}
	}

	account, err := s.resolveOrCreateAccount(ctx, phone)
	if err != nil {
		return nil, nil, fmt.Errorf("verify otp: resolve account: %w", err)
	}
	if account.Status != domain.CustomerAccountActive {
		return nil, nil, ErrAccountBlocked
	}

	if err := s.linkExistingClients(ctx, phone, account.ID); err != nil {
		s.log.WarnContext(ctx, "link existing clients to customer account", "customer_account_id", account.ID, "error", err)
	}

	if req.ConsentDocumentVersion != "" {
		consent := &domain.CustomerConsent{
			CustomerAccountID: account.ID,
			DocumentVersion:   req.ConsentDocumentVersion,
			Source:            req.ConsentSource,
			IP:                req.IP,
			UserAgent:         req.UserAgent,
		}
		if err := s.consents.Create(ctx, consent); err != nil {
			return nil, nil, fmt.Errorf("verify otp: record consent: %w", err)
		}
	}

	tokens, err := s.issueTokens(ctx, account.ID, req.IP, req.UserAgent)
	if err != nil {
		return nil, nil, fmt.Errorf("verify otp: issue tokens: %w", err)
	}

	accountID := account.ID
	s.auditLog(ctx, domain.AuditActorCustomer, &accountID, domain.AuditActionCustomerLogin, nil, req.IP, req.UserAgent)
	return tokens, account, nil
}

func (s *CustomerAuthService) resolveOrCreateAccount(ctx context.Context, phone string) (*domain.CustomerAccount, error) {
	account, err := s.accounts.GetByPhone(ctx, phone)
	if err == nil {
		return account, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	account = &domain.CustomerAccount{
		Phone:           phone,
		PhoneVerifiedAt: time.Now(),
		Status:          domain.CustomerAccountActive,
	}
	if err := s.accounts.Create(ctx, account); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			// Lost a race with a concurrent verify of the same phone.
			return s.accounts.GetByPhone(ctx, phone)
		}
		return nil, err
	}
	return account, nil
}

func (s *CustomerAuthService) linkExistingClients(ctx context.Context, phone string, accountID int64) error {
	clients, err := s.clients.ListUnlinkedByPhone(ctx, phone)
	if err != nil {
		return fmt.Errorf("list unlinked clients: %w", err)
	}
	for _, c := range clients {
		if err := s.clients.LinkCustomerAccount(ctx, c.ID, accountID); err != nil {
			return fmt.Errorf("link client %d: %w", c.ID, err)
		}
	}
	return nil
}

// Refresh rotates refreshToken: the presented token is revoked and a new
// pair is issued. Presenting a token that's already revoked is treated as
// compromise (token reuse) — every session for that CustomerAccountID is
// revoked and the event is audit-logged.
func (s *CustomerAuthService) Refresh(ctx context.Context, refreshToken, ip, userAgent string) (*CustomerAuthTokens, error) {
	hash := auth.HashOpaqueToken(refreshToken)
	session, err := s.sessions.GetByRefreshTokenHash(ctx, hash)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, ErrSessionInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("refresh: load session: %w", err)
	}

	if session.RevokedAt != nil {
		if err := s.sessions.RevokeAllForAccount(ctx, session.CustomerAccountID); err != nil {
			s.log.ErrorContext(ctx, "revoke all sessions after reuse detection", "customer_account_id", session.CustomerAccountID, "error", err)
		}
		accountID := session.CustomerAccountID
		s.auditLog(ctx, domain.AuditActorCustomer, &accountID, domain.AuditActionCustomerRefreshReuse, nil, ip, userAgent)
		return nil, ErrSessionInvalid
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionInvalid
	}

	tokens, newSessionID, err := s.issueTokensReturningID(ctx, session.CustomerAccountID, ip, userAgent)
	if err != nil {
		return nil, fmt.Errorf("refresh: issue tokens: %w", err)
	}
	if err := s.sessions.Rotate(ctx, session.ID, newSessionID); err != nil {
		return nil, fmt.Errorf("refresh: rotate session: %w", err)
	}

	return tokens, nil
}

// Logout revokes the session identified by refreshToken. Idempotent: a
// token that doesn't resolve to a session is treated as already logged out.
func (s *CustomerAuthService) Logout(ctx context.Context, refreshToken string) error {
	hash := auth.HashOpaqueToken(refreshToken)
	session, err := s.sessions.GetByRefreshTokenHash(ctx, hash)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("logout: load session: %w", err)
	}

	if err := s.sessions.Revoke(ctx, session.ID); err != nil {
		return fmt.Errorf("logout: revoke session: %w", err)
	}

	accountID := session.CustomerAccountID
	s.auditLog(ctx, domain.AuditActorCustomer, &accountID, domain.AuditActionCustomerLogout, nil, "", "")
	return nil
}

// LogoutAll revokes every non-revoked session for customerAccountID.
func (s *CustomerAuthService) LogoutAll(ctx context.Context, customerAccountID int64) error {
	if err := s.sessions.RevokeAllForAccount(ctx, customerAccountID); err != nil {
		return fmt.Errorf("logout all: %w", err)
	}
	s.auditLog(ctx, domain.AuditActorCustomer, &customerAccountID, domain.AuditActionCustomerLogoutAll, nil, "", "")
	return nil
}

func (s *CustomerAuthService) issueTokens(ctx context.Context, customerAccountID int64, ip, userAgent string) (*CustomerAuthTokens, error) {
	tokens, _, err := s.issueTokensReturningID(ctx, customerAccountID, ip, userAgent)
	return tokens, err
}

func (s *CustomerAuthService) issueTokensReturningID(ctx context.Context, customerAccountID int64, ip, userAgent string) (*CustomerAuthTokens, int64, error) {
	refreshToken, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, 0, fmt.Errorf("generate refresh token: %w", err)
	}

	now := time.Now()
	session := &domain.CustomerSession{
		CustomerAccountID: customerAccountID,
		RefreshTokenHash:  refreshHash,
		ExpiresAt:         now.Add(s.cfg.RefreshTokenTTL),
		UserAgent:         userAgent,
		IP:                ip,
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, 0, fmt.Errorf("create session: %w", err)
	}

	accessToken, err := auth.IssueCustomerAccessToken(s.cfg.AccessTokenSecret, customerAccountID, s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, 0, fmt.Errorf("issue access token: %w", err)
	}

	return &CustomerAuthTokens{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  now.Add(s.cfg.AccessTokenTTL),
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: session.ExpiresAt,
	}, session.ID, nil
}

func (s *CustomerAuthService) auditLog(ctx context.Context, actorType domain.AuditActorType, actorID *int64, action string, metadata map[string]any, ip, userAgent string) {
	event := &domain.AuditEvent{
		ActorType: actorType,
		ActorID:   actorID,
		Action:    action,
		Metadata:  metadata,
		IP:        ip,
		UserAgent: userAgent,
	}
	if err := s.audit.Create(ctx, event); err != nil {
		s.log.WarnContext(ctx, "write audit event", "action", action, "error", err)
	}
}
