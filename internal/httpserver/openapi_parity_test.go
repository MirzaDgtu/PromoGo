package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	yaml "go.yaml.in/yaml/v3"
)

// openAPIOperation is the subset of an OpenAPI Operation Object this parity
// test cares about.
type openAPIOperation struct {
	OperationID string                 `yaml:"operationId"`
	Security    *[]map[string][]string `yaml:"security"`
}

type openAPIDoc struct {
	Paths    map[string]map[string]openAPIOperation `yaml:"paths"`
	Security []map[string][]string                  `yaml:"security"`
}

func loadOpenAPIDoc(t *testing.T) *openAPIDoc {
	t.Helper()
	// internal/httpserver -> repo root is two directories up.
	path := filepath.Join("..", "..", "docs", "openapi", "openapi.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc openAPIDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return &doc
}

// openAPIOperationEntry flattens openAPIDoc.Paths into one entry per
// operation, resolving each operation's effective security requirement
// (its own "security" if set, otherwise the document's top-level default —
// exactly how OpenAPI's inheritance rule works) so contour checks don't
// need to special-case "inherited from the document root".
type openAPIOperationEntry struct {
	Method      string
	Path        string
	OperationID string
	Security    []map[string][]string
}

func flattenOpenAPIDoc(doc *openAPIDoc) []openAPIOperationEntry {
	var entries []openAPIOperationEntry
	for path, methods := range doc.Paths {
		for method, op := range methods {
			security := doc.Security
			if op.Security != nil {
				security = *op.Security
			}
			entries = append(entries, openAPIOperationEntry{
				Method:      strings.ToUpper(method),
				Path:        path,
				OperationID: op.OperationID,
				Security:    security,
			})
		}
	}
	return entries
}

// securitySchemeFor reports the OpenAPI security scheme name a contour is
// expected to declare. contourPublic expects an explicit, empty security
// requirement (`security: []`) — "no scheme, and that's deliberate" rather
// than merely absent.
func securitySchemeFor(c contour) (scheme string, public bool) {
	switch c {
	case contourPublic:
		return "", true
	case contourStoreKey:
		return "storeApiKey", false
	case contourCustomer:
		return "customerAccessToken", false
	case contourStaff:
		return "staffAccessToken", false
	default:
		return "", false
	}
}

// TestRouteOpenAPIParity verifies routeTable (internal/httpserver/routes.go
// — the single source of truth New() registers handlers from) and
// docs/openapi/openapi.yaml describe exactly the same 29 HTTP operations,
// with matching method, path, and security contour. A mismatch here means
// either the contract lied about a route, or a code change to routeTable
// wasn't reflected in the contract.
func TestRouteOpenAPIParity(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	entries := flattenOpenAPIDoc(doc)

	byOperationID := make(map[string]openAPIOperationEntry, len(entries))
	for _, e := range entries {
		if _, dup := byOperationID[e.OperationID]; dup {
			t.Fatalf("docs/openapi/openapi.yaml: duplicate operationId %q", e.OperationID)
		}
		byOperationID[e.OperationID] = e
	}

	routeOperationIDs := make(map[string]bool, len(routeTable))

	for _, route := range routeTable {
		routeOperationIDs[route.OperationID] = true

		entry, ok := byOperationID[route.OperationID]
		if !ok {
			t.Errorf("route %s %s (operationId %q) has no matching OpenAPI operation",
				route.Method, route.Path, route.OperationID)
			continue
		}

		if entry.Method != route.Method {
			t.Errorf("operation %q: OpenAPI method = %s, route table = %s", route.OperationID, entry.Method, route.Method)
		}
		if entry.Path != route.Path {
			t.Errorf("operation %q: OpenAPI path = %s, route table = %s", route.OperationID, entry.Path, route.Path)
		}

		wantScheme, wantPublic := securitySchemeFor(route.Contour)
		if wantPublic {
			if len(entry.Security) != 0 {
				t.Errorf("operation %q: route table contour is public, but OpenAPI security = %v (want explicit empty array)",
					route.OperationID, entry.Security)
			}
			continue
		}

		found := false
		for _, req := range entry.Security {
			if _, ok := req[wantScheme]; ok {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("operation %q: route table contour expects security scheme %q, OpenAPI security = %v",
				route.OperationID, wantScheme, entry.Security)
		}
	}

	// No operations in the contract beyond what routeTable registers.
	for operationID := range byOperationID {
		if !routeOperationIDs[operationID] {
			t.Errorf("docs/openapi/openapi.yaml has operation %q with no matching route in routeTable", operationID)
		}
	}
	if len(byOperationID) != len(routeTable) {
		t.Errorf("OpenAPI operation count = %d, routeTable count = %d, want equal", len(byOperationID), len(routeTable))
	}
}

// TestPublicHealthEndpointsAreExplicitlyMarked verifies /healthz and
// /readyz declare `security: []` on the operation itself, rather than
// merely inheriting the document's top-level default — a reader looking at
// either operation in isolation should see unambiguously that it's public.
func TestPublicHealthEndpointsAreExplicitlyMarked(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	for _, path := range []string{"/healthz", "/readyz"} {
		methods, ok := doc.Paths[path]
		if !ok {
			t.Fatalf("docs/openapi/openapi.yaml has no path %q", path)
		}
		op, ok := methods["get"]
		if !ok {
			t.Fatalf("docs/openapi/openapi.yaml path %q has no GET operation", path)
		}
		if op.Security == nil {
			t.Errorf("operation %q (%s): expected an explicit `security: []`, got none (inherits the document default instead)",
				op.OperationID, path)
		} else if len(*op.Security) != 0 {
			t.Errorf("operation %q (%s): expected `security: []`, got %v", op.OperationID, path, *op.Security)
		}
	}
}
