package fcmchannel

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"firebase.google.com/go/v4/messaging"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

type fakeClientRepo struct {
	byID map[int64]*domain.Client
}

func (f *fakeClientRepo) GetByPhone(context.Context, int64, string) (*domain.Client, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeClientRepo) GetByID(_ context.Context, id int64) (*domain.Client, error) {
	c, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}
func (f *fakeClientRepo) Create(context.Context, *domain.Client) error { return nil }
func (f *fakeClientRepo) ListUnlinkedByPhone(context.Context, string) ([]*domain.Client, error) {
	return nil, nil
}
func (f *fakeClientRepo) LinkCustomerAccount(context.Context, int64, int64) error { return nil }
func (f *fakeClientRepo) ListByCustomerAccount(context.Context, int64) ([]*domain.Client, error) {
	return nil, nil
}

type fakeDeviceRepo struct {
	active  map[int64][]*domain.CustomerDevice
	revoked []string
}

func (f *fakeDeviceRepo) Upsert(context.Context, *domain.CustomerDevice) error { return nil }
func (f *fakeDeviceRepo) ListActiveByCustomerAccount(_ context.Context, customerAccountID int64) ([]*domain.CustomerDevice, error) {
	return f.active[customerAccountID], nil
}
func (f *fakeDeviceRepo) Revoke(context.Context, int64, int64) error { return nil }
func (f *fakeDeviceRepo) RevokeByToken(_ context.Context, pushToken string) error {
	f.revoked = append(f.revoked, pushToken)
	return nil
}

// fakeMessagingSender mirrors messaging.Client.SendEach's two-level failure
// mode: err simulates a total batch failure (SendEach itself returns an
// error, nothing in the batch delivered), while perMsgErr simulates a
// per-message failure inside an otherwise-successful batch call (a
// BatchResponse with that message's SendResponse.Success=false) — the
// shape Channel.Send actually branches on for invalid-token revocation.
type fakeMessagingSender struct {
	err       error
	perMsgErr error
	sentCount int
}

func (f *fakeMessagingSender) SendEach(_ context.Context, messages []*messaging.Message) (*messaging.BatchResponse, error) {
	f.sentCount += len(messages)
	if f.err != nil {
		return nil, f.err
	}
	resp := &messaging.BatchResponse{}
	for range messages {
		if f.perMsgErr != nil {
			resp.Responses = append(resp.Responses, &messaging.SendResponse{Success: false, Error: f.perMsgErr})
			resp.FailureCount++
			continue
		}
		resp.Responses = append(resp.Responses, &messaging.SendResponse{Success: true, MessageID: "message-id"})
		resp.SuccessCount++
	}
	return resp, nil
}

func newTestChannel(sender messagingSender, clients *fakeClientRepo, devices *fakeDeviceRepo, isInvalidToken func(error) bool) *Channel {
	return &Channel{
		log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		client:         sender,
		clients:        clients,
		devices:        devices,
		isInvalidToken: isInvalidToken,
	}
}

var errTransient = errors.New("transient fcm error")

func TestFCMChannel_Send_Success(t *testing.T) {
	accountID := int64(42)
	clients := &fakeClientRepo{byID: map[int64]*domain.Client{1: {ID: 1, CustomerAccountID: &accountID}}}
	devices := &fakeDeviceRepo{active: map[int64][]*domain.CustomerDevice{accountID: {{ID: 10, PushToken: "tok-1"}}}}
	sender := &fakeMessagingSender{}

	ch := newTestChannel(sender, clients, devices, isInvalidTokenError)
	if err := ch.Send(context.Background(), 1, "hello"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if sender.sentCount != 1 {
		t.Fatalf("sentCount = %d, want 1", sender.sentCount)
	}
	if len(devices.revoked) != 0 {
		t.Fatalf("revoked = %v, want none", devices.revoked)
	}
}

func TestFCMChannel_Send_InvalidTokenIsRevoked(t *testing.T) {
	accountID := int64(43)
	clients := &fakeClientRepo{byID: map[int64]*domain.Client{1: {ID: 1, CustomerAccountID: &accountID}}}
	devices := &fakeDeviceRepo{active: map[int64][]*domain.CustomerDevice{accountID: {{ID: 11, PushToken: "dead-token"}}}}
	sender := &fakeMessagingSender{perMsgErr: errTransient}

	// isInvalidToken stands in for messaging.IsRegistrationTokenNotRegistered/
	// IsInvalidArgument, which inspect an unexported SDK error type that
	// can't be constructed from outside firebase.google.com/go/v4 — this
	// tests Channel's own revoke-on-invalid-token branching, not the SDK's
	// error classification.
	ch := newTestChannel(sender, clients, devices, func(error) bool { return true })

	if err := ch.Send(context.Background(), 1, "hello"); err != nil {
		t.Fatalf("Send() error = %v, want nil (best-effort, must never fail the caller)", err)
	}
	if len(devices.revoked) != 1 || devices.revoked[0] != "dead-token" {
		t.Fatalf("revoked = %v, want [dead-token]", devices.revoked)
	}
}

func TestFCMChannel_Send_TransientErrorNotRevoked(t *testing.T) {
	accountID := int64(44)
	clients := &fakeClientRepo{byID: map[int64]*domain.Client{1: {ID: 1, CustomerAccountID: &accountID}}}
	devices := &fakeDeviceRepo{active: map[int64][]*domain.CustomerDevice{accountID: {{ID: 12, PushToken: "tok-2"}}}}
	sender := &fakeMessagingSender{perMsgErr: errTransient}

	ch := newTestChannel(sender, clients, devices, func(error) bool { return false })

	if err := ch.Send(context.Background(), 1, "hello"); err != nil {
		t.Fatalf("Send() error = %v, want nil (best-effort)", err)
	}
	if len(devices.revoked) != 0 {
		t.Fatalf("revoked = %v, want none — a transient error must not revoke a possibly-valid token", devices.revoked)
	}
}

func TestFCMChannel_Send_NoLinkedAccountIsNoOp(t *testing.T) {
	clients := &fakeClientRepo{byID: map[int64]*domain.Client{1: {ID: 1, CustomerAccountID: nil}}}
	devices := &fakeDeviceRepo{}
	sender := &fakeMessagingSender{}

	ch := newTestChannel(sender, clients, devices, isInvalidTokenError)
	if err := ch.Send(context.Background(), 1, "hello"); err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if sender.sentCount != 0 {
		t.Fatalf("sentCount = %d, want 0 (no linked account, nothing to push to)", sender.sentCount)
	}
}

func TestFCMChannel_Send_NoDevicesIsNoOp(t *testing.T) {
	accountID := int64(45)
	clients := &fakeClientRepo{byID: map[int64]*domain.Client{1: {ID: 1, CustomerAccountID: &accountID}}}
	devices := &fakeDeviceRepo{} // no active devices for this account
	sender := &fakeMessagingSender{}

	ch := newTestChannel(sender, clients, devices, isInvalidTokenError)
	if err := ch.Send(context.Background(), 1, "hello"); err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if sender.sentCount != 0 {
		t.Fatalf("sentCount = %d, want 0", sender.sentCount)
	}
}

// TestFCMChannel_Send_TotalBatchFailureIsBestEffort covers SendEach's
// documented total-failure mode (an error from SendEach itself, not a
// per-message failure inside the batch) — Channel.Send must still honor
// domain.NotificationChannel's best-effort contract rather than
// propagating it to the caller.
func TestFCMChannel_Send_TotalBatchFailureIsBestEffort(t *testing.T) {
	accountID := int64(46)
	clients := &fakeClientRepo{byID: map[int64]*domain.Client{1: {ID: 1, CustomerAccountID: &accountID}}}
	devices := &fakeDeviceRepo{active: map[int64][]*domain.CustomerDevice{accountID: {{ID: 13, PushToken: "tok-3"}}}}
	sender := &fakeMessagingSender{err: errTransient}

	ch := newTestChannel(sender, clients, devices, isInvalidTokenError)
	if err := ch.Send(context.Background(), 1, "hello"); err != nil {
		t.Fatalf("Send() error = %v, want nil (best-effort even on a total SendEach failure)", err)
	}
}

// TestFCMChannel_Send_MultipleDevicesOneBatchCall guards Phase 3's
// batch/parallelize requirement (docs/audit-remediation-prompt.md): fanning
// out to several devices must be one SendEach call carrying every message,
// not a sequential per-device loop.
func TestFCMChannel_Send_MultipleDevicesOneBatchCall(t *testing.T) {
	accountID := int64(47)
	clients := &fakeClientRepo{byID: map[int64]*domain.Client{1: {ID: 1, CustomerAccountID: &accountID}}}
	devices := &fakeDeviceRepo{active: map[int64][]*domain.CustomerDevice{accountID: {
		{ID: 14, PushToken: "tok-a"}, {ID: 15, PushToken: "tok-b"}, {ID: 16, PushToken: "tok-c"},
	}}}
	sender := &fakeCallCountingSender{}

	ch := newTestChannel(sender, clients, devices, isInvalidTokenError)
	if err := ch.Send(context.Background(), 1, "hello"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if sender.calls != 1 {
		t.Fatalf("SendEach calls = %d, want 1 (one batch call for all 3 devices, not one call per device)", sender.calls)
	}
	if sender.lastBatchSize != 3 {
		t.Fatalf("last batch size = %d, want 3", sender.lastBatchSize)
	}
}

type fakeCallCountingSender struct {
	calls         int
	lastBatchSize int
}

func (f *fakeCallCountingSender) SendEach(_ context.Context, messages []*messaging.Message) (*messaging.BatchResponse, error) {
	f.calls++
	f.lastBatchSize = len(messages)
	resp := &messaging.BatchResponse{SuccessCount: len(messages)}
	for range messages {
		resp.Responses = append(resp.Responses, &messaging.SendResponse{Success: true, MessageID: "message-id"})
	}
	return resp, nil
}

func TestFCMChannel_Send_ClientLookupErrorPropagates(t *testing.T) {
	clients := &fakeClientRepo{byID: map[int64]*domain.Client{}}
	devices := &fakeDeviceRepo{}
	sender := &fakeMessagingSender{}

	ch := newTestChannel(sender, clients, devices, isInvalidTokenError)
	if err := ch.Send(context.Background(), 999, "hello"); err == nil {
		t.Fatal("Send() error = nil, want an error when the client itself can't be loaded")
	}
}
