package httpserver

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/ratelimit"
	"github.com/MirzaDgtu/PromoGo/internal/service"
)

// This file collects the in-memory fakes and the Deps builder shared by
// route-level tests that exercise the actual Handler New() assembles
// (server_test.go, admin_*_test.go, transactions_test.go, etc). Narrower,
// handler-local fakes (fakeMeClientRepo, fakeStoreRepo, ...) stay in their
// existing test files.

// --- Client / Balance / Transaction / Ledger / LoyaltyConfig ---

// fakeFullClientRepo is a complete in-memory domain.ClientRepository — full
// Create/lookup/link semantics, unlike me_test.go's read-mostly
// fakeMeClientRepo — for tests that exercise accrual/redemption/admin client
// flows end to end.
type fakeFullClientRepo struct {
	mu     sync.Mutex
	byID   map[int64]*domain.Client
	nextID int64
}

func newFakeFullClientRepo() *fakeFullClientRepo {
	return &fakeFullClientRepo{byID: map[int64]*domain.Client{}}
}

func (f *fakeFullClientRepo) GetByPhone(_ context.Context, storeID int64, phone string) (*domain.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.byID {
		if c.StoreID == storeID && c.Phone == phone {
			return c, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeFullClientRepo) GetByID(_ context.Context, id int64) (*domain.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeFullClientRepo) Create(_ context.Context, c *domain.Client) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.byID {
		if existing.StoreID == c.StoreID && existing.Phone == c.Phone {
			return domain.ErrConflict
		}
	}
	f.nextID++
	c.ID = f.nextID
	f.byID[c.ID] = c
	return nil
}

func (f *fakeFullClientRepo) ListUnlinkedByPhone(_ context.Context, phone string) ([]*domain.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Client
	for _, c := range f.byID {
		if c.Phone == phone && c.CustomerAccountID == nil {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeFullClientRepo) LinkCustomerAccount(_ context.Context, clientID, customerAccountID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[clientID]
	if !ok {
		return domain.ErrNotFound
	}
	c.CustomerAccountID = &customerAccountID
	return nil
}

func (f *fakeFullClientRepo) ListByCustomerAccount(_ context.Context, customerAccountID int64) ([]*domain.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Client
	for _, c := range f.byID {
		if c.CustomerAccountID != nil && *c.CustomerAccountID == customerAccountID {
			out = append(out, c)
		}
	}
	return out, nil
}

// seed inserts c directly (bypassing the (storeID, phone) conflict check)
// and assigns it an ID, for test setup.
func (f *fakeFullClientRepo) seed(c *domain.Client) *domain.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	c.ID = f.nextID
	f.byID[c.ID] = c
	return c
}

// fakeBalanceRepo is an in-memory domain.BalanceRepository.
type fakeBalanceRepo struct {
	mu     sync.Mutex
	points map[int64]int64
}

func newFakeBalanceRepo() *fakeBalanceRepo { return &fakeBalanceRepo{points: map[int64]int64{}} }

func (f *fakeBalanceRepo) Get(_ context.Context, clientID int64) (*domain.Balance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &domain.Balance{ClientID: clientID, Points: f.points[clientID]}, nil
}

func (f *fakeBalanceRepo) set(clientID, points int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.points[clientID] = points
}

// fakeFullTransactionRepo is a complete in-memory domain.TransactionRepository.
type fakeFullTransactionRepo struct {
	mu  sync.Mutex
	all []*domain.Transaction
}

func (f *fakeFullTransactionRepo) GetByExternalID(_ context.Context, storeID int64, txType domain.TransactionType, externalTxID string) (*domain.Transaction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, tx := range f.all {
		if tx.StoreID == storeID && tx.Type == txType && tx.ExternalTxID == externalTxID {
			return tx, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeFullTransactionRepo) ListByClient(_ context.Context, clientID int64) ([]*domain.Transaction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Transaction
	for _, tx := range f.all {
		if tx.ClientID == clientID {
			out = append(out, tx)
		}
	}
	return out, nil
}

func (f *fakeFullTransactionRepo) ListByClientIDs(_ context.Context, clientIDs []int64, limit int, before *domain.TransactionCursor) ([]*domain.Transaction, error) {
	ids := make(map[int64]bool, len(clientIDs))
	for _, id := range clientIDs {
		ids[id] = true
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Transaction
	for _, tx := range f.all {
		if ids[tx.ClientID] {
			out = append(out, tx)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// fakeLedgerRepo mirrors LedgerRepository.Post's real semantics (atomic
// insert + balance adjustment, ErrConflict on a duplicate key,
// ErrInsufficientBalance instead of letting the balance go negative) against
// the same fakeFullTransactionRepo/fakeBalanceRepo the service reads from.
// See internal/service/loyalty_test.go's fakeLedgerRepo for the pattern this
// mirrors.
type fakeLedgerRepo struct {
	mu       sync.Mutex
	txs      *fakeFullTransactionRepo
	balances *fakeBalanceRepo
	nextID   int64
}

func (f *fakeLedgerRepo) Post(_ context.Context, tx *domain.Transaction) (*domain.Transaction, *domain.Balance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.txs.mu.Lock()
	for _, existing := range f.txs.all {
		if existing.StoreID == tx.StoreID && existing.Type == tx.Type && existing.ExternalTxID == tx.ExternalTxID {
			f.txs.mu.Unlock()
			return nil, nil, domain.ErrConflict
		}
	}
	f.txs.mu.Unlock()

	f.balances.mu.Lock()
	newPoints := f.balances.points[tx.ClientID] + tx.PointsDelta
	if newPoints < 0 {
		f.balances.mu.Unlock()
		return nil, nil, domain.ErrInsufficientBalance
	}
	f.balances.points[tx.ClientID] = newPoints
	f.balances.mu.Unlock()

	f.nextID++
	posted := *tx
	posted.ID = f.nextID
	posted.BalanceAfter = newPoints
	posted.CreatedAt = time.Now()

	f.txs.mu.Lock()
	f.txs.all = append(f.txs.all, &posted)
	f.txs.mu.Unlock()

	return &posted, &domain.Balance{ClientID: tx.ClientID, Points: newPoints}, nil
}

// fakeLoyaltyConfigRepo is an in-memory domain.LoyaltyConfigRepository.
type fakeLoyaltyConfigRepo struct {
	mu      sync.Mutex
	byStore map[int64]*domain.LoyaltyConfig
}

func newFakeLoyaltyConfigRepo() *fakeLoyaltyConfigRepo {
	return &fakeLoyaltyConfigRepo{byStore: map[int64]*domain.LoyaltyConfig{}}
}

func (f *fakeLoyaltyConfigRepo) GetByStore(_ context.Context, storeID int64) (*domain.LoyaltyConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cfg, ok := f.byStore[storeID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cfg, nil
}

func (f *fakeLoyaltyConfigRepo) Upsert(_ context.Context, cfg *domain.LoyaltyConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byStore[cfg.StoreID] = cfg
	return nil
}

// --- Organization ---

type fakeOrganizationRepo struct {
	mu     sync.Mutex
	byID   map[int64]*domain.Organization
	nextID int64
}

func newFakeOrganizationRepo() *fakeOrganizationRepo {
	return &fakeOrganizationRepo{byID: map[int64]*domain.Organization{}}
}

func (f *fakeOrganizationRepo) GetByID(_ context.Context, id int64) (*domain.Organization, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return o, nil
}

func (f *fakeOrganizationRepo) GetByName(_ context.Context, name string) (*domain.Organization, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, o := range f.byID {
		if o.Name == name {
			return o, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeOrganizationRepo) Create(_ context.Context, o *domain.Organization) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	o.ID = f.nextID
	o.CreatedAt = time.Now()
	f.byID[o.ID] = o
	return nil
}

// --- Customer account / session / consent ---

type fakeCustomerAccountRepo struct {
	mu     sync.Mutex
	byID   map[int64]*domain.CustomerAccount
	nextID int64
}

func newFakeCustomerAccountRepo() *fakeCustomerAccountRepo {
	return &fakeCustomerAccountRepo{byID: map[int64]*domain.CustomerAccount{}}
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
	for _, a := range f.byID {
		if a.Phone == acc.Phone {
			return domain.ErrConflict
		}
	}
	f.nextID++
	acc.ID = f.nextID
	acc.CreatedAt = time.Now()
	acc.UpdatedAt = time.Now()
	f.byID[acc.ID] = acc
	return nil
}

// seed inserts an already-verified, active account directly for test setup.
func (f *fakeCustomerAccountRepo) seed(phone string) *domain.CustomerAccount {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	acc := &domain.CustomerAccount{
		ID: f.nextID, Phone: phone, Status: domain.CustomerAccountActive,
		PhoneVerifiedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	f.byID[acc.ID] = acc
	return acc
}

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
	f.byID[s.ID] = s
	return nil
}

func (f *fakeCustomerSessionRepo) GetByRefreshTokenHash(_ context.Context, hash string) (*domain.CustomerSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.byID {
		if s.RefreshTokenHash == hash {
			return s, nil
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
	now := time.Now()
	s.RevokedAt = &now
	return nil
}

func (f *fakeCustomerSessionRepo) RevokeAllForAccount(_ context.Context, customerAccountID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for _, s := range f.byID {
		if s.CustomerAccountID == customerAccountID && s.RevokedAt == nil {
			s.RevokedAt = &now
		}
	}
	return nil
}

type fakeCustomerConsentRepo struct {
	mu  sync.Mutex
	all []*domain.CustomerConsent
}

func (f *fakeCustomerConsentRepo) Create(_ context.Context, c *domain.CustomerConsent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.all = append(f.all, c)
	return nil
}

// --- Staff user / membership ---

type fakeStaffUserRepo struct {
	mu     sync.Mutex
	byID   map[int64]*domain.StaffUser
	nextID int64
}

func newFakeStaffUserRepo() *fakeStaffUserRepo {
	return &fakeStaffUserRepo{byID: map[int64]*domain.StaffUser{}}
}

func (f *fakeStaffUserRepo) GetByExternalSubject(_ context.Context, subject string) (*domain.StaffUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.byID {
		if u.ExternalSubject == subject {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeStaffUserRepo) GetByID(_ context.Context, id int64) (*domain.StaffUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeStaffUserRepo) Create(_ context.Context, u *domain.StaffUser) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.byID {
		if existing.ExternalSubject == u.ExternalSubject {
			return domain.ErrConflict
		}
	}
	f.nextID++
	u.ID = f.nextID
	u.CreatedAt = time.Now()
	f.byID[u.ID] = u
	return nil
}

func (f *fakeStaffUserRepo) UpdateProfile(_ context.Context, id int64, email, displayName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	u.Email = email
	u.DisplayName = displayName
	return nil
}

func (f *fakeStaffUserRepo) seed(u *domain.StaffUser) *domain.StaffUser {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	u.ID = f.nextID
	u.CreatedAt = time.Now()
	f.byID[u.ID] = u
	return u
}

type fakeStaffMembershipRepo struct {
	mu     sync.Mutex
	byID   map[int64]*domain.StaffMembership
	nextID int64
}

func newFakeStaffMembershipRepo() *fakeStaffMembershipRepo {
	return &fakeStaffMembershipRepo{byID: map[int64]*domain.StaffMembership{}}
}

func (f *fakeStaffMembershipRepo) ListByStaffUser(_ context.Context, staffUserID int64) ([]*domain.StaffMembership, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.StaffMembership
	for _, m := range f.byID {
		if m.StaffUserID == staffUserID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeStaffMembershipRepo) ListByOrganization(_ context.Context, organizationID int64) ([]*domain.StaffMembership, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.StaffMembership
	for _, m := range f.byID {
		if m.OrganizationID == organizationID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeStaffMembershipRepo) Create(_ context.Context, m *domain.StaffMembership) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.byID {
		if existing.StaffUserID == m.StaffUserID && existing.OrganizationID == m.OrganizationID &&
			samePtr(existing.StoreID, m.StoreID) {
			return domain.ErrConflict
		}
	}
	f.nextID++
	m.ID = f.nextID
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	f.byID[m.ID] = m
	return nil
}

func (f *fakeStaffMembershipRepo) UpdateStatus(_ context.Context, id int64, status domain.StaffStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	m.Status = status
	m.UpdatedAt = time.Now()
	return nil
}

func (f *fakeStaffMembershipRepo) UpdateRole(_ context.Context, id int64, role domain.Role) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	m.Role = role
	m.UpdatedAt = time.Now()
	return nil
}

func (f *fakeStaffMembershipRepo) seed(m *domain.StaffMembership) *domain.StaffMembership {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	m.ID = f.nextID
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	f.byID[m.ID] = m
	return m
}

func samePtr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// --- Audit ---

type fakeAuditEventRepo struct {
	mu     sync.Mutex
	all    []*domain.AuditEvent
	nextID int64
}

func newFakeAuditEventRepo() *fakeAuditEventRepo { return &fakeAuditEventRepo{} }

func (f *fakeAuditEventRepo) Create(_ context.Context, e *domain.AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	e.ID = f.nextID
	e.OccurredAt = time.Now()
	f.all = append(f.all, e)
	return nil
}

func (f *fakeAuditEventRepo) ListByOrganization(_ context.Context, organizationID int64, limit int) ([]*domain.AuditEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.AuditEvent
	for i := len(f.all) - 1; i >= 0 && len(out) < limit; i-- {
		e := f.all[i]
		if e.OrganizationID != nil && *e.OrganizationID == organizationID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeAuditEventRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.all)
}

// --- Notification / SMS ---

type fakeNotifier struct{}

func (fakeNotifier) Name() string                              { return "fake" }
func (fakeNotifier) Send(context.Context, int64, string) error { return nil }

type fakeSMSSender struct {
	mu   sync.Mutex
	sent []string
}

func (f *fakeSMSSender) Send(_ context.Context, phone, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, phone+":"+message)
	return nil
}

// --- Deps builder ---

// testFakes exposes every fake repository backing a newTestDeps server, so
// tests can seed state and make assertions.
type testFakes struct {
	Stores           *fakeStoreRepo
	StoreAPIKeys     *fakeStoreAPIKeyRepo
	Clients          *fakeFullClientRepo
	Balances         *fakeBalanceRepo
	Transactions     *fakeFullTransactionRepo
	Ledger           *fakeLedgerRepo
	LoyaltyConfigs   *fakeLoyaltyConfigRepo
	Organizations    *fakeOrganizationRepo
	CustomerAccounts *fakeCustomerAccountRepo
	CustomerSessions *fakeCustomerSessionRepo
	CustomerConsents *fakeCustomerConsentRepo
	StaffUsers       *fakeStaffUserRepo
	StaffMemberships *fakeStaffMembershipRepo
	AuditEvents      *fakeAuditEventRepo
	SMS              *fakeSMSSender
	Redis            *redis.Client
}

// newTestDeps builds a full Deps — real services wired to in-memory fakes
// and a miniredis instance — suitable for exercising the actual Handler
// New() assembles via httptest. Secrets match the customer/staff access
// token secrets already used by middleware_customer_test.go /
// middleware_staff_test.go, so tokens minted with IssueCustomerAccessToken /
// IssueStaffAccessToken in those files work against a server built from
// this Deps too.
func newTestDeps(t *testing.T) (Deps, *testFakes) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	stores := newFakeStoreRepo()
	storeAPIKeys := newFakeStoreAPIKeyRepo()
	clients := newFakeFullClientRepo()
	balances := newFakeBalanceRepo()
	txs := &fakeFullTransactionRepo{}
	ledger := &fakeLedgerRepo{txs: txs, balances: balances}
	configs := newFakeLoyaltyConfigRepo()
	orgs := newFakeOrganizationRepo()
	customerAccounts := newFakeCustomerAccountRepo()
	customerSessions := newFakeCustomerSessionRepo()
	customerConsents := &fakeCustomerConsentRepo{}
	staffUsers := newFakeStaffUserRepo()
	staffMemberships := newFakeStaffMembershipRepo()
	auditEvents := newFakeAuditEventRepo()
	sms := &fakeSMSSender{}

	log := testLogger()

	loyaltySvc := service.New(log, clients, txs, balances, ledger, configs, fakeNotifier{})

	customerAuthSvc := service.NewCustomerAuthService(
		log, customerAccounts, customerSessions, customerConsents, clients, auditEvents, sms, rdb,
		service.CustomerAuthConfig{
			AccessTokenSecret: testCustomerSecret,
			AccessTokenTTL:    time.Hour,
			RefreshTokenTTL:   30 * 24 * time.Hour,
			OTP: service.OTPConfig{
				CodeTTL:             5 * time.Minute,
				ResendCooldown:      time.Minute,
				MaxAttempts:         5,
				RateLimitWindow:     time.Hour,
				MaxRequestsPerPhone: 5,
				MaxRequestsPerIP:    20,
			},
		},
	)

	// Empty issuer/audience/JWKS URL: fine for construction (no network
	// call happens until VerifyIDToken parses a well-formed JWT with a
	// "kid" header) — sufficient for tests that only need the staff OIDC
	// route's request-validation and error-mapping behavior.
	oidcVerifier := auth.NewOIDCVerifier("", "", "", time.Minute)
	staffAuthSvc := service.NewStaffAuthService(
		log, staffUsers, staffMemberships, auditEvents, oidcVerifier,
		service.StaffAuthConfig{AccessTokenSecret: testStaffSecret, AccessTokenTTL: time.Hour},
	)

	deps := Deps{
		Log: log,

		Stores:         stores,
		Clients:        clients,
		Balances:       balances,
		Transactions:   txs,
		LoyaltyConfigs: configs,
		StoreAPIKeys:   storeAPIKeys,

		Organizations:    orgs,
		CustomerAccounts: customerAccounts,
		StaffUsers:       staffUsers,
		StaffMemberships: staffMemberships,
		AuditEvents:      auditEvents,

		Loyalty:      loyaltySvc,
		CustomerAuth: customerAuthSvc,
		StaffAuth:    staffAuthSvc,

		CustomerAccessTokenSecret: testCustomerSecret,
		StaffAccessTokenSecret:    testStaffSecret,

		Ready: func(context.Context) error { return nil },
	}

	return deps, &testFakes{
		Stores: stores, StoreAPIKeys: storeAPIKeys, Clients: clients, Balances: balances,
		Transactions: txs, Ledger: ledger, LoyaltyConfigs: configs, Organizations: orgs,
		CustomerAccounts: customerAccounts, CustomerSessions: customerSessions, CustomerConsents: customerConsents,
		StaffUsers: staffUsers, StaffMemberships: staffMemberships, AuditEvents: auditEvents,
		SMS: sms, Redis: rdb,
	}
}

// newTestServer builds the actual Handler New() assembles, backed by fresh
// in-memory fakes, for route-level tests.
func newTestServer(t *testing.T) (http.Handler, *testFakes) {
	t.Helper()
	deps, fakes := newTestDeps(t)
	return New(deps).Handler, fakes
}

// issueStaffToken mints a staff access token for staffUserID and seeds an
// active membership so RequireStaff/RequireGlobalStaffPermission checks
// against it pass for perm within (organizationID, storeID).
func issueStaffToken(t *testing.T, fakes *testFakes, staffUserID, organizationID int64, storeID *int64, role domain.Role) string {
	t.Helper()

	// ResolvePrincipal (called by RequireStaff/RequireGlobalStaffPermission
	// on every request) loads the StaffUser row itself, not just its
	// memberships — an active membership alone isn't enough to authenticate.
	if _, ok := fakes.StaffUsers.byID[staffUserID]; !ok {
		fakes.StaffUsers.byID[staffUserID] = &domain.StaffUser{
			ID: staffUserID, ExternalSubject: "test-subject-" + itoa(staffUserID), Status: domain.StaffActive,
		}
		if staffUserID > fakes.StaffUsers.nextID {
			fakes.StaffUsers.nextID = staffUserID
		}
	}

	fakes.StaffMemberships.seed(&domain.StaffMembership{
		StaffUserID: staffUserID, OrganizationID: organizationID, StoreID: storeID,
		Role: role, Status: domain.StaffActive,
	})
	token, err := auth.IssueStaffAccessToken(testStaffSecret, staffUserID, time.Hour)
	if err != nil {
		t.Fatalf("IssueStaffAccessToken() error = %v", err)
	}
	return token
}

// testRateLimitConfig returns small, deterministic limits for rate-limit
// route tests — production defaults live in config.setDefaults.
func testRateLimitConfig() config.RateLimitConfig {
	return config.RateLimitConfig{
		StaffLoginIPLimit:  2,
		StaffLoginIPWindow: time.Minute,

		AdminIPLimit:     5,
		AdminIPWindow:    time.Minute,
		AdminStaffLimit:  2,
		AdminStaffWindow: time.Minute,

		ClientLookupIPLimit:         100,
		ClientLookupIPWindow:        time.Minute,
		ClientLookupPrincipalLimit:  2,
		ClientLookupPrincipalWindow: time.Minute,
		ClientLookupPhoneLimit:      2,
		ClientLookupPhoneWindow:     time.Minute,

		AccrualIPLimit:         100,
		AccrualIPWindow:        time.Minute,
		AccrualPrincipalLimit:  2,
		AccrualPrincipalWindow: time.Minute,
	}
}

// newTestDepsWithRateLimit builds on newTestDeps with a real
// ratelimit.Limiter backed by a fresh miniredis instance, for route-level
// rate-limit tests. The returned *miniredis.Miniredis lets tests fast-
// forward past a window or inspect stored keys (e.g. to assert no raw
// phone/IP ever appears in one — see ratelimit_route_test.go).
func newTestDepsWithRateLimit(t *testing.T, cfg config.RateLimitConfig) (Deps, *testFakes, *miniredis.Miniredis) {
	t.Helper()
	deps, fakes := newTestDeps(t)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })

	deps.RateLimiter = ratelimit.New(rdb)
	deps.RateLimit = cfg
	return deps, fakes, mr
}

func issueCustomerToken(t *testing.T, customerAccountID int64) string {
	t.Helper()
	token, err := auth.IssueCustomerAccessToken(testCustomerSecret, customerAccountID, time.Hour)
	if err != nil {
		t.Fatalf("IssueCustomerAccessToken() error = %v", err)
	}
	return token
}
