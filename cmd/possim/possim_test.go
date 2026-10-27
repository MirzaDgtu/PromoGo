package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeAPI is a minimal stand-in for the store-key contour: it answers
// accruals idempotently on transaction_id and can be told to fail the next
// requests, which is all the retry and offline-queue logic depends on.
type fakeAPI struct {
	mu       sync.Mutex
	failNext []int // statuses to answer with before behaving normally
	seen     map[string]bool
	order    []string
	auth     string
}

func newFakeAPI(t *testing.T) (*fakeAPI, *httptest.Server) {
	f := &fakeAPI{seen: map[string]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auth = r.Header.Get("Authorization")
	if len(f.failNext) > 0 {
		status := f.failNext[0]
		f.failNext = f.failNext[1:]
		if status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "1")
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": http.StatusText(status)})
		return
	}
	switch r.URL.Path {
	case "/api/v1/transactions":
		var req saleRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Phone == "bad" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid phone", "code": "invalid_phone"})
			return
		}
		replayed := f.seen[req.TransactionID]
		if !replayed {
			f.seen[req.TransactionID] = true
			f.order = append(f.order, req.TransactionID)
		}
		_ = json.NewEncoder(w).Encode(saleResponse{ClientID: 1, PointsEarned: 10, Balance: 10, Replayed: replayed})
	case "/api/v1/clients/lookup":
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func testClient(url string) *client {
	c := newClient(url, "test-key")
	c.backoff = func(int) time.Duration { return time.Millisecond }
	c.maxRetryAfter = time.Millisecond
	return c
}

func testEnv(t *testing.T, url string) env {
	return env{client: testClient(url), queue: queue{path: filepath.Join(t.TempDir(), "queue.jsonl")}}
}

func TestClientRetriesTransientFailuresWithTheSameTransactionID(t *testing.T) {
	api, srv := newFakeAPI(t)
	api.failNext = []int{http.StatusServiceUnavailable, http.StatusTooManyRequests}

	resp, err := testClient(srv.URL).sale(context.Background(), saleRequest{TransactionID: "T1", Phone: "+79990000001", Amount: "100.00"})
	if err != nil {
		t.Fatalf("sale: %v", err)
	}
	if resp.Replayed || len(api.order) != 1 {
		t.Fatalf("want one fresh accrual, got replayed=%v order=%v", resp.Replayed, api.order)
	}
	if api.auth != "Bearer test-key" {
		t.Errorf("Authorization = %q", api.auth)
	}
}

func TestClientDoesNotRetryFinalRejections(t *testing.T) {
	_, srv := newFakeAPI(t)
	_, err := testClient(srv.URL).sale(context.Background(), saleRequest{TransactionID: "T1", Phone: "bad", Amount: "100.00"})
	var apiErr *apiError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest || apiErr.Code != "invalid_phone" || isTransient(err) {
		t.Fatalf("want a final 400 invalid_phone, got %v", err)
	}
}

func TestLookupTreatsUnknownPhoneAsNewCustomer(t *testing.T) {
	_, srv := newFakeAPI(t)
	_, found, err := testClient(srv.URL).lookup(context.Background(), "+79990000001")
	if err != nil || found {
		t.Fatalf("want not found without error, got found=%v err=%v", found, err)
	}
}

func TestSaleQueuesWhileAPIIsDownAndReplaysInOrder(t *testing.T) {
	api, srv := newFakeAPI(t)
	e := testEnv(t, srv.URL)
	ctx := context.Background()

	// Every attempt of the first sale fails: it goes to the queue.
	api.failNext = []int{503, 503, 503}
	if err := checkoutSale(ctx, e, saleRequest{TransactionID: "A", Phone: "+79990000001", Amount: "100.00"}, false); err != nil {
		t.Fatal(err)
	}
	// An offline sale queues behind it without touching the API.
	if err := checkoutSale(ctx, e, saleRequest{TransactionID: "B", Phone: "+79990000001", Amount: "200.00"}, true); err != nil {
		t.Fatal(err)
	}
	if ops, _ := e.queue.load(); len(ops) != 2 || ops[0].LastErr == "" {
		t.Fatalf("want 2 queued ops with the first carrying its error, got %+v", ops)
	}

	// The link is back: the next sale drains the queue first, keeping order.
	if err := checkoutSale(ctx, e, saleRequest{TransactionID: "C", Phone: "+79990000001", Amount: "300.00"}, false); err != nil {
		t.Fatal(err)
	}
	if got := api.order; len(got) != 3 || got[0] != "A" || got[1] != "B" || got[2] != "C" {
		t.Fatalf("delivery order = %v, want [A B C]", got)
	}
	if ops, _ := e.queue.load(); len(ops) != 0 {
		t.Fatalf("queue not drained: %+v", ops)
	}
}

func TestReplayStopsAtTransientFailureAndDropsFinalRejections(t *testing.T) {
	api, srv := newFakeAPI(t)
	e := testEnv(t, srv.URL)
	ctx := context.Background()

	// "A" reached the API before the link dropped; the register queued it anyway.
	api.seen["A"] = true
	for _, op := range []queuedOp{
		{Kind: "sale", Sale: &saleRequest{TransactionID: "A", Phone: "+79990000001", Amount: "1.00"}},
		{Kind: "sale", Sale: &saleRequest{TransactionID: "X", Phone: "bad", Amount: "1.00"}},
		{Kind: "sale", Sale: &saleRequest{TransactionID: "B", Phone: "+79990000001", Amount: "1.00"}},
		{Kind: "sale", Sale: &saleRequest{TransactionID: "C", Phone: "+79990000001", Amount: "1.00"}},
	} {
		if err := e.queue.append(op); err != nil {
			t.Fatal(err)
		}
	}

	// A and X go through (X is rejected for good), then the link drops at B.
	api.failNext = nil
	quiet := func(queuedOp, string) {}
	e.client.attempts = 1
	res, err := replayWithFailureAt(ctx, api, e, quiet, "B")
	if !isTransient(err) {
		t.Fatalf("want a transient stop, got %v", err)
	}
	if res.Replayed != 1 || res.Rejected != 1 || res.Left != 2 {
		t.Fatalf("result = %+v, want replayed 1, rejected 1, left 2", res)
	}
	ops, _ := e.queue.load()
	if len(ops) != 2 || ops[0].transactionID() != "B" || ops[1].transactionID() != "C" {
		t.Fatalf("queue after stop = %+v, want [B C]", ops)
	}

	res, err = replay(ctx, e.client, e.queue, quiet)
	if err != nil || res.Delivered != 2 {
		t.Fatalf("second replay: res=%+v err=%v", res, err)
	}
	if ops, _ := e.queue.load(); len(ops) != 0 {
		t.Fatalf("queue not empty: %+v", ops)
	}
}

// replayWithFailureAt replays with the fake API failing from the moment
// the given transaction is attempted.
func replayWithFailureAt(ctx context.Context, api *fakeAPI, e env, report func(queuedOp, string), id string) (replayResult, error) {
	wrapped := func(op queuedOp, outcome string) {
		report(op, outcome)
		if next := nextAfter(e, op); next == id {
			api.mu.Lock()
			api.failNext = []int{503}
			api.mu.Unlock()
		}
	}
	return replay(ctx, e.client, e.queue, wrapped)
}

func nextAfter(e env, op queuedOp) string {
	ops, _ := e.queue.load()
	for i, o := range ops {
		if o.transactionID() == op.transactionID() && i+1 < len(ops) {
			return ops[i+1].transactionID()
		}
	}
	return ""
}

func TestParseAmount(t *testing.T) {
	for in, want := range map[string]string{"1250": "1250.00", "99,9": "99.90", "0.5": "0.50"} {
		if got, err := parseAmount(in); err != nil || got != want {
			t.Errorf("parseAmount(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "0", "-5", "abc"} {
		if _, err := parseAmount(in); err == nil {
			t.Errorf("parseAmount(%q): want error", in)
		}
	}
}
