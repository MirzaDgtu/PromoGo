package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/MirzaDgtu/PromoGo/internal/auth"
	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

// fakeMeClientRepo is an in-memory domain.ClientRepository for me.go tests.
type fakeMeClientRepo struct {
	byCustomerAccount map[int64][]*domain.Client
}

func (f *fakeMeClientRepo) GetByPhone(context.Context, int64, string) (*domain.Client, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeMeClientRepo) GetByID(context.Context, int64) (*domain.Client, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeMeClientRepo) Create(context.Context, *domain.Client) error { return nil }
func (f *fakeMeClientRepo) ListUnlinkedByPhone(context.Context, string) ([]*domain.Client, error) {
	return nil, nil
}
func (f *fakeMeClientRepo) LinkCustomerAccount(context.Context, int64, int64) error { return nil }
func (f *fakeMeClientRepo) ListByCustomerAccount(_ context.Context, customerAccountID int64) ([]*domain.Client, error) {
	return f.byCustomerAccount[customerAccountID], nil
}

// fakeMeTransactionRepo is an in-memory domain.TransactionRepository for
// me.go tests. ListByClientIDs replicates the Postgres query's semantics
// (filter by client_id, keyset-filter by cursor, sort, limit) in memory.
type fakeMeTransactionRepo struct {
	txs            []*domain.Transaction
	listByIDsCalls int
}

func (f *fakeMeTransactionRepo) GetByExternalID(context.Context, int64, domain.TransactionType, string) (*domain.Transaction, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeMeTransactionRepo) ListByClient(context.Context, int64) ([]*domain.Transaction, error) {
	return nil, nil
}
func (f *fakeMeTransactionRepo) ListByClientIDs(_ context.Context, clientIDs []int64, limit int, before *domain.TransactionCursor) ([]*domain.Transaction, error) {
	f.listByIDsCalls++

	idSet := make(map[int64]bool, len(clientIDs))
	for _, id := range clientIDs {
		idSet[id] = true
	}

	var matched []*domain.Transaction
	for _, tx := range f.txs {
		if !idSet[tx.ClientID] {
			continue
		}
		if before != nil {
			if tx.CreatedAt.After(before.CreatedAt) {
				continue
			}
			if tx.CreatedAt.Equal(before.CreatedAt) && tx.ID >= before.ID {
				continue
			}
		}
		matched = append(matched, tx)
	}

	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID > matched[j].ID
	})

	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

func newMeTransactionsRequest(t *testing.T, customerAccountID int64, query string) *http.Request {
	t.Helper()
	token, err := auth.IssueCustomerAccessToken(testCustomerSecret, customerAccountID, time.Minute)
	if err != nil {
		t.Fatalf("IssueCustomerAccessToken() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/transactions"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

type meTransactionsResponse struct {
	Transactions []meTransactionItem `json:"transactions"`
	NextCursor   string              `json:"next_cursor"`
}

func decodeMeTransactionsResponse(t *testing.T, rec *httptest.ResponseRecorder) meTransactionsResponse {
	t.Helper()
	var resp meTransactionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
	return resp
}

func seedTransactions(clientID int64, n int, base time.Time) []*domain.Transaction {
	txs := make([]*domain.Transaction, 0, n)
	for i := 0; i < n; i++ {
		txs = append(txs, &domain.Transaction{
			ID:        int64(i + 1),
			StoreID:   1,
			ClientID:  clientID,
			Type:      domain.TransactionAccrual,
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}
	return txs
}

func TestHandleGetMyTransactions_FirstPageHasNextCursor(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clients := &fakeMeClientRepo{byCustomerAccount: map[int64][]*domain.Client{
		1: {{ID: 10, StoreID: 1}, {ID: 20, StoreID: 2}},
	}}
	txRepo := &fakeMeTransactionRepo{txs: append(seedTransactions(10, 3, base), seedTransactions(20, 3, base)...)}

	handler := RequireCustomerSession(testCustomerSecret)(handleGetMyTransactions(clients, txRepo, testLogger()))
	rec := httptest.NewRecorder()
	handler(rec, newMeTransactionsRequest(t, 1, "?limit=4"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	resp := decodeMeTransactionsResponse(t, rec)
	if len(resp.Transactions) != 4 {
		t.Fatalf("len(Transactions) = %d, want 4", len(resp.Transactions))
	}
	if resp.NextCursor == "" {
		t.Fatal("expected next_cursor to be set (6 total transactions, limit=4)")
	}
	for i := 1; i < len(resp.Transactions); i++ {
		if resp.Transactions[i].CreatedAt.After(resp.Transactions[i-1].CreatedAt) {
			t.Fatalf("transactions not newest-first at index %d", i)
		}
	}
}

func TestHandleGetMyTransactions_SecondPageExhausts(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clients := &fakeMeClientRepo{byCustomerAccount: map[int64][]*domain.Client{
		1: {{ID: 10, StoreID: 1}},
	}}
	txRepo := &fakeMeTransactionRepo{txs: seedTransactions(10, 5, base)}
	handler := RequireCustomerSession(testCustomerSecret)(handleGetMyTransactions(clients, txRepo, testLogger()))

	rec1 := httptest.NewRecorder()
	handler(rec1, newMeTransactionsRequest(t, 1, "?limit=3"))
	first := decodeMeTransactionsResponse(t, rec1)
	if len(first.Transactions) != 3 || first.NextCursor == "" {
		t.Fatalf("first page = %+v, want 3 items with a next_cursor", first)
	}

	rec2 := httptest.NewRecorder()
	handler(rec2, newMeTransactionsRequest(t, 1, "?limit=3&cursor="+first.NextCursor))
	second := decodeMeTransactionsResponse(t, rec2)
	if len(second.Transactions) != 2 {
		t.Fatalf("second page len = %d, want 2", len(second.Transactions))
	}
	if second.NextCursor != "" {
		t.Fatalf("expected no next_cursor once history is exhausted, got %q", second.NextCursor)
	}

	if len(first.Transactions)+len(second.Transactions) != 5 {
		t.Fatalf("total across both pages = %d, want 5 (no overlap/gap)", len(first.Transactions)+len(second.Transactions))
	}
}

func TestHandleGetMyTransactions_InvalidCursorRejected(t *testing.T) {
	clients := &fakeMeClientRepo{byCustomerAccount: map[int64][]*domain.Client{1: {{ID: 10}}}}
	txRepo := &fakeMeTransactionRepo{}
	handler := RequireCustomerSession(testCustomerSecret)(handleGetMyTransactions(clients, txRepo, testLogger()))

	rec := httptest.NewRecorder()
	handler(rec, newMeTransactionsRequest(t, 1, "?cursor=not-valid-base64!!"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleGetMyTransactions_NoLinkedClientsSkipsQuery(t *testing.T) {
	clients := &fakeMeClientRepo{byCustomerAccount: map[int64][]*domain.Client{}}
	txRepo := &fakeMeTransactionRepo{}
	handler := RequireCustomerSession(testCustomerSecret)(handleGetMyTransactions(clients, txRepo, testLogger()))

	rec := httptest.NewRecorder()
	handler(rec, newMeTransactionsRequest(t, 1, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	resp := decodeMeTransactionsResponse(t, rec)
	if len(resp.Transactions) != 0 {
		t.Fatalf("len(Transactions) = %d, want 0", len(resp.Transactions))
	}
	if txRepo.listByIDsCalls != 0 {
		t.Errorf("listByIDsCalls = %d, want 0 (should skip the query with no linked clients)", txRepo.listByIDsCalls)
	}
}

func TestTransactionCursor_RoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 3, 4, 5, 6, 7, 890, time.UTC)
	encoded := encodeTransactionCursor(createdAt, 42)

	decoded, err := decodeTransactionCursor(encoded)
	if err != nil {
		t.Fatalf("decodeTransactionCursor() error = %v", err)
	}
	if !decoded.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt = %v, want %v", decoded.CreatedAt, createdAt)
	}
	if decoded.ID != 42 {
		t.Errorf("ID = %d, want 42", decoded.ID)
	}
}

func TestDecodeTransactionCursor_Empty(t *testing.T) {
	decoded, err := decodeTransactionCursor("")
	if err != nil {
		t.Fatalf("decodeTransactionCursor(\"\") error = %v", err)
	}
	if decoded != nil {
		t.Errorf("decoded = %+v, want nil", decoded)
	}
}
