package procurement_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	nc "github.com/dujiao-next/internal/modules/notification/contract"
	nd "github.com/dujiao-next/internal/modules/notification/domain"
	ns "github.com/dujiao-next/internal/modules/notification/infrastructure/gormstore"
	od "github.com/dujiao-next/internal/modules/order/domain"
	os "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	pa "github.com/dujiao-next/internal/modules/procurement/application"
	pc "github.com/dujiao-next/internal/modules/procurement/contract"
	pd "github.com/dujiao-next/internal/modules/procurement/domain"
	ps "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"
	pn "github.com/dujiao-next/internal/modules/procurement/infrastructure/notificationadapter"
	pr "github.com/dujiao-next/internal/modules/procurement/infrastructure/orderreader"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type recoveryTestBalance struct {
	pc.UpstreamConnection
	amount, currency string
	reads            int
	err              error
}

func (b *recoveryTestBalance) ReadBalance(context.Context) (string, string, error) {
	b.reads++
	return b.amount, b.currency, b.err
}

type recoveryTestConnections map[uint]*recoveryTestBalance

func (c recoveryTestConnections) Open(id uint) (pc.UpstreamConnection, error) {
	if c[id] == nil {
		return nil, errors.New("offline")
	}
	return c[id], nil
}

type recoveryTestMappings map[uint]uint

func (m recoveryTestMappings) FindConnectionID(id uint) (uint, bool, error) {
	value, ok := m[id]
	return value, ok, nil
}

type recoveryTestQueue struct {
	ids []uint
	err error
}

func (q *recoveryTestQueue) EnqueueBalanceRecoveryEmail(id uint) error {
	if q.err != nil {
		return q.err
	}
	q.ids = append(q.ids, id)
	return nil
}

type recoveryTestSender struct {
	mu     sync.Mutex
	to     []string
	inputs []nc.BalanceRecoveryEmailInput
	err    error
}

func (s *recoveryTestSender) SendBalanceRecoveryEmail(to string, input nc.BalanceRecoveryEmailInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.to = append(s.to, to)
	s.inputs = append(s.inputs, input)
	return s.err
}

type recoveryFixture struct {
	sequence int
	db       *gorm.DB
	store    *ps.Store
	reader   *pr.Reader
	balances recoveryTestConnections
	queue    *recoveryTestQueue
	sender   *recoveryTestSender
	mailer   pc.BalanceRecoveryMailer
	recovery *pa.BalanceRecovery
	checkout *pa.Service
}

func newRecoveryFixture(t *testing.T) *recoveryFixture {
	t.Helper()
	db := setupProcurementTestDB(t)
	if err := db.AutoMigrate(&userdomain.User{}, &pd.BalanceWait{}, &nd.NotificationLog{}); err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sql.Close() })
	f := &recoveryFixture{db: db, store: ps.New(db), reader: pr.New(os.New(db, "balance-recovery-test")), balances: recoveryTestConnections{1: {amount: "365.64", currency: "CNY"}, 2: {amount: "0", currency: "CNY"}}, queue: &recoveryTestQueue{}, sender: &recoveryTestSender{}}
	f.mailer = pn.NewRecoveryMailer(f.sender, ns.NewLogStore(db))
	f.recovery = pa.NewBalanceRecovery(f.store, f.reader, f.balances, f.queue, f.mailer)
	f.checkout = pa.NewService(pa.Options{Orders: f.reader, ProductMappings: recoveryTestMappings{1: 1, 2: 2}, Connections: f.balances})
	f.checkout.SetBalanceRecovery(f.recovery)
	return f
}

func (f *recoveryFixture) order(t *testing.T, member bool, cost string) od.Order {
	t.Helper()
	f.sequence++
	order := od.Order{OrderNo: fmt.Sprintf("RECOVERY-%d", f.sequence), GuestEmail: "guest@example.invalid", Status: constants.OrderStatusPendingPayment, Currency: "CNY"}
	if member {
		user := userdomain.User{Email: fmt.Sprintf("member-%d@example.invalid", f.sequence), PasswordHash: "test"}
		if err := f.db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		order.UserID = user.ID
		order.GuestEmail = ""
	}
	expires := time.Now().Add(30 * time.Minute)
	order.ExpiresAt = &expires
	order.Items = []od.OrderItem{{ProductID: 1, Quantity: 1, CostPrice: money.FromDecimal(decimal.RequireFromString(cost)), TitleJSON: jsonmap.JSON{"zh-CN": "5x 测试商品"}, FulfillmentType: constants.FulfillmentTypeUpstream}}
	if err := f.db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	return order
}

func (f *recoveryFixture) block(t *testing.T, order od.Order) pd.BalanceWait {
	t.Helper()
	var block *pc.PaymentBlock
	if err := f.checkout.CheckPaymentBalance(context.Background(), order.ID); !errors.As(err, &block) || block.Code != pc.BalanceInsufficient {
		t.Fatalf("expected insufficient funds, got %v", err)
	}
	var wait pd.BalanceWait
	if err := f.db.Where("order_id = ?", order.ID).First(&wait).Error; err != nil {
		t.Fatal(err)
	}
	return wait
}

func (f *recoveryFixture) wait(t *testing.T, id uint) pd.BalanceWait {
	t.Helper()
	var value pd.BalanceWait
	if err := f.db.First(&value, id).Error; err != nil {
		t.Fatal(err)
	}
	return value
}
func mustRecovery(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBalanceRecoveryPersistsEveryCustomerAndSurvivesRestart(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	guest := f.order(t, false, "595")
	member := f.order(t, true, "600")
	w1 := f.block(t, guest)
	w2 := f.block(t, member)
	f.block(t, guest)
	var count int64
	f.db.Model(&pd.BalanceWait{}).Count(&count)
	if count != 2 {
		t.Fatal("repeat clicks duplicated or lost recipients", count)
	}
	// Operator mail is deliberately absent: its cooldown/failure cannot drop a customer.
	f.recovery = pa.NewBalanceRecovery(ps.New(f.db), pr.New(os.New(f.db, "balance-recovery-test")), f.balances, f.queue, f.mailer)
	f.balances[1].reads = 0
	mustRecovery(t, f.recovery.Poll(ctx))
	if len(f.queue.ids) != 0 || f.balances[1].reads != 1 {
		t.Fatal("low balance sent mail or supplier was queried per customer")
	}
	f.balances[1].amount = "595"
	mustRecovery(t, f.recovery.Poll(ctx))
	if len(f.queue.ids) != 1 || f.queue.ids[0] != w1.ID || f.wait(t, w2.ID).Status != pd.BalanceWaitWaiting {
		t.Fatal("wrong per-order threshold")
	}
	mustRecovery(t, f.recovery.Send(ctx, w1.ID))
	mustRecovery(t, f.recovery.Send(ctx, w1.ID))
	if len(f.sender.to) != 1 || f.sender.to[0] != "guest@example.invalid" {
		t.Fatal("wrong guest or duplicate delivery")
	}
	f.balances[1].amount = "600"
	mustRecovery(t, f.recovery.Poll(ctx))
	mustRecovery(t, f.recovery.Send(ctx, w2.ID))
	if len(f.sender.to) != 2 || f.sender.to[1] == f.sender.to[0] {
		t.Fatal("member email not resolved")
	}
	for _, id := range []uint{w1.ID, w2.ID} {
		if f.wait(t, id).Status != pd.BalanceWaitSent {
			t.Fatal("successful wait not closed")
		}
	}
	before := f.balances[1].reads
	mustRecovery(t, f.recovery.Poll(ctx))
	if f.balances[1].reads != before {
		t.Fatal("polled after all alarms ended")
	}
	var orders []od.Order
	mustRecovery(t, f.db.Find(&orders).Error)
	for _, o := range orders {
		if o.Status != constants.OrderStatusPendingPayment || o.PaidAt != nil {
			t.Fatal("recovery changed an order")
		}
	}
	f.db.Model(&pd.Order{}).Count(&count)
	if count != 0 {
		t.Fatal("recovery created procurement")
	}
}

func TestBalanceRecoveryRequiresEverySupplierAndHandlesReadFailures(t *testing.T) {
	for _, tc := range []struct {
		name, amount, currency string
		err                    error
	}{
		{"insufficient", "199.99", "CNY", nil}, {"unavailable", "200", "CNY", errors.New("timeout")}, {"malformed", "bad", "CNY", nil}, {"negative", "-1", "CNY", nil}, {"wrong_currency", "200", "USD", nil}, {"exact", "200", "CNY", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRecoveryFixture(t)
			o := f.order(t, false, "595")
			mustRecovery(t, f.db.Create(&od.OrderItem{OrderID: o.ID, ProductID: 2, Quantity: 2, TitleJSON: jsonmap.JSON{"zh-CN": "第二家测试商品"}, CostPrice: money.FromDecimal(decimal.NewFromInt(100)), FulfillmentType: constants.FulfillmentTypeUpstream}).Error)
			w := f.block(t, o)
			f.balances[1].amount = "595"
			b := f.balances[2]
			b.amount, b.currency, b.err = tc.amount, tc.currency, tc.err
			mustRecovery(t, f.recovery.Poll(context.Background()))
			ready := f.wait(t, w.ID).Status == pd.BalanceWaitReady
			if ready != (tc.name == "exact") || len(f.queue.ids) != map[bool]int{true: 1, false: 0}[ready] {
				t.Fatal("invalid recovery result", ready)
			}
		})
	}
}

func TestBalanceRecoveryQueueFailureAndSMTPRetryAreDurable(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	w := f.block(t, f.order(t, false, "595"))
	f.balances[1].amount = "595"
	f.queue.err = errors.New("redis unavailable")
	if f.recovery.Poll(ctx) == nil {
		t.Fatal("queue failure hidden")
	}
	if f.wait(t, w.ID).Status != pd.BalanceWaitReady {
		t.Fatal("lost notification after queue failure")
	}
	f.queue.err = nil
	mustRecovery(t, f.recovery.Poll(ctx))
	f.sender.err = errors.New("SMTP refused")
	if f.recovery.Send(ctx, w.ID) == nil {
		t.Fatal("mail failure hidden")
	}
	failed := f.wait(t, w.ID)
	if failed.Status != pd.BalanceWaitWaiting || failed.SentAt != nil || failed.NextAttemptAt == nil {
		t.Fatal("failure claimed success")
	}
	mustRecovery(t, f.recovery.Send(ctx, w.ID))
	if len(f.sender.to) != 1 {
		t.Fatal("immediate retry ignored cooldown")
	}
	mustRecovery(t, f.db.Model(&pd.BalanceWait{}).Where("id = ?", w.ID).Update("next_attempt_at", time.Now().UTC().Add(-time.Second)).Error)
	f.sender.err = nil
	mustRecovery(t, f.recovery.Poll(ctx))
	mustRecovery(t, f.recovery.Send(ctx, w.ID))
	if f.wait(t, w.ID).Status != pd.BalanceWaitSent || len(f.sender.to) != 2 {
		t.Fatal("mail retry did not recover")
	}
	var logs []nd.NotificationLog
	mustRecovery(t, f.db.Order("id").Find(&logs).Error)
	if len(logs) != 2 || logs[0].Status != "failed" || logs[1].Status != "success" {
		t.Fatal("actual results missing from notification log")
	}
}

func TestBalanceRecoveryStopsForPaidCanceledAndChangedRecipients(t *testing.T) {
	for _, mode := range []string{"paid", "cancelled", "changed_email", "deleted", "expired"} {
		t.Run(mode, func(t *testing.T) {
			f := newRecoveryFixture(t)
			ctx := context.Background()
			o := f.order(t, false, "595")
			w := f.block(t, o)
			now := time.Now().UTC()
			updates := map[string]interface{}{}
			switch mode {
			case "paid":
				updates["status"], updates["paid_at"] = constants.OrderStatusPaid, now
			case "cancelled":
				updates["status"], updates["canceled_at"] = constants.OrderStatusCanceled, now
			case "changed_email":
				updates["guest_email"] = "new@example.invalid"
			case "deleted":
				updates["deleted_at"] = now
			case "expired":
				updates["status"], updates["canceled_at"], updates["expires_at"] = constants.OrderStatusCanceled, now, now.Add(-time.Second)
			}
			mustRecovery(t, f.db.Model(&od.Order{}).Where("id = ?", o.ID).Updates(updates).Error)
			f.balances[1].amount = "595"
			mustRecovery(t, f.recovery.Poll(ctx))
			mustRecovery(t, f.recovery.Send(ctx, w.ID))
			if mode == "expired" {
				if len(f.sender.to) != 1 {
					t.Fatal("timeout lost return-to-store reminder")
				}
			} else if len(f.sender.to) != 0 || f.wait(t, w.ID).Status != pd.BalanceWaitClosed {
				t.Fatal("ineligible order received reminder")
			}
		})
	}
}

func TestBalanceRecoveryRechecksOrderAtSendAndRejectsStaleSnapshot(t *testing.T) {
	for _, mode := range []string{"paid", "old_balance", "balance_dropped"} {
		t.Run(mode, func(t *testing.T) {
			f := newRecoveryFixture(t)
			ctx := context.Background()
			o := f.order(t, false, "595")
			w := f.block(t, o)
			f.balances[1].amount = "595"
			mustRecovery(t, f.recovery.Poll(ctx))
			switch mode {
			case "paid":
				mustRecovery(t, f.db.Model(&od.Order{}).Where("id = ?", o.ID).Update("status", constants.OrderStatusPaid).Error)
			case "old_balance":
				mustRecovery(t, f.db.Model(&pd.BalanceWait{}).Where("id = ?", w.ID).Update("recovered_at", time.Now().Add(-2*time.Minute)).Error)
			case "balance_dropped":
				f.balances[1].amount = "0"
				mustRecovery(t, f.recovery.Poll(ctx))
			}
			_ = f.recovery.Send(ctx, w.ID)
			if len(f.sender.to) != 0 {
				t.Fatal("sent obsolete recovery notice")
			}
		})
	}
}

func TestBalanceRecoveryConcurrentAndRedeliveredJobsDoNotResend(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	w := f.block(t, f.order(t, false, "595"))
	f.balances[1].amount = "595"
	mustRecovery(t, f.recovery.Poll(ctx))
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- f.recovery.Send(ctx, w.ID) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		mustRecovery(t, err)
	}
	if len(f.sender.to) != 1 {
		t.Fatal("concurrent workers duplicated email", len(f.sender.to))
	}
	// Simulate loss of the final wait update after SMTP and its audit succeeded.
	mustRecovery(t, f.db.Model(&pd.BalanceWait{}).Where("id = ?", w.ID).Updates(map[string]interface{}{"status": pd.BalanceWaitReady, "sent_at": nil}).Error)
	mustRecovery(t, f.recovery.Send(ctx, w.ID))
	if len(f.sender.to) != 1 {
		t.Fatal("successful audit was ignored on redelivery")
	}
}

type recoveryBrokenAudit struct{ pn.RecoveryLogs }

func (recoveryBrokenAudit) Create(*nd.NotificationLog) error {
	return errors.New("audit storage unavailable")
}

func TestBalanceRecoveryAcceptedEmailIsNotRetriedOnAuditFailure(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	w := f.block(t, f.order(t, false, "595"))
	f.balances[1].amount = "595"
	f.mailer = pn.NewRecoveryMailer(f.sender, recoveryBrokenAudit{ns.NewLogStore(f.db)})
	f.recovery = pa.NewBalanceRecovery(f.store, f.reader, f.balances, f.queue, f.mailer)
	mustRecovery(t, f.recovery.Poll(ctx))
	if err := f.recovery.Send(ctx, w.ID); !errors.Is(err, pc.ErrBalanceRecoveryMailAccepted) {
		t.Fatal("accepted delivery was misclassified", err)
	}
	mustRecovery(t, f.recovery.Send(ctx, w.ID))
	if len(f.sender.to) != 1 || f.wait(t, w.ID).Status != pd.BalanceWaitSent {
		t.Fatal("audit failure resent accepted email")
	}
}

func TestBalanceRecoveryExpiredWorkerLeaseRecovers(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	w := f.block(t, f.order(t, false, "595"))
	f.balances[1].amount = "595"
	mustRecovery(t, f.recovery.Poll(ctx))
	claimed, err := f.store.ClaimBalanceMail(ctx, w.ID, "dead-worker", time.Now().UTC())
	mustRecovery(t, err)
	if claimed == nil {
		t.Fatal("claim missing")
	}
	mustRecovery(t, f.recovery.Send(ctx, w.ID))
	if len(f.sender.to) != 0 {
		t.Fatal("active lease was bypassed")
	}
	mustRecovery(t, f.db.Model(&pd.BalanceWait{}).Where("id = ?", w.ID).Update("lease_until", time.Now().UTC().Add(-time.Second)).Error)
	mustRecovery(t, f.recovery.Poll(ctx))
	mustRecovery(t, f.recovery.Send(ctx, w.ID))
	if len(f.sender.to) != 1 || f.wait(t, w.ID).LeaseToken != "" {
		t.Fatal("crashed worker did not recover")
	}
}

func TestBalanceRecoveryInvalidRecipientsAndTenantOrdersAreNotRegistered(t *testing.T) {
	for _, mode := range []string{"no_email", "invalid_email", "tenant"} {
		t.Run(mode, func(t *testing.T) {
			f := newRecoveryFixture(t)
			o := f.order(t, false, "595")
			update := map[string]interface{}{"guest_email": ""}
			if mode == "invalid_email" {
				update["guest_email"] = "bad\r\nBcc: victim@example.invalid"
			}
			if mode == "tenant" {
				update = map[string]interface{}{"reseller_id": 9}
			}
			mustRecovery(t, f.db.Model(&od.Order{}).Where("id = ?", o.ID).Updates(update).Error)
			var block *pc.PaymentBlock
			if err := f.checkout.CheckPaymentBalance(context.Background(), o.ID); !errors.As(err, &block) {
				t.Fatal("payment safety guard was lost", err)
			}
			var count int64
			mustRecovery(t, f.db.Model(&pd.BalanceWait{}).Count(&count).Error)
			if count != 0 {
				t.Fatal("registered an unsafe recipient or another storefront")
			}
		})
	}
}

func TestBalanceRecoveryPaginationSharesOneSupplierRead(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	for i := 0; i < 103; i++ {
		o := f.order(t, false, "595")
		mustRecovery(t, f.recovery.Record(ctx, o.ID, []pd.FundingRequirement{{ConnectionID: 1, Amount: "595", Currency: "CNY"}}))
	}
	f.balances[1].amount = "595"
	mustRecovery(t, f.recovery.Poll(ctx))
	if len(f.queue.ids) != 103 || f.balances[1].reads != 1 {
		t.Fatal("lost a later page or repeated supplier queries", len(f.queue.ids), f.balances[1].reads)
	}
}
