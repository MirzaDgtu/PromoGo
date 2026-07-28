package httpserver

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

const defaultAuditListLimit = 100

type auditEventResponseBody struct {
	ID         int64          `json:"id"`
	OccurredAt time.Time      `json:"occurred_at"`
	ActorType  string         `json:"actor_type"`
	ActorID    *int64         `json:"actor_id,omitempty"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type,omitempty"`
	TargetID   *int64         `json:"target_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	IP         string         `json:"ip,omitempty"`
}

// handleListAuditEvents returns a handler for GET
// /api/v1/admin/organizations/{orgID}/audit. Must run behind
// RequireStaff(audit.read, orgScopeFromPath).
func handleListAuditEvents(audit domain.AuditEventRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		events, err := audit.ListByOrganization(r.Context(), orgID, defaultAuditListLimit)
		if err != nil {
			log.ErrorContext(r.Context(), "list audit events", "organization_id", orgID, "error", err)
			writeError(w, http.StatusInternalServerError, "list audit events")
			return
		}

		out := make([]auditEventResponseBody, 0, len(events))
		for _, e := range events {
			out = append(out, auditEventResponseBody{
				ID: e.ID, OccurredAt: e.OccurredAt, ActorType: string(e.ActorType), ActorID: e.ActorID,
				Action: e.Action, TargetType: e.TargetType, TargetID: e.TargetID, Metadata: e.Metadata, IP: e.IP,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"events": out})
	}
}
