package domain

import (
	"context"
	"time"
)

// Organization is the tenant parent of Store: a retailer or trading network.
// Every Store belongs to exactly one Organization; StaffMembership grants a
// role scoped to an Organization (optionally narrowed to one Store).
type Organization struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

// OrganizationRepository persists and retrieves Organization rows.
type OrganizationRepository interface {
	// GetByID returns domain.ErrNotFound if no organization has that id.
	GetByID(ctx context.Context, id int64) (*Organization, error)
	// GetByName returns domain.ErrNotFound if no organization has exactly
	// this name, or a plain error if more than one does — name has no
	// unique constraint, so ambiguity must be resolved explicitly (e.g. via
	// GetByID), not guessed through.
	GetByName(ctx context.Context, name string) (*Organization, error)
	Create(ctx context.Context, org *Organization) error
}
