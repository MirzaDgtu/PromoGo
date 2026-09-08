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

type fakeMessagingSender struct {
	err       error
	sentCount int
}

func (f *fakeMessagingSender) Send(context.Context, *messaging.Message) (string, error) {
	f.sentCount++
	if f.err != nil {
		return "", f.err
	}
	return "message-id", nil
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
	sender := &fakeMessagingSender{err: errTransient}

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
	sender := &fakeMessagingSender{err: errTransient}

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

func TestFCMChannel_Send_ClientLookupErrorPropagates(t *testing.T) {
	clients := &fakeClientRepo{byID: map[int64]*domain.Client{}}
	devices := &fakeDeviceRepo{}
	sender := &fakeMessagingSender{}

	ch := newTestChannel(sender, clients, devices, isInvalidTokenError)
	if err := ch.Send(context.Background(), 999, "hello"); err == nil {
		t.Fatal("Send() error = nil, want an error when the client itself can't be loaded")
	}
}
