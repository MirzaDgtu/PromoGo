package httpsms

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/config"
)

func newTestSender(t *testing.T, cfg config.SMSConfig, logBuf *bytes.Buffer) *Sender {
	t.Helper()
	log := slog.New(slog.NewTextHandler(logBuf, nil))
	return New(cfg, log)
}

func TestHTTPSMS_Send_Success(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)

		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
		}

		var body smsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Phone != "+79261234567" || body.Message != "секретный код 123456" {
			t.Fatalf("unexpected request body: %+v", body)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(smsResponse{Status: "queued"})
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	sender := newTestSender(t, config.SMSConfig{Endpoint: srv.URL, Token: "test-token", Sender: "PromoGo", Timeout: time.Second}, &logBuf)

	if err := sender.Send(context.Background(), "+79261234567", "секретный код 123456"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("gateway call count = %d, want 1", got)
	}

	logged := logBuf.String()
	if strings.Contains(logged, "123456") {
		t.Fatalf("log output contains the OTP code: %q", logged)
	}
	if strings.Contains(logged, "test-token") {
		t.Fatalf("log output contains the gateway token: %q", logged)
	}
	if strings.Contains(logged, "+79261234567") {
		t.Fatalf("log output contains the raw phone number: %q", logged)
	}
}

func TestHTTPSMS_Send_ClientErrorNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	sender := newTestSender(t, config.SMSConfig{Endpoint: srv.URL, Token: "t", Timeout: time.Second}, &logBuf)

	if err := sender.Send(context.Background(), "+79261234567", "code"); err == nil {
		t.Fatal("Send() error = nil, want an error for a 400 response")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("gateway call count = %d, want exactly 1 (4xx must not be retried)", got)
	}
}

func TestHTTPSMS_Send_ServerErrorRetriedThenFails(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	sender := newTestSender(t, config.SMSConfig{Endpoint: srv.URL, Token: "t", Timeout: time.Second}, &logBuf)

	if err := sender.Send(context.Background(), "+79261234567", "code"); err == nil {
		t.Fatal("Send() error = nil, want an error after exhausting retries on persistent 5xx")
	}
	if got := atomic.LoadInt32(&calls); got != maxAttempts {
		t.Fatalf("gateway call count = %d, want %d (bounded retry on 5xx)", got, maxAttempts)
	}
}

func TestHTTPSMS_Send_ServerErrorThenSuccessRecovers(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(smsResponse{Status: "queued"})
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	sender := newTestSender(t, config.SMSConfig{Endpoint: srv.URL, Token: "t", Timeout: time.Second}, &logBuf)

	if err := sender.Send(context.Background(), "+79261234567", "code"); err != nil {
		t.Fatalf("Send() error = %v, want the retry after a transient 5xx to succeed", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("gateway call count = %d, want 2 (one failure, one successful retry)", got)
	}
}

func TestHTTPSMS_Send_TimeoutIsRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(smsResponse{Status: "queued"})
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	sender := newTestSender(t, config.SMSConfig{Endpoint: srv.URL, Token: "t", Timeout: 5 * time.Millisecond}, &logBuf)

	err := sender.Send(context.Background(), "+79261234567", "code")
	if err == nil {
		t.Fatal("Send() error = nil, want a timeout error (handler always exceeds the configured client timeout)")
	}
	if got := atomic.LoadInt32(&calls); got != int32(maxAttempts) {
		t.Fatalf("gateway call count = %d, want %d (transport-level timeout is retried)", got, maxAttempts)
	}
}
