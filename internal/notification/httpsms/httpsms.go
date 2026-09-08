// Package httpsms implements domain.SMSSender against a generic HTTPS SMS
// gateway contract. No specific vendor is hardcoded — PromoGo hasn't chosen
// one yet (see .claude/skills/add-notification-channel/SKILL.md) — so this
// adapter is the production default (config.SMSConfig.Provider "http")
// until a concrete provider integration replaces it.
//
// Request/response contract (documented here as the single source of
// truth until a real vendor is chosen):
//
//	POST <endpoint>
//	Authorization: Bearer <token>
//	Content-Type: application/json
//
//	{"phone": "+79991234567", "message": "...", "sender": "PromoGo"}
//
//	200 OK
//	{"status": "queued", "message_id": "..."}
package httpsms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/config"
)

// maxAttempts bounds retries on transport failures and 5xx responses only
// — a 4xx is the gateway rejecting the request as malformed/unauthorized,
// which a retry can never fix. No backoff delay between attempts: this is
// a small, bounded retry against a request that already timed out or
// failed at the transport level, not a rate-limited API needing backoff.
const maxAttempts = 3

// Sender implements domain.SMSSender by posting to a configured HTTPS
// gateway. Never logs the message body (may contain an OTP code), the
// gateway token, the raw phone number, or the response body.
type Sender struct {
	cfg    config.SMSConfig
	client *http.Client
	log    *slog.Logger
}

// New constructs a Sender. log defaults to slog.Default() if nil.
func New(cfg config.SMSConfig, log *slog.Logger) *Sender {
	if log == nil {
		log = slog.Default()
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Sender{cfg: cfg, client: &http.Client{Timeout: timeout}, log: log}
}

type smsRequest struct {
	Phone   string `json:"phone"`
	Message string `json:"message"`
	Sender  string `json:"sender,omitempty"`
}

type smsResponse struct {
	Status string `json:"status"`
}

// Send implements domain.SMSSender.
func (s *Sender) Send(ctx context.Context, phone, message string) error {
	body, err := json.Marshal(smsRequest{Phone: phone, Message: message, Sender: s.cfg.Sender})
	if err != nil {
		return fmt.Errorf("send sms: encode request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		status, sendErr := s.attempt(ctx, body)
		if sendErr == nil {
			s.log.InfoContext(ctx, "sms sent", "phone", auth.MaskPhone(phone), "status", status)
			return nil
		}

		lastErr = sendErr
		if !isRetryable(sendErr) {
			break
		}
	}

	return fmt.Errorf("send sms: %w", lastErr)
}

type retryableError struct{ err error }

func (e retryableError) Error() string { return e.err.Error() }
func (e retryableError) Unwrap() error { return e.err }

func isRetryable(err error) bool {
	_, ok := err.(retryableError)
	return ok
}

// attempt performs a single POST, returning the gateway's reported status
// on success. Transport errors and 5xx responses are wrapped as
// retryableError; a 4xx is returned as a plain (non-retryable) error.
func (s *Sender) attempt(ctx context.Context, body []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", retryableError{fmt.Errorf("transport: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return "", retryableError{fmt.Errorf("gateway returned %d", resp.StatusCode)}
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("gateway rejected request: %d", resp.StatusCode)
	}

	var out smsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return out.Status, nil
}
