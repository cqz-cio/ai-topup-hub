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
