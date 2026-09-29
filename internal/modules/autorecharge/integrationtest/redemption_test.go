package integrationtest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"github.com/dujiao-next/internal/modules/autorecharge/domain"
	rechargehttp "github.com/dujiao-next/internal/modules/autorecharge/transport/http"
	"github.com/gin-gonic/gin"
)

func TestTwoCodesCannotChargeSameAccountConcurrently(t *testing.T) {
	f := setup(t, cards{})
	second := *f.task(t)
	second.ID, second.OrderID = 0, 2
	second.IdempotencyKey = "second-test-task"
	code, err := domain.GenerateRedemptionCode()
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte(code))
	hash := hex.EncodeToString(h[:])
	second.CodeHash = &hash
	if err = f.db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	v1, err := f.s.CheckAccount(context.Background(), f.orders.code, session)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := f.s.CheckAccount(context.Background(), code, session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ConfirmCode(context.Background(), f.orders.code, v1.ConfirmationToken); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ConfirmCode(context.Background(), code, v2.ConfirmationToken); err == nil {
		t.Fatal("two codes reserved the same account")
	}
	stored, err := f.store.Get(context.Background(), 2)
	if err != nil || stored.ConfirmedAt != nil {
		t.Fatal("failed account reservation committed confirmation")
	}
	f.advance(t)
	if f.partner.calls != 1 {
		t.Fatal("unexpected number of payments")
	}
}

func TestPaymentOnlyDeliversCodeAndPrecheckNeverCharges(t *testing.T) {
	f := setup(t, cards{})
	if f.orders.code == "" || f.orders.done != 1 || f.task(t).State != "issued" {
		t.Fatal("code not delivered")
	}
	for i := 0; i < 3; i++ {
		f.advance(t)
	}
	if f.partner.calls != 0 || f.task(t).ConfirmedAt != nil {
		t.Fatal("payment without consent")
	}
	v, err := f.s.CheckAccount(context.Background(), f.orders.code, session)
	if err != nil || v.Account == nil || v.Account.ID != "account_1" || v.ConfirmationToken == "" {
		t.Fatalf("account precheck: %+v %v", v, err)
	}
	f.advance(t)
	if f.partner.calls != 0 || f.task(t).State != "awaiting_confirmation" {
		t.Fatal("precheck submitted payment")
	}
	for i := 0; i < 2; i++ {
		if _, err = f.s.ConfirmCode(context.Background(), f.orders.code, v.ConfirmationToken); err != nil {
			t.Fatal(err)
		}
	}
	f.advance(t)
	if f.partner.calls != 1 || f.orders.done != 1 {
		t.Fatal("duplicate payment or code delivery")
	}
}

func TestChangingSessionInvalidatesPreviousConfirmation(t *testing.T) {
	f := setup(t, cards{})
	v, err := f.s.CheckAccount(context.Background(), f.orders.code, session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.CheckAccount(context.Background(), f.orders.code, `{"accessToken":"missing-session-token"}`); !errors.Is(err, contract.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = f.s.ConfirmCode(context.Background(), f.orders.code, v.ConfirmationToken); !errors.Is(err, contract.ErrInvalid) {
		t.Fatal("stale confirmation accepted")
	}
	f.advance(t)
	if f.partner.calls != 0 || f.task(t).SessionCipher != "" {
		t.Fatal("old session retained")
	}
}

func TestExpiredPrecheckMustBeRepeated(t *testing.T) {
	f := setup(t, cards{})
	v, err := f.s.CheckAccount(context.Background(), f.orders.code, session)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Minute)
	if err = f.db.Model(f.task(t)).Update("checked_until", past).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ConfirmCode(context.Background(), f.orders.code, v.ConfirmationToken); !errors.Is(err, contract.ErrInvalid) {
		t.Fatal("expired proof accepted")
	}
	f.advance(t)
	if f.task(t).State != "issued" || f.task(t).AccountCipher != "" || f.task(t).SessionCipher != "" {
		t.Fatal("expired credentials not cleared")
	}
}

func TestSubscribedOrUnverifiedAccountCannotConfirm(t *testing.T) {
	f := setup(t, cards{})
	f.partner.account = &contract.Account{ID: "account_1", Plan: "plus"}
	v, err := f.s.CheckAccount(context.Background(), f.orders.code, session)
	if err != nil || v.ConfirmationToken != "" || v.Reason != "existing_subscription_not_supported" {
		t.Fatal("subscribed account accepted")
	}
	f.partner.inspectErr = errors.New("upstream unavailable")
	if _, err = f.s.CheckAccount(context.Background(), f.orders.code, session); !errors.Is(err, contract.ErrUnavailable) {
		t.Fatal("unverified account accepted")
	}
	f.advance(t)
	if f.partner.calls != 0 {
		t.Fatal("ineligible account charged")
	}
}

func TestAccountIsRecheckedBeforeCardPayment(t *testing.T) {
	f := setup(t, cards{})
	f.session(t)
	f.partner.account = &contract.Account{ID: "different-account", Plan: "free"}
	f.advance(t)
	if f.partner.calls != 0 || f.task(t).State != "manual_review" {
		t.Fatal("changed account was charged")
	}
}

func TestConcurrentConfirmationPinsOneTask(t *testing.T) {
	f := setup(t, cards{})
	v, err := f.s.CheckAccount(context.Background(), f.orders.code, session)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := f.s.ConfirmCode(context.Background(), f.orders.code, v.ConfirmationToken)
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil && !errors.Is(e, contract.ErrBusy) {
			t.Fatal(e)
		}
	}
	f.advance(t)
	f.advance(t)
	if f.partner.calls != 1 || f.task(t).ConfirmedAt == nil {
		t.Fatal("concurrent confirm duplicated submission")
	}
}

func TestPublicRedemptionAPIConsentAndSecretBoundaries(t *testing.T) {
	f := setup(t, nil)
	r := gin.New()
	rechargehttp.RegisterRedemptionRoutes(r.Group("/redeem"), rechargehttp.New(f.s))
	post := func(path string, body map[string]interface{}) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(raw))))
		return w
	}
	if w := post("/redeem/lookup", map[string]interface{}{"code": "invalid"}); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := post("/redeem/account", map[string]interface{}{"code": f.orders.code, "session": session}); w.Code != 400 {
		t.Fatal("missing authorization accepted")
	}
	w := post("/redeem/account", map[string]interface{}{"code": f.orders.code, "session": session, "authorized": true})
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("account lookup failed", w.Code)
	}
	var reply struct {
		Data contract.RedemptionView `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	if w := post("/redeem/confirm", map[string]interface{}{"code": f.orders.code, "confirmation_token": reply.Data.ConfirmationToken}); w.Code != 400 {
		t.Fatal("implicit confirmation accepted")
	}
	w = post("/redeem/confirm", map[string]interface{}{"code": f.orders.code, "confirmation_token": reply.Data.ConfirmationToken, "confirmed": true})
	if w.Code != 200 {
		t.Fatal("confirmation failed", w.Code)
	}
	f.advance(t)
	w = post("/redeem/lookup", map[string]interface{}{"code": strings.ToLower(strings.ReplaceAll(f.orders.code, "-", ""))})
	if w.Code != 200 || !strings.Contains(w.Body.String(), "waiting_card") {
		t.Fatal("lookup normalization or state failed")
	}
	for _, secret := range []string{f.orders.code, "test-access", reply.Data.ConfirmationToken, "order_id", "code_hash", "account_1"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("status leaked credentials or identity")
		}
	}
	f.orders.paid = false
	if w := post("/redeem/lookup", map[string]interface{}{"code": f.orders.code}); w.Code != 404 {
		t.Fatal("refunded code accepted")
	}
}
