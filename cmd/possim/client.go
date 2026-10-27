package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// apiError is a response the API answered on purpose — a 4xx, or a 5xx
// that survived every retry. Transient reports whether replaying the same
// request later can succeed, which is what decides whether a sale goes to
// the offline queue or is shown to the cashier as rejected.
type apiError struct {
	Status    int
	Message   string
	Code      string
	Transient bool
}

func (e *apiError) Error() string {
	msg := fmt.Sprintf("HTTP %d: %s", e.Status, e.Message)
	if e.Code != "" {
		msg += " (" + e.Code + ")"
	}
	return msg
}

// isTransient reports whether err means "the API was unreachable or
// overloaded", as opposed to "the API refused this request". Network
// errors, 429 and 5xx are transient; every other 4xx is final.
func isTransient(err error) bool {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		return apiErr.Transient
	}
	return err != nil
}

// client talks to the store-key contour exactly as 1C does: a bearer store
// API key, JSON bodies, and retries that resend the same transaction_id so
// the server's idempotency turns a retry into a replay, never a double
// accrual.
type client struct {
	baseURL  string
	apiKey   string
	http     *http.Client
	attempts int
	// backoff is the wait before retry n (1-based); tests shorten it.
	backoff func(n int) time.Duration
	// maxRetryAfter caps how long a 429's Retry-After may stall a retry.
	maxRetryAfter time.Duration
}

func newClient(baseURL, apiKey string) *client {
	return &client{
		baseURL:       strings.TrimRight(baseURL, "/"),
		apiKey:        apiKey,
		http:          &http.Client{Timeout: 10 * time.Second},
		attempts:      3,
		backoff:       func(n int) time.Duration { return time.Duration(n) * 500 * time.Millisecond },
		maxRetryAfter: 10 * time.Second,
	}
}

// do sends one request, retrying transient failures, and decodes a 2xx
// body into out (which may be nil).
func (c *client) do(ctx context.Context, method, path string, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	var lastErr error
	for attempt := 1; attempt <= c.attempts; attempt++ {
		wait, err := c.once(ctx, method, path, payload, out)
		if err == nil || !isTransient(err) {
			return err
		}
		lastErr = err
		if attempt == c.attempts {
			break
		}
		if wait <= 0 {
			wait = c.backoff(attempt)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return lastErr
}

// once performs a single attempt. The returned duration is the server's
// Retry-After hint on a 429, zero otherwise.
func (c *client) once(ctx context.Context, method, path string, payload []byte, out any) (time.Duration, error) {
	var reqBody io.Reader
	if payload != nil {
		reqBody = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("API недоступен: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil {
			return 0, nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return 0, fmt.Errorf("decode response: %w", err)
		}
		return 0, nil
	}

	apiErr := &apiError{
		Status:    resp.StatusCode,
		Transient: resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500,
	}
	var e struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Error != "" {
		apiErr.Message, apiErr.Code = e.Error, e.Code
	} else {
		apiErr.Message = strings.TrimSpace(string(raw))
	}

	var wait time.Duration
	if resp.StatusCode == http.StatusTooManyRequests {
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
			wait = min(time.Duration(secs)*time.Second, c.maxRetryAfter)
		}
	}
	return wait, apiErr
}

// --- store-contour operations (docs/openapi/openapi.yaml, tag Store) ---

type saleRequest struct {
	TransactionID string `json:"transaction_id"`
	Phone         string `json:"phone"`
	Amount        string `json:"amount"`
}

type saleResponse struct {
	ClientID     int64 `json:"client_id"`
	PointsEarned int64 `json:"points_earned"`
	Balance      int64 `json:"balance"`
	Replayed     bool  `json:"replayed"`
}

type redeemRequest struct {
	TransactionID string `json:"transaction_id"`
	ClientID      int64  `json:"client_id"`
	Points        int64  `json:"points"`
	Amount        string `json:"amount"`
}

type redeemResponse struct {
	PointsRedeemed int64 `json:"points_redeemed"`
	Balance        int64 `json:"balance"`
	Replayed       bool  `json:"replayed"`
}

type refundRequest struct {
	TransactionID           string `json:"transaction_id"`
	OriginalTransactionID   string `json:"original_transaction_id"`
	OriginalTransactionType string `json:"original_transaction_type,omitempty"`
	Amount                  string `json:"amount"`
}

type refundResponse struct {
	ClientID            int64  `json:"client_id"`
	PointsReversed      int64  `json:"points_reversed"`
	Balance             int64  `json:"balance"`
	RefundedAmountTotal string `json:"refunded_amount_total"`
	FullyRefunded       bool   `json:"fully_refunded"`
	Replayed            bool   `json:"replayed"`
}

type clientBalance struct {
	ClientID int64 `json:"client_id"`
	Balance  int64 `json:"balance"`
}

func (c *client) sale(ctx context.Context, req saleRequest) (saleResponse, error) {
	var out saleResponse
	err := c.do(ctx, http.MethodPost, "/api/v1/transactions", req, &out)
	return out, err
}

func (c *client) redeem(ctx context.Context, req redeemRequest) (redeemResponse, error) {
	var out redeemResponse
	err := c.do(ctx, http.MethodPost, "/api/v1/transactions/redeem", req, &out)
	return out, err
}

func (c *client) refund(ctx context.Context, req refundRequest) (refundResponse, error) {
	var out refundResponse
	err := c.do(ctx, http.MethodPost, "/api/v1/transactions/refund", req, &out)
	return out, err
}

// lookup returns found=false (and no error) for a phone the store has
// never seen — at a real register that is a new customer, not a failure.
func (c *client) lookup(ctx context.Context, phone string) (clientBalance, bool, error) {
	var out clientBalance
	err := c.do(ctx, http.MethodGet, "/api/v1/clients/lookup?phone="+url.QueryEscape(phone), nil, &out)
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return clientBalance{}, false, nil
	}
	return out, err == nil, err
}

func (c *client) resolveQR(ctx context.Context, payload string) (clientBalance, error) {
	var out clientBalance
	err := c.do(ctx, http.MethodPost, "/api/v1/clients/resolve-qr", map[string]string{"payload": payload}, &out)
	return out, err
}
