package integrationtest

import (
	"context"
	"encoding/json"
	"errors"
	app "github.com/dujiao-next/internal/modules/autorecharge/application"
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"github.com/dujiao-next/internal/modules/autorecharge/domain"
	"github.com/dujiao-next/internal/modules/autorecharge/infrastructure/gormstore"
	rechargehttp "github.com/dujiao-next/internal/modules/autorecharge/transport/http"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const session = `{"accessToken":"test-access-sensitive","sessionToken":"test-session-sensitive"}`

func TestUserRoutesKeepOrderNumberAndHideMerchantVerification(t *testing.T) {
	f := setup(t, nil)
	r := gin.New()
	user := uint(7)
	r.Use(func(c *gin.Context) { c.Set("user_id", user); c.Next() })
	r.GET("/orders/:order_no", func(c *gin.Context) { c.Status(200) })
	rechargehttp.RegisterUserRoutes(r, rechargehttp.New(f.s))
	for _, tc := range []struct {
		path   string
		user   uint
		status int
	}{
		{"/orders/ORDER1/auto-recharge", 7, 200},
		{"/orders/ORDER1/auto-recharge", 8, 404},
		{"/orders/ORDER1/auto-recharge/verification", 7, 404},
	} {
		user = tc.user
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != tc.status {
			t.Fatalf("%s status %d", tc.path, w.Code)
		}
	}
	user = 7
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/orders/ORDER1/auto-recharge/session", strings.NewReader(`{"session":"ignored","authorized":false}`)))
	if w.Code != 404 {
		t.Fatal("legacy Session route still available")
	}
}

type orders struct {
	paid bool
	done int
	code string
}

func (o *orders) Resolve(_ context.Context, no string, user uint) (uint, error) {
	if no != "ORDER1" || user != 7 {
		return 0, contract.ErrNotFound
	}
	return 1, nil
}

func (o *orders) Get(context.Context, uint) (*contract.Order, error) {
	return &contract.Order{ID: 1, UserID: 7, Paid: o.paid, Eligible: true, SKUID: 9}, nil
}
func (o *orders) Children(context.Context, uint) ([]uint, error)           { return nil, nil }
func (o *orders) Unscheduled(context.Context, []uint, int) ([]uint, error) { return []uint{1}, nil }
func (o *orders) DeliverCode(_ context.Context, _ uint, code string) error {
	if o.code == "" {
		o.done++
		o.code = code
	}
	return nil
}

type cards struct{}

func (cards) Reserve(context.Context, string, string) (string, error) { return "card-reference", nil }
func (cards) Credentials(context.Context, string) (contract.Card, error) {
	return contract.Card{Number: "4242424242424242", ExpMonth: "12", ExpYear: "2030", CVC: "123"}, nil
}

type partner struct {
	account    *contract.Account
	inspectErr error
	calls      int
	checks     int
	payErr     error
	result     *contract.Result
	active     bool
	finalize   contract.FinalizeRequest
	request    contract.PayRequest
	key        string
}

func (p *partner) InspectAccount(context.Context, string) (*contract.Account, error) {
	if p.inspectErr != nil {
		return nil, p.inspectErr
	}
	if p.account != nil {
		return p.account, nil
	}
	return &contract.Account{ID: "account_1", Plan: "free"}, nil
}

func (p *partner) Ready() bool                                   { return true }
func (p *partner) Check(context.Context, contract.Binding) error { p.checks++; return nil }
func (p *partner) Pay(_ context.Context, key string, r contract.PayRequest) (*contract.Result, error) {
	p.calls++
	p.key = key
	p.request = r
	return p.result, p.payErr
}
func (p *partner) Poll(context.Context, string) (*contract.Result, error) { return p.result, nil }
func (p *partner) Finalize(_ context.Context, r contract.FinalizeRequest) (*contract.Result, error) {
	p.finalize = r
	return p.result, nil
}
func (p *partner) SubscriptionActive(context.Context, string, string) (bool, error) {
	return p.active, nil
}

type fixture struct {
	s       *app.Service
	store   *gormstore.Store
	orders  *orders
	partner *partner
	db      *gorm.DB
}

func setup(t *testing.T, c contract.Cards) *fixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sql.Close() })
	if err = db.AutoMigrate(&domain.Task{}); err != nil {
		t.Fatal(err)
	}
	f := &fixture{store: gormstore.New(db), db: db, orders: &orders{paid: true}, partner: &partner{result: &contract.Result{OperationID: "op_1", State: "queued"}}}
	f.s, err = app.New(app.Options{Store: f.store, Orders: f.orders, Cards: c, Partner: f.partner, Secret: strings.Repeat("k", 32), Bindings: []contract.Binding{{SKUID: 9, Plan: "chatgptplusplan", Region: "US", RegionVersion: 12, Channel: "3", CardRule: "plus-us", CancelAfterSuccess: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.CreateForOrder(1); err != nil {
		t.Fatal(err)
	}
	f.advance(t) // Paid order delivers its code without submitting a payment.
	return f
}
func (f *fixture) advance(t *testing.T) {
	t.Helper()
	if err := f.s.Advance(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) task(t *testing.T) *domain.Task {
	t.Helper()
	v, err := f.store.Get(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func (f *fixture) session(t *testing.T) {
	t.Helper()
	v, err := f.s.CheckAccount(context.Background(), f.orders.code, session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.ConfirmCode(context.Background(), f.orders.code, v.ConfirmationToken); err != nil {
		t.Fatal(err)
	}
}
func TestNoCardProviderNeverCharges(t *testing.T) {
	f := setup(t, nil)
	f.session(t)
	f.advance(t)
	if f.task(t).State != "waiting_card" || f.partner.calls != 0 || f.partner.checks != 0 {
		t.Fatal("missing card platform must pause before external operations")
	}
}
func TestPaidOrderToCompletedExactlyOnce(t *testing.T) {
	f := setup(t, cards{})
	f.session(t)
	before := f.task(t).IdempotencyKey
	if err := f.s.CreateForOrder(1); err != nil {
		t.Fatal(err)
	}
	f.advance(t)
	f.advance(t)
	if f.partner.calls != 1 || f.partner.key != before || f.task(t).State != "queued" {
		t.Fatal("duplicate submission")
	}
	if f.partner.request.Channel != "3" || !f.partner.request.CancelAfterSuccess || f.partner.request.Session != session {
		t.Fatal("request mapping")
	}
	yes := true
	f.partner.result = &contract.Result{OperationID: "op_1", State: "succeeded", SubscriptionSynced: &yes, BillingState: "charged", ChargedCredits: 1}
	f.advance(t)
	f.advance(t)
	v := f.task(t)
	if v.State != "completed" || f.orders.done != 1 || f.partner.calls != 1 || v.SessionCipher != "" {
		t.Fatal("completion not idempotent or retained credentials")
	}
}
func TestAmbiguousWriteDoesNotResubmitOrRotateCard(t *testing.T) {
	f := setup(t, cards{})
	f.session(t)
	f.partner.payErr = errors.New("connection dropped")
	f.advance(t)
	f.advance(t)
	if f.task(t).State != "unknown" || f.partner.calls != 1 {
		t.Fatal("uncertain payment retried")
	}
	if !errors.Is(f.s.SetSession(context.Background(), 1, 7, false, session), contract.ErrLocked) {
		t.Fatal("in-flight parameters mutable")
	}
}
func TestCrashAfterSubmittingCheckpointDoesNotReplay(t *testing.T) {
	f := setup(t, cards{})
	f.session(t)
	v, err := f.store.Claim(context.Background(), 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	v.State = "submitting"
	if err = f.store.Save(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	f.advance(t)
	if f.task(t).State != "unknown" || f.partner.calls != 0 {
		t.Fatal("replayed uncertain payment")
	}
}
func TestCredentialsAreEncryptedAndNeverReturned(t *testing.T) {
	f := setup(t, cards{})
	f.session(t)
	f.advance(t)
	v := f.task(t)
	if strings.Contains(v.SessionCipher, "test-access") || v.SessionCipher == "" {
		t.Fatal("session encryption")
	}
	raw, _ := json.Marshal(v)
	for _, secret := range []string{"test-access", "test-session", "4242424242424242", "card-reference", v.IdempotencyKey, v.SessionCipher} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("public task leaks secret")
		}
	}
	var stored map[string]interface{}
	if err := f.db.Table("auto_recharge_tasks").Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(stored)
	if strings.Contains(string(raw), "4242424242424242") || strings.Contains(string(raw), "test-access") {
		t.Fatal("raw credentials persisted")
	}
}
func TestOtherUserCannotReadOrSetSession(t *testing.T) {
	f := setup(t, cards{})
	if _, err := f.s.Get(context.Background(), 1, 8, false); !errors.Is(err, contract.ErrNotFound) {
		t.Fatal("ownership read")
	}
	if err := f.s.SetSession(context.Background(), 1, 8, false, session); !errors.Is(err, contract.ErrNotFound) {
		t.Fatal("ownership write")
	}
}
func TestFullSessionRequired(t *testing.T) {
	f := setup(t, cards{})
	if err := f.s.SetSession(context.Background(), 1, 7, false, `{"accessToken":"only"}`); !errors.Is(err, contract.ErrInvalid) {
		t.Fatal("accepted incomplete session")
	}
}
func TestUnpaidOrRefundedOrderCannotCharge(t *testing.T) {
	f := setup(t, cards{})
	f.session(t)
	f.orders.paid = false
	f.advance(t)
	if f.partner.calls != 0 || f.task(t).State != "manual_review" {
		t.Fatal("charged invalid local order")
	}
}

func TestRefundAfterSubmissionStillReconcilesButNeverDelivers(t *testing.T) {
	f := setup(t, cards{})
	f.session(t)
	f.advance(t)
	f.orders.paid = false
	f.advance(t)
	if f.task(t).State != "unknown" || f.partner.calls != 1 {
		t.Fatal("lost pending remote payment")
	}
	yes := true
	f.partner.result = &contract.Result{OperationID: "op_1", State: "succeeded", SubscriptionSynced: &yes}
	f.advance(t)
	if f.task(t).State != "manual_review" || f.task(t).RedeemedAt != nil || f.partner.calls != 1 {
		t.Fatal("delivered refunded order or charged again")
	}
}
func TestPaymentSuccessWaitsForSubscriptionWithoutRepay(t *testing.T) {
	f := setup(t, cards{})
	f.session(t)
	no := false
	f.partner.result = &contract.Result{OperationID: "op_1", State: "succeeded", SubscriptionSynced: &no}
	f.advance(t)
	f.advance(t)
	if f.task(t).RedeemedAt != nil || f.task(t).State != "confirming_subscription" {
		t.Fatal("delivered unconfirmed subscription")
	}
	f.partner.active = true
	f.advance(t)
	if f.task(t).RedeemedAt == nil || f.partner.calls != 1 {
		t.Fatal("subscription confirmation")
	}
}
func TestBankVerificationResumesOriginalTask(t *testing.T) {
	f := setup(t, cards{})
	f.session(t)
	f.partner.result = &contract.Result{OperationID: "op_1", State: "action_required", Plan: "chatgptplusplan", AccountID: "account_1", SetupIntentID: "seti_1", VerificationStage: "bind", ClientSecret: "seti_private", PublishableKey: "pk_test"}
	f.advance(t)
	if f.task(t).State != "action_required" || strings.Contains(f.task(t).VerificationCipher, "seti_private") {
		t.Fatal("verification state")
	}
	f.partner.result = &contract.Result{OperationID: "op_1", State: "running"}
	if err := f.s.Finalize(context.Background(), 1, false); err != nil {
		t.Fatal(err)
	}
	if f.partner.finalize.OperationID != "op_1" || f.partner.finalize.SetupIntentID != "seti_1" || f.partner.finalize.PaymentIntentID != "" || f.partner.calls != 1 {
		t.Fatal("bank validation used new task")
	}
}
func TestLeaseAllowsOnlyOneWorker(t *testing.T) {
	f := setup(t, cards{})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.store.Claim(context.Background(), 1, time.Now().UTC())
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	ok, busy := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, contract.ErrBusy) {
			busy++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || busy != 1 {
		t.Fatal("multiple workers claimed task")
	}
}
func TestUnknownProviderStateCannotComplete(t *testing.T) {
	f := setup(t, cards{})
	f.session(t)
	f.partner.result.State = "new_provider_state"
	f.advance(t)
	if f.task(t).State != "unknown" || f.task(t).RedeemedAt != nil {
		t.Fatal("unrecognized state delivered")
	}
}
