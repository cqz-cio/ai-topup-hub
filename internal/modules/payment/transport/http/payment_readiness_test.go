package paymenthttp

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	pc "github.com/dujiao-next/internal/modules/procurement/contract"
	"github.com/gin-gonic/gin"
)

type blockedPendingLookup struct{ calls int }

func (p *blockedPendingLookup) GetLatestPendingByOrder(uint, time.Time) (*paymentdomain.Payment, error) {
	p.calls++
	return &paymentdomain.Payment{QRCode: "must-not-be-exposed"}, nil
}

type blockedReadiness struct {
	checks, notices int
	noticeID        uint
}

func (r *blockedReadiness) CheckPaymentBalance(context.Context, uint) error {
	r.checks++
	return &pc.PaymentBlock{Code: pc.BalanceInsufficient, NotificationStatus: "failed", NoticeID: 7}
}
func (r *blockedReadiness) ReadPaymentBalanceNotice(_ context.Context, _ uint, noticeID uint) string {
	r.notices++
	r.noticeID = noticeID
	return "sent"
}

func TestLatestPaymentChecksBeforeExposingCachedQRCode(t *testing.T) {
	pending := &blockedPendingLookup{}
	readiness := &blockedReadiness{}
	h := &LatestHandler{payments: pending, readiness: readiness}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/payments/latest", nil)
	h.respondLatestPayment(c, &orderdomain.Order{ID: 1, Status: constants.OrderStatusPendingPayment})
	var body struct {
		Status int `json:"status_code"`
		Data   struct {
			Block  *pc.PaymentBlock `json:"payment_block"`
			QRCode string           `json:"qr_code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status == 0 || body.Data.Block == nil || body.Data.Block.NotificationStatus != "failed" || body.Data.QRCode != "" || pending.calls != 0 || readiness.checks != 1 {
		t.Fatalf("cached payment leaked: %s", w.Body.String())
	}
}

func TestMailStatusPollingDoesNotRunPaymentOrSupplierChecks(t *testing.T) {
	pending := &blockedPendingLookup{}
	readiness := &blockedReadiness{}
	h := &LatestHandler{payments: pending, readiness: readiness}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/payments/balance-notice?notice_id=7", nil)
	h.respondBalanceNotice(c, &orderdomain.Order{ID: 1})
	if readiness.checks != 0 || readiness.notices != 1 || readiness.noticeID != 7 || pending.calls != 0 {
		t.Fatal("mail polling had payment side effects")
	}
}
