package upstreamreader

import (
	"context"
	"github.com/dujiao-next/internal/upstream"
	"testing"
)

type referenceAdapter struct {
	upstream.Adapter
	reference    string
	numericCalls int
}

func (a *referenceAdapter) GetOrderByReference(_ context.Context, ref string) (*upstream.UpstreamOrderDetail, error) {
	a.reference = ref
	return &upstream.UpstreamOrderDetail{Status: "delivered"}, nil
}
func (a *referenceAdapter) GetOrder(context.Context, uint) (*upstream.UpstreamOrderDetail, error) {
	a.numericCalls++
	return &upstream.UpstreamOrderDetail{Status: "completed", Amount: "10.00"}, nil
}

func TestStringSupplierOrderUsesReferenceAdapter(t *testing.T) {
	a := &referenceAdapter{}
	reader := &session{adapter: a}
	got, err := reader.Get(context.Background(), 0, "trade-string-123")
	if err != nil || got.Status != "delivered" || a.reference != "trade-string-123" || a.numericCalls != 0 {
		t.Fatalf("reference query failed: %+v %v", got, err)
	}
	got, err = reader.Get(context.Background(), 9, "")
	if err != nil || got.Amount != "10.00" || a.numericCalls != 1 {
		t.Fatalf("numeric fallback failed: %+v %v", got, err)
	}
}

type amountAdapter struct {
	referenceAdapter
	amountCalls int
}

func (a *amountAdapter) GetOrderWithAmountByReference(_ context.Context, ref string) (*upstream.UpstreamOrderDetail, error) {
	a.reference = ref
	a.amountCalls++
	return &upstream.UpstreamOrderDetail{OrderNo: ref, Status: "delivered", Amount: "107.50", Currency: "CNY"}, nil
}

func TestAmountReaderUsesFinancialEndpointOnlyWhenRequested(t *testing.T) {
	a := &amountAdapter{}
	reader := &session{adapter: a}
	got, err := reader.Get(context.Background(), 0, "00123")
	if err != nil || got.Amount != "" || a.amountCalls != 0 {
		t.Fatalf("status-only query used amount endpoint: %+v %v", got, err)
	}
	got, err = reader.GetWithAmount(context.Background(), 0, "00123")
	if err != nil || got.Amount != "107.50" || got.Currency != "CNY" || a.reference != "00123" || a.amountCalls != 1 {
		t.Fatalf("amount query did not use financial endpoint: %+v %v", got, err)
	}
	native := &session{adapter: &referenceAdapter{}}
	got, err = native.GetWithAmount(context.Background(), 9, "")
	if err != nil || got.Amount != "10.00" {
		t.Fatalf("native numeric query regressed: %+v %v", got, err)
	}
}
