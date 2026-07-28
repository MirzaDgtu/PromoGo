package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// fakeCustomerAccountRepo is an in-memory domain.CustomerAccountRepository.
// createConflictOnce mirrors fakeClientRepo's race-simulation helper (see
// loyalty_test.go): the next Create for that phone returns
// domain.ErrConflict but still persists an account, as if a concurrent
// goroutine's Create had won the race first.
type fakeCustomerAccountRepo struct {
	mu                 sync.Mutex
	byID               map[int64]*domain.CustomerAccount
	nextID             int64
	createConflictOnce map[string]bool
}

func newFakeCustomerAccountRepo() *fakeCustomerAccountRepo {
	return &fakeCustomerAccountRepo{byID: map[int64]*domain.CustomerAccount{}, createConflictOnce: map[string]bool{}}
}

func (f *fakeCustomerAccountRepo) GetByPhone(_ context.Context, phone string) (*domain.CustomerAccount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.byID {
		if a.Phone == phone {
			return a, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeCustomerAccountRepo) GetByID(_ context.Context, id int64) (*domain.CustomerAccount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (f *fakeCustomerAccountRepo) Create(_ context.Context, acc *domain.CustomerAccount) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createConflictOnce[acc.Phone] {
		delete(f.createConflictOnce, acc.Phone)
		f.nextID++
		winner := &domain.CustomerAccount{
			ID: f.nextID, Phone: acc.Phone, PhoneVerifiedAt: acc.PhoneVerifiedAt,
			Status: acc.Status, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		f.byID[winner.ID] = winner
		return domain.ErrConflict
	}

	f.nextID++
	acc.ID = f.nextID
	acc.CreatedAt = time.Now()
	acc.UpdatedAt = time.Now()
	f.byID[acc.ID] = acc
	return nil
}

func (f *fakeCustomerAccountRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byID)
}

// fakeCustomerSessionRepo is an in-memory domain.CustomerSessionRepository.
type fakeCustomerSessionRepo struct {
	mu     sync.Mutex
	byID   map[int64]*domain.CustomerSession
	nextID int64
}

func newFakeCustomerSessionRepo() *fakeCustomerSessionRepo {
	return &fakeCustomerSessionRepo{byID: map[int64]*domain.CustomerSession{}}
}

func (f *fakeCustomerSessionRepo) Create(_ context.Context, s *domain.CustomerSession) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	s.ID = f.nextID
	s.IssuedAt = time.Now()
	cp := *s
	f.byID[s.ID] = &cp
	return nil
}

func (f *fakeCustomerSessionRepo) GetByRefreshTokenHash(_ context.Context, hash string) (*domain.CustomerSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.byID {
		if s.RefreshTokenHash == hash {
			cp := *s
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeCustomerSessionRepo) Rotate(_ context.Context, sessionID, replacedByID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[sessionID]
	if !ok {
		return domain.ErrNotFound
	}
	now := time.Now()
	s.RevokedAt = &now
	s.ReplacedByID = &replacedByID
	return nil
}

func (f *fakeCustomerSessionRepo) Revoke(_ context.Context, sessionID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[sessionID]
	if !ok {
		return nil
	}
	if s.RevokedAt == nil {
		now := time.Now()
		s.RevokedAt = &now
	}
	return nil
}

func (f *fakeCustomerSessionRepo) RevokeAllForAccount(_ context.Context, accountID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.byID {
		if s.CustomerAccountID == accountID && s.RevokedAt == nil {
			now := time.Now()
			s.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeCustomerSessionRepo) get(id int64) *domain.CustomerSession {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *f.byID[id]
	return &cp
}

func (f *fakeCustomerSessionRepo) countNonRevoked(accountID int64) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, s := range f.byID {
		if s.CustomerAccountID == accountID && s.RevokedAt == nil {
			n++
		}
	}
	return n
}

// fakeCustomerConsentRepo is an in-memory domain.CustomerConsentRepository.
type fakeCustomerConsentRepo struct {
	mu      sync.Mutex
	created []*domain.CustomerConsent
}

func (f *fakeCustomerConsentRepo) Create(_ context.Context, c *domain.CustomerConsent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c.ID = int64(len(f.created) + 1)
	c.GrantedAt = time.Now()
	f.created = append(f.created, c)
	return nil
}

func (f *fakeCustomerConsentRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.created)
}

// fakeAuditEventRepo is an in-memory domain.AuditEventRepository.
type fakeAuditEventRepo struct {
	mu     sync.Mutex
	events []*domain.AuditEvent
}

func (f *fakeAuditEventRepo) Create(_ context.Context, e *domain.AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e.ID = int64(len(f.events) + 1)
	e.OccurredAt = time.Now()
	f.events = append(f.events, e)
	return nil
}

func (f *fakeAuditEventRepo) ListByOrganization(_ context.Context, _ int64, _ int) ([]*domain.AuditEvent, error) {
	return nil, nil
}

func (f *fakeAuditEventRepo) actions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.events))
	for i, e := range f.events {
		out[i] = e.Action
	}
	return out
}

// fakeSMSSender is an in-memory domain.SMSSender that captures sent
// messages for test assertions. Real code must never log an OTP message
// body (see internal/notification/logsms) — that constraint is about
// production logging, not about a test double inspecting its own captured
// state to extract the code it needs to complete a test flow.
type fakeSMSSender struct {
	mu       sync.Mutex
	messages []string
}

var otpCodePattern = regexp.MustCompile(`\d{6}`)

func (f *fakeSMSSender) Send(_ context.Context, _, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, message)
	return nil
}

func (f *fakeSMSSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.messages)
}

// lastCode extracts the 6-digit OTP code from the most recently sent
// message.
func (f *fakeSMSSender) lastCode(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.messages) == 0 {
		t.Fatal("no sms sent yet")
	}
	code := otpCodePattern.FindString(f.messages[len(f.messages)-1])
	if code == "" {
		t.Fatalf("no 6-digit code found in sms message %q", f.messages[len(f.messages)-1])
	}
	return code
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type customerAuthTestDeps struct {
	svc      *CustomerAuthService
	mr       *miniredis.Miniredis
	clients  *fakeClientRepo
	accounts *fakeCustomerAccountRepo
	sessions *fakeCustomerSessionRepo
	consents *fakeCustomerConsentRepo
	audit    *fakeAuditEventRepo
	sms      *fakeSMSSender
}

func newCustomerAuthTestDeps(t *testing.T, otpCfg OTPConfig) *customerAuthTestDeps {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	clients := newFakeClientRepo()
	accounts := newFakeCustomerAccountRepo()
	sessions := newFakeCustomerSessionRepo()
	consents := &fakeCustomerConsentRepo{}
	audit := &fakeAuditEventRepo{}
	sms := &fakeSMSSender{}

	cfg := CustomerAuthConfig{
		AccessTokenSecret: []byte("test-secret-at-least-32-bytes-long!"),
		AccessTokenTTL:    time.Minute,
		RefreshTokenTTL:   time.Hour,
		OTP:               otpCfg,
	}

	svc := NewCustomerAuthService(discardLogger(), accounts, sessions, consents, clients, audit, sms, rdb, cfg)

	return &customerAuthTestDeps{svc: svc, mr: mr, clients: clients, accounts: accounts, sessions: sessions, consents: consents, audit: audit, sms: sms}
}

func defaultOTPConfig() OTPConfig {
	return OTPConfig{
		CodeTTL:             2 * time.Second,
		ResendCooldown:      10 * time.Millisecond,
		MaxAttempts:         3,
		RateLimitWindow:     time.Hour,
		MaxRequestsPerPhone: 2,
		MaxRequestsPerIP:    10,
	}
}

func TestCustomerAuth_OTPRequestVerifyHappyPath(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()
	const phone = "+70000000101"

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP() error = %v", err)
	}
	if got := deps.sms.count(); got != 1 {
		t.Fatalf("sms sent count = %d, want 1", got)
	}
	code := deps.sms.lastCode(t)

	tokens, account, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{
		Phone: phone, Code: code, IP: "1.2.3.4", UserAgent: "test-agent",
		ConsentDocumentVersion: "v1", ConsentSource: "mobile_app",
	})
	if err != nil {
		t.Fatalf("VerifyOTP() error = %v", err)
	}
	if account.Phone != phone {
		t.Errorf("account.Phone = %q, want %q", account.Phone, phone)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Error("VerifyOTP() returned empty tokens")
	}
	if deps.consents.count() != 1 {
		t.Errorf("consents recorded = %d, want 1", deps.consents.count())
	}

	actions := deps.audit.actions()
	if len(actions) == 0 || actions[len(actions)-1] != domain.AuditActionCustomerLogin {
		t.Errorf("audit actions = %v, want last entry %q", actions, domain.AuditActionCustomerLogin)
	}
}

func TestCustomerAuth_VerifyOTPWrongCodeRejected(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()
	const phone = "+70000000102"

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP() error = %v", err)
	}

	_, _, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: "000000", IP: "1.2.3.4"})
	if !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("VerifyOTP() with wrong code error = %v, want ErrOTPInvalid", err)
	}
}

func TestCustomerAuth_OTPExpiresAfterTTL(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()
	const phone = "+70000000103"

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP() error = %v", err)
	}
	code := deps.sms.lastCode(t)

	deps.mr.FastForward(3 * time.Second) // > CodeTTL (2s)

	_, _, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: code, IP: "1.2.3.4"})
	if !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("VerifyOTP() after TTL expiry error = %v, want ErrOTPInvalid", err)
	}
}

func TestCustomerAuth_OTPResendCooldown(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()
	const phone = "+70000000104"

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("first RequestOTP() error = %v", err)
	}
	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); !errors.Is(err, ErrOTPCooldown) {
		t.Fatalf("immediate second RequestOTP() error = %v, want ErrOTPCooldown", err)
	}

	deps.mr.FastForward(20 * time.Millisecond) // > ResendCooldown (10ms)

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP() after cooldown elapsed error = %v", err)
	}
}

func TestCustomerAuth_OTPRateLimitedByPhone(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig()) // MaxRequestsPerPhone: 2
	ctx := context.Background()
	const phone = "+70000000105"

	for i := 0; i < 2; i++ {
		if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
			t.Fatalf("RequestOTP() call %d error = %v", i+1, err)
		}
		deps.mr.FastForward(20 * time.Millisecond) // clear cooldown, stay within rate-limit window
	}

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); !errors.Is(err, ErrOTPRateLimited) {
		t.Fatalf("3rd RequestOTP() within rate-limit window error = %v, want ErrOTPRateLimited", err)
	}
}

func TestCustomerAuth_OTPMaxAttemptsLocksChallenge(t *testing.T) {
	cfg := defaultOTPConfig()
	cfg.MaxAttempts = 2
	deps := newCustomerAuthTestDeps(t, cfg)
	ctx := context.Background()
	const phone = "+70000000106"

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP() error = %v", err)
	}
	realCode := deps.sms.lastCode(t)
	wrongCode := "000000"
	if wrongCode == realCode {
		wrongCode = "111111"
	}

	if _, _, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: wrongCode, IP: "1.2.3.4"}); !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("1st wrong attempt error = %v, want ErrOTPInvalid", err)
	}
	if _, _, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: wrongCode, IP: "1.2.3.4"}); !errors.Is(err, ErrOTPLocked) {
		t.Fatalf("2nd wrong attempt (hits MaxAttempts=2) error = %v, want ErrOTPLocked", err)
	}

	// Even the real code is rejected now — the challenge was deleted on lockout.
	if _, _, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: realCode, IP: "1.2.3.4"}); !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("verify with real code after lockout error = %v, want ErrOTPInvalid", err)
	}
}

// TestCustomerAuth_ConcurrentAccountCreateRaceRecovered mirrors
// loyalty_test.go's TestAccrue_ConcurrentClientCreateRaceRecovered: two
// goroutines resolving the same not-yet-registered phone must converge on
// exactly one CustomerAccount, even when the repository's Create loses a
// race (simulated via createConflictOnce, see fakeCustomerAccountRepo).
func TestCustomerAuth_ConcurrentAccountCreateRaceRecovered(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()
	const phone = "+70000000107"

	deps.accounts.createConflictOnce[phone] = true

	account, err := deps.svc.resolveOrCreateAccount(ctx, phone)
	if err != nil {
		t.Fatalf("resolveOrCreateAccount() error = %v", err)
	}
	if deps.accounts.count() != 1 {
		t.Fatalf("accounts created = %d, want 1", deps.accounts.count())
	}
	if account.Phone != phone {
		t.Errorf("account.Phone = %q, want %q", account.Phone, phone)
	}
}

// TestCustomerAuth_VerifyOTPLinksExistingClient verifies that a Client row
// created earlier by the (unauthenticated) 1C accrual webhook — same phone,
// CustomerAccountID nil — gets linked to the CustomerAccount created on
// first successful OTP verification.
func TestCustomerAuth_VerifyOTPLinksExistingClient(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()
	const phone = "+70000000108"

	preExisting := &domain.Client{StoreID: 1, Phone: phone, CreatedAt: time.Now()}
	if err := deps.clients.Create(ctx, preExisting); err != nil {
		t.Fatalf("seed pre-existing client: %v", err)
	}

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP() error = %v", err)
	}
	code := deps.sms.lastCode(t)

	_, account, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: code, IP: "1.2.3.4"})
	if err != nil {
		t.Fatalf("VerifyOTP() error = %v", err)
	}

	linked, err := deps.clients.GetByID(ctx, preExisting.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if linked.CustomerAccountID == nil || *linked.CustomerAccountID != account.ID {
		t.Errorf("client.CustomerAccountID = %v, want %d", linked.CustomerAccountID, account.ID)
	}
}

func TestCustomerAuth_RefreshRotatesToken(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()
	const phone = "+70000000109"

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP() error = %v", err)
	}
	code := deps.sms.lastCode(t)
	first, _, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: code, IP: "1.2.3.4"})
	if err != nil {
		t.Fatalf("VerifyOTP() error = %v", err)
	}

	second, err := deps.svc.Refresh(ctx, first.RefreshToken, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Error("Refresh() returned the same refresh token, want a new one")
	}

	// The rotated-out (first) token must no longer work.
	if _, err := deps.svc.Refresh(ctx, first.RefreshToken, "1.2.3.4", "test-agent"); err == nil {
		t.Error("Refresh() with an already-rotated token succeeded, want error")
	}
}

// TestCustomerAuth_RefreshReuseDetectionRevokesChain verifies that
// presenting an already-rotated refresh token a second time — the replay
// signature of a stolen token — revokes every session for that account, not
// just the one being replayed.
func TestCustomerAuth_RefreshReuseDetectionRevokesChain(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()
	const phone = "+70000000110"

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP() error = %v", err)
	}
	code := deps.sms.lastCode(t)
	first, account, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: code, IP: "1.2.3.4"})
	if err != nil {
		t.Fatalf("VerifyOTP() error = %v", err)
	}

	second, err := deps.svc.Refresh(ctx, first.RefreshToken, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("first Refresh() error = %v", err)
	}

	// Replay the rotated-out token — simulated theft/reuse.
	if _, err := deps.svc.Refresh(ctx, first.RefreshToken, "9.9.9.9", "attacker-agent"); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("replayed Refresh() error = %v, want ErrSessionInvalid", err)
	}

	// The legitimate second-generation token must also now be revoked.
	if _, err := deps.svc.Refresh(ctx, second.RefreshToken, "1.2.3.4", "test-agent"); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("Refresh() with legitimate token after reuse detection error = %v, want ErrSessionInvalid", err)
	}

	if deps.sessions.countNonRevoked(account.ID) != 0 {
		t.Errorf("non-revoked sessions after reuse detection = %d, want 0", deps.sessions.countNonRevoked(account.ID))
	}

	actions := deps.audit.actions()
	found := false
	for _, a := range actions {
		if a == domain.AuditActionCustomerRefreshReuse {
			found = true
		}
	}
	if !found {
		t.Errorf("audit actions = %v, want %q present", actions, domain.AuditActionCustomerRefreshReuse)
	}
}

func TestCustomerAuth_LogoutRevokesSingleSession(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()
	const phone = "+70000000111"

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP() error = %v", err)
	}
	code := deps.sms.lastCode(t)
	tokens, _, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: code, IP: "1.2.3.4"})
	if err != nil {
		t.Fatalf("VerifyOTP() error = %v", err)
	}

	if err := deps.svc.Logout(ctx, tokens.RefreshToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	if _, err := deps.svc.Refresh(ctx, tokens.RefreshToken, "1.2.3.4", "test-agent"); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("Refresh() after logout error = %v, want ErrSessionInvalid", err)
	}

	// Logout on an already-revoked/unknown token is idempotent, not an error.
	if err := deps.svc.Logout(ctx, tokens.RefreshToken); err != nil {
		t.Errorf("second Logout() (idempotent) error = %v, want nil", err)
	}
}

func TestCustomerAuth_LogoutAllRevokesEverySession(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()
	const phone = "+70000000112"

	if err := deps.svc.RequestOTP(ctx, phone, "1.2.3.4"); err != nil {
		t.Fatalf("RequestOTP() error = %v", err)
	}
	code := deps.sms.lastCode(t)
	first, account, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: code, IP: "1.2.3.4"})
	if err != nil {
		t.Fatalf("VerifyOTP() error = %v", err)
	}

	// A second "device" logs in with a fresh OTP round.
	deps.mr.FastForward(20 * time.Millisecond)
	if err := deps.svc.RequestOTP(ctx, phone, "5.6.7.8"); err != nil {
		t.Fatalf("second RequestOTP() error = %v", err)
	}
	code2 := deps.sms.lastCode(t)
	second, _, err := deps.svc.VerifyOTP(ctx, VerifyOTPRequest{Phone: phone, Code: code2, IP: "5.6.7.8"})
	if err != nil {
		t.Fatalf("second VerifyOTP() error = %v", err)
	}

	if err := deps.svc.LogoutAll(ctx, account.ID); err != nil {
		t.Fatalf("LogoutAll() error = %v", err)
	}

	if _, err := deps.svc.Refresh(ctx, first.RefreshToken, "1.2.3.4", "test-agent"); !errors.Is(err, ErrSessionInvalid) {
		t.Error("Refresh() with first device's token after LogoutAll succeeded, want error")
	}
	if _, err := deps.svc.Refresh(ctx, second.RefreshToken, "5.6.7.8", "test-agent"); !errors.Is(err, ErrSessionInvalid) {
		t.Error("Refresh() with second device's token after LogoutAll succeeded, want error")
	}
}

func TestCustomerAuth_InvalidPhoneRejected(t *testing.T) {
	deps := newCustomerAuthTestDeps(t, defaultOTPConfig())
	ctx := context.Background()

	if err := deps.svc.RequestOTP(ctx, "123", "1.2.3.4"); !errors.Is(err, ErrInvalidPhone) {
		t.Fatalf("RequestOTP() with malformed phone error = %v, want ErrInvalidPhone", err)
	}
}
