package httpserver

import (
	"encoding/json"
	"net/http"
)

// writeJSON writes v as a JSON response body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response of the form {"error": message}.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeErrorCode writes a JSON error response of the form
// {"error": message, "code": code} — additive to writeError's shape, for
// error conditions callers need to branch on programmatically (e.g.
// "daily_redeem_limit_exceeded" vs a generic insufficient-balance message).
func writeErrorCode(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
