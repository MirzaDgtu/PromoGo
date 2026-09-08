package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// Exported QR errors — see qr_store.go's unexported errQR* for the
// underlying Redis-level conditions these wrap.
var (
	// ErrQRIssueCooldown is returned by IssueQR when a QR was already issued
	// for this CustomerAccount within QRConfig.IssueCooldown.
	ErrQRIssueCooldown = errors.New("qr issue cooldown active")
	// ErrQRConsumeCooldown is returned by ResolveQR when the calling store
	// principal has attempted a resolve too recently (QRConfig.ConsumeCooldown).
	ErrQRConsumeCooldown = errors.New("qr consume cooldown active")
	// ErrQRGone is returned by ResolveQR for a token that is expired,
	// already consumed, or was never issued — deliberately a single error
	// for all three (see qr_store.go's consume doc comment); HTTP maps this
	// to 410.
	ErrQRGone = errors.New("qr token expired, consumed, or unknown")
	// ErrQRMalformed is returned by ResolveQR when payload doesn't parse as
	// a well-formed QR token, before any Redis lookup; HTTP maps this to 400.
	ErrQRMalformed = errors.New("malformed qr payload")
)

// QRService implements QR-based client identification (DEC-011): a
// customer mints a one-time opaque token via IssueQR, and a store's POS
// exchanges it for a store-scoped Client via ResolveQR.
type QRService struct {
	log              *slog.Logger
	store            *qrStore
	clients          domain.ClientRepository
	customerAccounts domain.CustomerAccountRepository
	balances         domain.BalanceRepository
	audit            domain.AuditEventRepository
}

// NewQRService constructs a QRService.
func NewQRService(
	log *slog.Logger,
	rdb *redis.Client,
	cfg QRConfig,
	clients domain.ClientRepository,
	customerAccounts domain.CustomerAccountRepository,
	balances domain.BalanceRepository,
	audit domain.AuditEventRepository,
) *QRService {
	return &QRService{
		log:              log,
		store:            newQRStore(rdb, cfg),
		clients:          clients,
		customerAccounts: customerAccounts,
		balances:         balances,
		audit:            audit,
	}
}

// IssueQR mints a new one-time QR payload for customerAccountID. Returns
// ErrQRIssueCooldown (with retryAfter set) if issued too recently.
func (s *QRService) IssueQR(ctx context.Context, customerAccountID int64) (payload string, expiresAt time.Time, retryAfter time.Duration, err error) {
	payload, expiresAt, retryAfter, err = s.store.issue(ctx, customerAccountID)
	if errors.Is(err, errQRIssueCooldown) {
		return "", time.Time{}, retryAfter, ErrQRIssueCooldown
	}
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("issue qr: %w", err)
	}
	return payload, expiresAt, 0, nil
}

// ResolveQR atomically consumes payload and returns the store-scoped
// Client and Balance for the CustomerAccount it was issued to, creating and
// linking the Client if this is the first time that account has
// transacted at storeID. consumePrincipal identifies the calling store
// credential for ResolveQR's own cooldown (independent of the token's
// one-time-use property) — pass the store API key's KeyID.
//
// IDOR analysis (DEC-011): payload encodes nothing but an opaque token; the
// CustomerAccountID it maps to exists only in Redis, keyed by the token's
// hash. The response always scopes the returned Client/Balance to storeID
// (the caller's own store, from its API key) — never to any store implied
// by the payload, because the payload carries no store information at all.
// So no combination of a valid QR and a valid-but-wrong-store credential
// can cross a store boundary or enumerate another store's data.
func (s *QRService) ResolveQR(ctx context.Context, storeID int64, payload, consumePrincipal string) (*domain.Client, *domain.Balance, error) {
	if _, err := s.store.checkConsumeCooldown(ctx, consumePrincipal); err != nil {
		if errors.Is(err, errQRConsumeCooldown) {
			return nil, nil, ErrQRConsumeCooldown
		}
		return nil, nil, fmt.Errorf("resolve qr: check consume cooldown: %w", err)
	}

	customerAccountID, err := s.store.consume(ctx, payload)
	if errors.Is(err, errQRMalformed) {
		return nil, nil, ErrQRMalformed
	}
	if errors.Is(err, errQRGone) {
		return nil, nil, ErrQRGone
	}
	if err != nil {
		return nil, nil, fmt.Errorf("resolve qr: consume token: %w", err)
	}

	account, err := s.customerAccounts.GetByID(ctx, customerAccountID)
	if errors.Is(err, domain.ErrNotFound) {
		// The token was valid, but the account it named no longer exists —
		// as safe and uninformative to the caller as any other gone token.
		return nil, nil, ErrQRGone
	}
	if err != nil {
		return nil, nil, fmt.Errorf("resolve qr: load customer account: %w", err)
	}
	if account.Status != domain.CustomerAccountActive {
		return nil, nil, ErrQRGone
	}

	client, err := s.resolveOrCreateClientForAccount(ctx, storeID, account)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve qr: resolve client: %w", err)
	}

	balance, err := s.balances.Get(ctx, client.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve qr: load balance: %w", err)
	}

	if s.audit != nil {
		clientID := client.ID
		if err := s.audit.Create(ctx, &domain.AuditEvent{
			OccurredAt: time.Now(),
			ActorType:  domain.AuditActorStoreAPIKey,
			StoreID:    &storeID,
			Action:     domain.AuditActionQRResolved,
			TargetType: "client",
			TargetID:   &clientID,
		}); err != nil {
			s.log.WarnContext(ctx, "record qr resolve audit event", "store_id", storeID, "error", err)
		}
	}

	return client, balance, nil
}

// resolveOrCreateClientForAccount finds this store's existing Client for
// account, or creates and links one — the QR-flow analogue of
// LoyaltyService.resolveOrCreateClient, keyed by an already-known
// CustomerAccountID instead of a phone lookup.
func (s *QRService) resolveOrCreateClientForAccount(ctx context.Context, storeID int64, account *domain.CustomerAccount) (*domain.Client, error) {
	existing, err := s.clients.ListByCustomerAccount(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	for _, c := range existing {
		if c.StoreID == storeID {
			return c, nil
		}
	}

	client, err := s.clients.GetByPhone(ctx, storeID, account.Phone)
	if err == nil {
		if linkErr := s.clients.LinkCustomerAccount(ctx, client.ID, account.ID); linkErr != nil {
			return nil, linkErr
		}
		return client, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	client = &domain.Client{StoreID: storeID, Phone: account.Phone, CreatedAt: time.Now()}
	if createErr := s.clients.Create(ctx, client); createErr != nil {
		if errors.Is(createErr, domain.ErrConflict) {
			// Lost a race with a concurrent registration of the same phone —
			// re-read and link that winner instead.
			client, err = s.clients.GetByPhone(ctx, storeID, account.Phone)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, createErr
		}
	}
	if err := s.clients.LinkCustomerAccount(ctx, client.ID, account.ID); err != nil {
		return nil, err
	}
	return client, nil
}
