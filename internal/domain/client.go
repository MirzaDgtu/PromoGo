package domain

import (
	"context"
	"time"
)

// Client is a loyalty-program participant, identified by phone number.
// MVP scope is a single store per client (see StoreID); coalition/multi-store
// modes from Idea.md are a later phase.
//
// CustomerAccountID links this store-scoped participation to the customer's
// global, phone-verified identity (see CustomerAccount) once they register
// via the mobile app's OTP flow. It is nil for a Client created by the 1C
// accrual webhook for a phone that has never opened the app — that Client
// remains fully functional (accrual/redemption/balance) unauthenticated by
// design; CustomerAccountID only gates the customer's own API access
// (GET /me, /me/balance, /me/transactions), never the store-side webhook.
type Client struct {
	ID                int64
	StoreID           int64
	Phone             string
	CustomerAccountID *int64
	CreatedAt         time.Time
}

// ClientRepository persists and retrieves Client rows.
type ClientRepository interface {
	// GetByPhone returns domain.ErrNotFound if no client with that phone
	// exists for storeID.
	GetByPhone(ctx context.Context, storeID int64, phone string) (*Client, error)
	GetByID(ctx context.Context, id int64) (*Client, error)
	// Create returns domain.ErrConflict if storeID already has a client
	// with this phone number.
	Create(ctx context.Context, client *Client) error
	// ListUnlinkedByPhone returns every Client row across all stores with
	// this phone and CustomerAccountID still nil — the candidates
	// CustomerAuthService.VerifyOTP links to a newly verified
	// CustomerAccount.
	ListUnlinkedByPhone(ctx context.Context, phone string) ([]*Client, error)
	// LinkCustomerAccount sets clientID's CustomerAccountID. Called once per
	// Client during OTP verification linking; idempotent if already linked
	// to the same account.
	LinkCustomerAccount(ctx context.Context, clientID, customerAccountID int64) error
	// ListByCustomerAccount returns every Client row (across stores) linked
	// to customerAccountID, for the customer-facing /me endpoints.
	ListByCustomerAccount(ctx context.Context, customerAccountID int64) ([]*Client, error)
}
