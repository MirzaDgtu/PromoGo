package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

func TestParseTrustedProxies_Valid(t *testing.T) {
	prefixes, err := ParseTrustedProxies([]string{"10.0.0.0/8", " 192.168.1.0/24 "})
	if err != nil {
		t.Fatalf("ParseTrustedProxies() error = %v", err)
	}
	if len(prefixes) != 2 {
		t.Fatalf("len(prefixes) = %d, want 2", len(prefixes))
	}
}

// TestParseTrustedProxies_MalformedCIDRFails is the regression test for the
// audit finding that a malformed trusted-proxy CIDR was silently dropped
// rather than failing startup — an operator typo could silently narrow (or
// otherwise misconfigure) which peers are trusted to set X-Forwarded-For
// for every IP-scoped audit log, OTP limiter, and rate limiter, with no
// visible error.
func TestParseTrustedProxies_MalformedCIDRFails(t *testing.T) {
	_, err := ParseTrustedProxies([]string{"10.0.0.0/8", "not-a-cidr"})
	if err == nil {
		t.Fatal("ParseTrustedProxies() with a malformed CIDR error = nil, want an error")
	}
}

func TestResolveClientIP_NoTrustedProxiesUsesRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")

	if got := resolveClientIP(req, nil); got != "203.0.113.5" {
		t.Errorf("resolveClientIP() = %q, want RemoteAddr host (no trusted proxies configured)", got)
	}
}

func TestResolveClientIP_UntrustedPeerIgnoresXFF(t *testing.T) {
	prefixes, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:1234" // not in 10.0.0.0/8
	req.Header.Set("X-Forwarded-For", "198.51.100.9")

	if got := resolveClientIP(req, prefixes); got != "203.0.113.5" {
		t.Errorf("resolveClientIP() = %q, want RemoteAddr host (peer is not a trusted proxy)", got)
	}
}

func TestResolveClientIP_TrustedPeerUsesXFF(t *testing.T) {
	prefixes, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234" // in 10.0.0.0/8
	req.Header.Set("X-Forwarded-For", "198.51.100.9, 10.1.2.3")

	if got := resolveClientIP(req, prefixes); got != "198.51.100.9" {
		t.Errorf("resolveClientIP() = %q, want left-most X-Forwarded-For entry (peer is a trusted proxy)", got)
	}
}

// TestClientIP_AuditLogHonorsTrustedProxy is the end-to-end regression test
// for the audit finding that clientIP (used for audit logs and OTP
// throttling) and clientIPForRateLimit (used for the distributed rate
// limiter) were two independent resolutions that disagreed about trust:
// behind any reverse proxy, audit logs always recorded the proxy's own
// address instead of the real caller. Both now share one resolution,
// applied once per request by withResolvedClientIP (see server.go's New).
// This drives a real request through the full server chain with a trusted
// peer and an X-Forwarded-For header, and checks the audit event PromoGo
// itself recorded — not resolveClientIP directly — carries the forwarded
// address.
func TestClientIP_AuditLogHonorsTrustedProxy(t *testing.T) {
	deps, fakes := newTestDeps(t)
	deps.TrustedProxies, _ = ParseTrustedProxies([]string{"10.0.0.0/8"})
	handler := New(deps).Handler

	org := seedOrganization(fakes, "Acme")
	store := seedStore(fakes, org.ID, "Store")
	token := issueStaffToken(t, fakes, 1, org.ID, nil, domain.RoleRetailerAdmin)

	req := adminReq(http.MethodPost, "/api/v1/admin/organizations/"+itoa(org.ID)+"/stores/"+itoa(store.ID)+"/api-keys", token, map[string]any{
		"name": "1C webhook", "scopes": []string{domain.ScopeTransactionsWrite},
	})
	req.SetPathValue("orgID", itoa(org.ID))
	req.SetPathValue("storeID", itoa(store.ID))
	req.RemoteAddr = "10.5.5.5:9999" // a trusted reverse proxy
	req.Header.Set("X-Forwarded-For", "198.51.100.42")

	rec := doRequest(handler, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
	}

	fakes.AuditEvents.mu.Lock()
	defer fakes.AuditEvents.mu.Unlock()
	if len(fakes.AuditEvents.all) == 0 {
		t.Fatal("expected an audit event to be recorded")
	}
	last := fakes.AuditEvents.all[len(fakes.AuditEvents.all)-1]
	if last.IP != "198.51.100.42" {
		t.Errorf("audit event IP = %q, want the forwarded client address, not the trusted proxy's own", last.IP)
	}
}
