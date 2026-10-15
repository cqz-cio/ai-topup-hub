package orderhttp

import (
	"encoding/json"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderpresenter "github.com/dujiao-next/internal/modules/order/transport/presenter"
	pc "github.com/dujiao-next/internal/modules/procurement/contract"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

type blockedOrderPayment struct{}

func (blockedOrderPayment) CreatePayment(CreatePaymentInput) (*CreatePaymentResult, error) {
	return nil, &pc.PaymentBlock{Code: pc.BalanceInsufficient, NotificationStatus: "pending", NoticeID: 7}
}

func TestCreateAndPayReturnsRecoverableOrderWithoutPaymentInstructions(t *testing.T) {
	h := &CreateHandler{pay: blockedOrderPayment{}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/orders/create-and-pay", nil)
	h.respondCreateAndPay(c, &orderdomain.Order{ID: 1, OrderNo: "MOCK-BLOCKED"}, orderpresenter.OrderDetail{}, 3, false)
	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data["order_no"] != "MOCK-BLOCKED" || body.Data["payment_block"] == nil {
		t.Fatalf("lost retry details: %s", w.Body.String())
	}
	for _, key := range []string{"payment_id", "qr_code", "pay_url", "wallet_address", "chain_amount"} {
		if _, exists := body.Data[key]; exists {
			t.Fatalf("exposed %s", key)
		}
	}
}
