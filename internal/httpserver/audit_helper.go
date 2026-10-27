package httpserver

import (
	"context"
	"log/slog"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// auditCreate writes an AuditEvent, logging (not failing the request) on
// error — audit logging must never block the operation it's recording.
func auditCreate(
	ctx context.Context,
	audit domain.AuditEventRepository,
	log *slog.Logger,
	actorType domain.AuditActorType,
	actorID *int64,
	action string,
	organizationID, storeID *int64,
	targetType string,
	targetID *int64,
	ip, userAgent string,
) {
	auditCreateWithMetadata(ctx, audit, log, actorType, actorID, action, organizationID, storeID, targetType, targetID, nil, ip, userAgent)
}

// auditCreateWithMetadata is auditCreate plus a small snapshot of what
// changed (a role before/after, a key's name), so the audit screen can say
// more than "something happened". Metadata must never carry secrets or
// customer PII: every audit.read holder sees it.
func auditCreateWithMetadata(
	ctx context.Context,
	audit domain.AuditEventRepository,
	log *slog.Logger,
	actorType domain.AuditActorType,
	actorID *int64,
	action string,
	organizationID, storeID *int64,
	targetType string,
	targetID *int64,
	metadata map[string]any,
	ip, userAgent string,
) {
	event := &domain.AuditEvent{
		ActorType:      actorType,
		ActorID:        actorID,
		OrganizationID: organizationID,
		StoreID:        storeID,
		Action:         action,
		TargetType:     targetType,
		TargetID:       targetID,
		Metadata:       metadata,
		IP:             ip,
		UserAgent:      userAgent,
	}
	if err := audit.Create(ctx, event); err != nil {
		log.WarnContext(ctx, "write audit event", "action", action, "error", err)
	}
}
