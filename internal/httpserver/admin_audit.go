package httpserver

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

const (
	defaultAuditListLimit = 50
	maxAuditListLimit     = 200
)

// auditCategories maps the ?category= filter to the action prefix it
// selects. A fixed set — the prefix reaches a LIKE pattern, so raw input
// never does.
var auditCategories = map[string]string{
	"organization":   "organization.",
	"store":          "store.",
	"staff":          "staff.",
	"loyalty_config": "loyalty_config.",
	"api_key":        "api_key.",
}

type auditEventResponseBody struct {
	ID         int64          `json:"id"`
	OccurredAt time.Time      `json:"occurred_at"`
	ActorType  string         `json:"actor_type"`
	ActorID    *int64         `json:"actor_id,omitempty"`
	ActorName  string         `json:"actor_name,omitempty"`
	StoreID    *int64         `json:"store_id,omitempty"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type,omitempty"`
	TargetID   *int64         `json:"target_id,omitempty"`
	TargetName string         `json:"target_name,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	IP         string         `json:"ip,omitempty"`
}

// encodeAuditCursor and decodeAuditCursor carry the (occurred_at, id) keyset
// of a page's last event as an opaque string. A bad cursor is a 400, not a
// silent restart from the top (same rule as decodeAdminClientCursor).
func encodeAuditCursor(e *domain.AuditEvent) string {
	raw := strconv.FormatInt(e.OccurredAt.UnixMicro(), 10) + ":" + strconv.FormatInt(e.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeAuditCursor(raw string) (time.Time, int64, error) {
	if raw == "" {
		return time.Time{}, 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("decode cursor: %w", err)
	}
	micros, id, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return time.Time{}, 0, errors.New("malformed cursor")
	}
	us, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("malformed cursor time: %w", err)
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}, 0, errors.New("malformed cursor id")
	}
	return time.UnixMicro(us), n, nil
}

func parseAuditListLimit(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultAuditListLimit
	}
	return min(n, maxAuditListLimit)
}

// staffNames resolves staff user ids (actors) and staff membership ids
// (targets) to a display name, caching within one request. The name is a
// person's display name, else email, else the access code they were
// granted by — the same rule the staff screen uses.
type staffNames struct {
	users       domain.StaffUserRepository
	memberships domain.StaffMembershipRepository
	byUser      map[int64]string
}

func (n *staffNames) user(ctx context.Context, id int64) string {
	if name, ok := n.byUser[id]; ok {
		return name
	}
	var name string
	if u, err := n.users.GetByID(ctx, id); err == nil {
		name = u.DisplayName
		if name == "" {
			name = u.Email
		}
		if name == "" {
			name = u.ExternalSubject
		}
	}
	n.byUser[id] = name
	return name
}

func (n *staffNames) membership(ctx context.Context, id int64) string {
	m, err := n.memberships.GetByID(ctx, id)
	if err != nil {
		return ""
	}
	return n.user(ctx, m.StaffUserID)
}

// handleListAuditEvents returns a handler for GET
// /api/v1/admin/organizations/{orgID}/audit — the organization's events,
// newest first, keyset-paginated (?limit=&cursor=) and optionally filtered
// by ?category=. Staff actors and staff-membership targets come back with
// a resolved name; a name that can't be resolved is left empty rather than
// failing the page. Must run behind RequireStaff(audit.read, orgScopeFromPath).
func handleListAuditEvents(audit domain.AuditEventRepository, users domain.StaffUserRepository, memberships domain.StaffMembershipRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := strconv.ParseInt(r.PathValue("orgID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid organization id")
			return
		}

		query := domain.AuditListQuery{Limit: parseAuditListLimit(r.URL.Query().Get("limit"))}
		if category := r.URL.Query().Get("category"); category != "" {
			prefix, ok := auditCategories[category]
			if !ok {
				writeError(w, http.StatusBadRequest, "invalid category")
				return
			}
			query.ActionPrefix = prefix
		}
		query.BeforeOccurredAt, query.BeforeID, err = decodeAuditCursor(r.URL.Query().Get("cursor"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}

		// Fetch one extra row to detect whether a next page exists.
		limit := query.Limit
		query.Limit++
		events, err := audit.ListByOrganization(r.Context(), orgID, query)
		if err != nil {
			log.ErrorContext(r.Context(), "list audit events", "organization_id", orgID, "error", err)
			writeError(w, http.StatusInternalServerError, "list audit events")
			return
		}
		var nextCursor string
		if len(events) > limit {
			events = events[:limit]
			nextCursor = encodeAuditCursor(events[limit-1])
		}

		names := &staffNames{users: users, memberships: memberships, byUser: map[int64]string{}}
		out := make([]auditEventResponseBody, 0, len(events))
		for _, e := range events {
			body := auditEventResponseBody{
				ID: e.ID, OccurredAt: e.OccurredAt, ActorType: string(e.ActorType), ActorID: e.ActorID,
				StoreID: e.StoreID, Action: e.Action, TargetType: e.TargetType, TargetID: e.TargetID,
				Metadata: e.Metadata, IP: e.IP,
			}
			if e.ActorType == domain.AuditActorStaff && e.ActorID != nil {
				body.ActorName = names.user(r.Context(), *e.ActorID)
			}
			if e.TargetType == "staff_membership" && e.TargetID != nil {
				body.TargetName = names.membership(r.Context(), *e.TargetID)
			}
			out = append(out, body)
		}

		resp := map[string]any{"events": out}
		if nextCursor != "" {
			resp["next_cursor"] = nextCursor
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
