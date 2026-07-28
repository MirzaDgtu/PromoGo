package domain

import (
	"context"
	"time"
)

// Client is a loyalty-program participant, identified by phone number.
// MVP scope is a single store per client (see StoreID); coalition/multi-store
// modes from Idea.md are a later phase.
type Client struct {
	ID        int64
	StoreID   int64
	Phone     string
	CreatedAt time.Time
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
}
