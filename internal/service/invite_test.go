package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
	"github.com/MirzaDgtu/PromoGo/internal/notifytemplates"
)

// fakeInviteStoreRepo is the slice of domain.StoreRepository appInvite
// reads: GetByID.
type fakeInviteStoreRepo struct {
	domain.StoreRepository
	byID map[int64]*domain.Store
}

func (f *fakeInviteStoreRepo) GetByID(_ context.Context, id int64) (*domain.Store, error) {
	s, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

// fakeInviteCounter counts the app invites fakeLedgerRepo has queued for a
// store — the real repository counts the same rows in notification_outbox.
type fakeInviteCounter struct {
	domain.NotificationOutboxRepository
	ledger  *fakeLedgerRepo
	clients *fakeClientRepo
	err     error
}

func (f *fakeInviteCounter) CountByStoreTemplateSince(_ context.Context, storeID int64, templateID string, _ time.Time) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	var n int64
	for _, e := range f.ledger.outbox {
		if e.TemplateID == templateID && f.clients.byID[e.ClientID].StoreID == storeID {
			n++
		}
	}
	return n, nil
}

type inviteTestDeps struct {
	*testDeps
	stores   *fakeInviteStoreRepo
	accounts *fakeCustomerAccountRepo
	counter  *fakeInviteCounter
}

const testAppURL = "https://promogo.test/app"

// newInviteTestService builds a LoyaltyService with invites on and two
// stores: 1 «Океан» (invites on) and 2 «Залив» (invites on).
func newInviteTestService(limit int64) *inviteTestDeps {
	deps := newTestService(pointsConfig(1))
	stores := &fakeInviteStoreRepo{byID: map[int64]*domain.Store{
		1: {ID: 1, Name: "Океан", SMSInviteEnabled: true},
		2: {ID: 2, Name: "Залив", SMSInviteEnabled: true},
	}}
	accounts := newFakeCustomerAccountRepo()
	counter := &fakeInviteCounter{ledger: deps.ledger, clients: deps.clients}
	deps.svc.invites = Invites{
		AppURL: testAppURL, DailyStoreLimit: limit,
		Stores: stores, CustomerAccounts: accounts, Outbox: counter,
	}
	return &inviteTestDeps{testDeps: deps, stores: stores, accounts: accounts, counter: counter}
}

func (d *inviteTestDeps) accrue(t *testing.T, storeID int64, txID, phone string) *AccrueResult {
	t.Helper()
	// Store 2 has no loyalty config: the sale is recorded at 0 points
	// (DEC-028), which must not stop the invite.
	if storeID != 1 {
		d.configs.cfg = nil
		defer func() { d.configs.cfg = pointsConfig(1) }()
	}
	res, err := d.svc.Accrue(context.Background(), AccrueRequest{StoreID: storeID, ExternalTxID: txID, Phone: phone, Amount: decimal.NewFromInt(100)})
	if err != nil {
		t.Fatalf("Accrue(%s) error = %v", txID, err)
	}
	return res
}

func (d *inviteTestDeps) invites() []*domain.NotificationOutboxEntry {
	var out []*domain.NotificationOutboxEntry
	for _, e := range d.ledger.outbox {
		if e.TemplateID == notifytemplates.AppInviteID {
			out = append(out, e)
		}
	}
	return out
}

func TestAccrue_NewClientGetsSMSInvite(t *testing.T) {
	d := newInviteTestService(10)
	res := d.accrue(t, 1, "inv-1", "8 (999) 000-00-01")

	invites := d.invites()
	if len(invites) != 1 {
		t.Fatalf("invites = %d, want 1", len(invites))
	}
	inv := invites[0]
	if inv.Channel != domain.OutboxChannelSMS || inv.ClientID != res.ClientID {
		t.Fatalf("invite = %+v, want sms to client %d", inv, res.ClientID)
	}
	if inv.DedupeKey != "invite:+79990000001" {
		t.Fatalf("DedupeKey = %q, want the normalized phone", inv.DedupeKey)
	}
	if !strings.Contains(inv.Message, "Океан") || !strings.Contains(inv.Message, testAppURL) {
		t.Fatalf("Message = %q, want the store name and app link", inv.Message)
	}
	// The accrual push is still queued alongside.
	if len(d.ledger.outbox) != 2 {
		t.Fatalf("outbox = %d entries, want the accrual push and the invite", len(d.ledger.outbox))
	}
}

func TestAccrue_InviteOncePerPhone(t *testing.T) {
	d := newInviteTestService(10)
	d.accrue(t, 1, "inv-1", "+79990000001")
	d.accrue(t, 1, "inv-2", "+79990000001") // returning client
	d.accrue(t, 2, "inv-3", "+79990000001") // same phone, another store

	if n := len(d.invites()); n != 1 {
		t.Fatalf("invites = %d, want 1 per phone", n)
	}
}

func TestAccrue_NoInvite(t *testing.T) {
	tests := []struct {
		name  string
		setup func(d *inviteTestDeps)
		phone string
	}{
		{name: "store has invites off", phone: "+79990000001", setup: func(d *inviteTestDeps) { d.stores.byID[1].SMSInviteEnabled = false }},
		{name: "invites off in config", phone: "+79990000001", setup: func(d *inviteTestDeps) { d.svc.invites.AppURL = "" }},
		{name: "phone already has the app", phone: "+79990000001", setup: func(d *inviteTestDeps) {
			_ = d.accounts.Create(context.Background(), &domain.CustomerAccount{Phone: "+79990000001"})
		}},
		{name: "phone not normalizable", phone: "card-0042", setup: func(*inviteTestDeps) {}},
		{name: "counting invites fails", phone: "+79990000001", setup: func(d *inviteTestDeps) { d.counter.err = errors.New("db down") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newInviteTestService(10)
			tt.setup(d)
			res := d.accrue(t, 1, "inv-1", tt.phone)
			if n := len(d.invites()); n != 0 {
				t.Fatalf("invites = %d, want 0", n)
			}
			if res.PointsEarned != 10 {
				t.Fatalf("PointsEarned = %d, want the sale accrued as usual", res.PointsEarned)
			}
		})
	}
}

func TestAccrue_InviteDailyStoreLimit(t *testing.T) {
	d := newInviteTestService(2)
	d.accrue(t, 1, "inv-1", "+79990000001")
	d.accrue(t, 1, "inv-2", "+79990000002")
	d.accrue(t, 1, "inv-3", "+79990000003") // over the cap
	d.accrue(t, 2, "inv-4", "+79990000004") // another store has its own cap

	if n := len(d.invites()); n != 3 {
		t.Fatalf("invites = %d, want 2 for store 1 and 1 for store 2", n)
	}
}

func TestOutboxWorker_SMSEntryGoesToClientPhone(t *testing.T) {
	outbox := newFakeOutboxRepo()
	channel := &fakeChannel{}
	sms := &fakeSMSSender{}
	clients := newFakeClientRepo()
	client := &domain.Client{StoreID: 1, Phone: "+79990000001"}
	_ = clients.Create(context.Background(), client)
	entry := outbox.add(&domain.NotificationOutboxEntry{ClientID: client.ID, Channel: domain.OutboxChannelSMS, Message: "invite", DedupeKey: "invite:+79990000001"})

	w := NewOutboxWorker(testOutboxLogger(), outbox, channel, sms, clients, OutboxWorkerConfig{
		BatchSize: 10, MaxConcurrency: 4, MaxAttempts: 3, BaseBackoff: time.Second, MaxBackoff: time.Minute,
	})
	w.pollOnce(context.Background())

	if got := outbox.get(entry.ID); got.Status != domain.NotificationOutboxDelivered {
		t.Fatalf("status = %q, want delivered", got.Status)
	}
	if len(sms.phones) != 1 || sms.phones[0] != "+79990000001" || sms.messages[0] != "invite" {
		t.Fatalf("sms sent = %v %v, want one invite to +79990000001", sms.phones, sms.messages)
	}
	if len(channel.sent) != 0 {
		t.Fatalf("push sent = %v, want none", channel.sent)
	}
}
