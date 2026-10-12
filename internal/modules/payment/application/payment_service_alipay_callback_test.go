package application

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type alipayProcurementSpy struct{ orders []uint }

func (s *alipayProcurementSpy) CreateForOrder(orderID uint) error {
	s.orders = append(s.orders, orderID)
	return nil
}

// Notifications use ephemeral test keys, an in-memory database and a procurement spy.
func TestAlipayNotificationValidatesPaymentBeforeSchedulingProcurement(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, appID, sellerID, orderNo, amount, status string
		tamper, reject, paid                           bool
	}{
		{"paid", "2026000000000000", "2088000000000000", "DJP-NOTIFY-1", "1160.00", "TRADE_SUCCESS", false, false, true},
		{"finished", "2026000000000000", "2088000000000000", "DJP-NOTIFY-1", "1160.00", "TRADE_FINISHED", false, false, true},
		{"unpaid", "2026000000000000", "2088000000000000", "DJP-NOTIFY-1", "1160.00", "WAIT_BUYER_PAY", false, false, false},
		{"wrong app", "2026999999999999", "2088000000000000", "DJP-NOTIFY-1", "1160.00", "TRADE_SUCCESS", false, true, false},
		{"wrong seller", "2026000000000000", "2088999999999999", "DJP-NOTIFY-1", "1160.00", "TRADE_SUCCESS", false, true, false},
		{"missing seller", "2026000000000000", "", "DJP-NOTIFY-1", "1160.00", "TRADE_SUCCESS", false, true, false},
		{"wrong amount", "2026000000000000", "2088000000000000", "DJP-NOTIFY-1", "0.01", "TRADE_SUCCESS", false, true, false},
		{"wrong order", "2026000000000000", "2088000000000000", "DJP-NOTIFY-OTHER", "1160.00", "TRADE_SUCCESS", false, true, false},
		{"tampered", "2026000000000000", "2088000000000000", "DJP-NOTIFY-1", "1160.00", "TRADE_SUCCESS", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := setupPaymentServiceWalletTest(t)
			procurement := &alipayProcurementSpy{}
			svc.SetProcurementService(procurement)
			channel := &paymentdomain.PaymentChannel{
				Name:            "Alipay callback test",
				ProviderType:    constants.PaymentProviderOfficial,
				ChannelType:     constants.PaymentChannelTypeAlipay,
				InteractionMode: constants.PaymentInteractionPage,
				ConfigJSON: jsonmap.JSON{
					"app_id": "2026000000000000", "seller_id": "2088000000000000",
					"alipay_public_key": string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})),
					"sign_type":         "RSA2",
				},
			}
			if err := db.Create(channel).Error; err != nil {
				t.Fatal(err)
			}
			order := &orderdomain.Order{
				OrderNo: "SHOP-NOTIFY-1", Status: constants.OrderStatusPendingPayment,
				Currency: "CNY", TotalAmount: money.FromDecimal(decimal.NewFromInt(1160)),
			}
			if err := db.Create(order).Error; err != nil {
				t.Fatal(err)
			}
			payment := &paymentdomain.Payment{
				OrderID: order.ID, ChannelID: channel.ID,
				ProviderType: channel.ProviderType, ChannelType: channel.ChannelType,
				InteractionMode: channel.InteractionMode, Amount: order.TotalAmount,
				Currency: "CNY", Status: constants.PaymentStatusPending,
				GatewayOrderNo: "DJP-NOTIFY-1",
			}
			if err := db.Create(payment).Error; err != nil {
				t.Fatal(err)
			}
			form := url.Values{
				"app_id": {tc.appID}, "seller_id": {tc.sellerID}, "out_trade_no": {tc.orderNo},
				"trade_no": {"ALIPAY-NOTIFY-TRADE-1"}, "trade_status": {tc.status},
				"total_amount": {tc.amount}, "gmt_payment": {"2026-10-12 12:00:00"},
				"notify_type": {"trade_status_sync"}, "notify_id": {"NOTIFY-1"}, "sign_type": {"RSA2"},
			}
			keys := make([]string, 0, len(form))
			for name := range form {
				if name != "sign_type" && form.Get(name) != "" {
					keys = append(keys, name)
				}
			}
			sort.Strings(keys)
			parts := make([]string, 0, len(keys))
			for _, name := range keys {
				parts = append(parts, name+"="+form.Get(name))
			}
			digest := sha256.Sum256([]byte(strings.Join(parts, "&")))
			signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
			if err != nil {
				t.Fatal(err)
			}
			form.Set("sign", base64.StdEncoding.EncodeToString(signature))
			if tc.tamper {
				form.Set("total_amount", "0.02")
			}

			_, err = svc.HandleSyncCallback(channel, form, nil)
			if (err != nil) != tc.reject {
				t.Fatalf("callback rejected=%v, want %v; error=%v", err != nil, tc.reject, err)
			}
			if err := db.First(payment, payment.ID).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.First(order, order.ID).Error; err != nil {
				t.Fatal(err)
			}
			if !tc.paid {
				if payment.Status != constants.PaymentStatusPending || order.Status != constants.OrderStatusPendingPayment || order.PaidAt != nil || len(procurement.orders) != 0 {
					t.Fatal("invalid or unpaid notification changed payment or scheduled procurement")
				}
				return
			}
			if payment.Status != constants.PaymentStatusSuccess || order.Status != constants.OrderStatusPaid || order.PaidAt == nil {
				t.Fatal("verified notification did not settle order")
			}
			if len(procurement.orders) != 1 || procurement.orders[0] != order.ID {
				t.Fatal("confirmed paid order did not schedule procurement exactly once")
			}
			paidAt := *order.PaidAt
			for attempt := 0; attempt < 3; attempt++ {
				if _, err := svc.HandleSyncCallback(channel, form, nil); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.First(order, order.ID).Error; err != nil {
				t.Fatal(err)
			}
			if len(procurement.orders) != 1 || !order.PaidAt.Equal(paidAt) {
				t.Fatal("duplicate notifications repeated procurement or changed paid time")
			}
		})
	}
}
