package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// AuditEventRepository is a pgx-backed implementation of
// domain.AuditEventRepository.
type AuditEventRepository struct {
	pool *pgxpool.Pool
}

// NewAuditEventRepository creates an AuditEventRepository backed by pool.
func NewAuditEventRepository(pool *pgxpool.Pool) *AuditEventRepository {
	return &AuditEventRepository{pool: pool}
}

func (r *AuditEventRepository) Create(ctx context.Context, event *domain.AuditEvent) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal audit event metadata: %w", err)
	}

	const query = `
		INSERT INTO audit_events (actor_type, actor_id, organization_id, store_id, action, target_type, target_id, metadata, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, occurred_at`

	err = r.pool.QueryRow(ctx, query,
		event.ActorType, event.ActorID, event.OrganizationID, event.StoreID,
		event.Action, event.TargetType, event.TargetID, metadataJSON, event.IP, event.UserAgent,
	).Scan(&event.ID, &event.OccurredAt)
	if err != nil {
		return fmt.Errorf("create audit event %q: %w", event.Action, err)
	}

	return nil
}

func (r *AuditEventRepository) ListByOrganization(ctx context.Context, organizationID int64, limit int) ([]*domain.AuditEvent, error) {
	const query = `
		SELECT id, occurred_at, actor_type, actor_id, organization_id, store_id, action, target_type, target_id, metadata, ip, user_agent
		FROM audit_events
		WHERE organization_id = $1
		ORDER BY occurred_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, organizationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit events for organization %d: %w", organizationID, err)
	}
	defer rows.Close()

	var events []*domain.AuditEvent
	for rows.Next() {
		event, err := scanAuditEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list audit events for organization %d: %w", organizationID, err)
	}

	return events, nil
}

func scanAuditEvent(row pgx.Row) (*domain.AuditEvent, error) {
	event := &domain.AuditEvent{}
	var metadataJSON []byte
	err := row.Scan(&event.ID, &event.OccurredAt, &event.ActorType, &event.ActorID, &event.OrganizationID, &event.StoreID,
		&event.Action, &event.TargetType, &event.TargetID, &metadataJSON, &event.IP, &event.UserAgent)
	if err != nil {
		return nil, err
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal audit event metadata: %w", err)
		}
	}
	return event, nil
}
