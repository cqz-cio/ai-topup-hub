package domain

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestStatusConsistencyRefundWindows(t *testing.T) {
	for _, test := range []struct {
		name, local, upstream string
		want                  bool
	}{
		{"canceled refunded", constants.ProcurementStatusCanceled, "refunded", true},
		{"canceled partial refund", constants.ProcurementStatusCanceled, "partially_refunded", true},
		{"canceled normalization", constants.ProcurementStatusCanceled, "  CaNcElLeD  ", true},
		{"fulfilled partial refund", constants.ProcurementStatusFulfilled, "partially_refunded", true},
		{"completed refunded", constants.ProcurementStatusCompleted, "refunded", true},
		{"accepted partial refund", constants.ProcurementStatusAccepted, "partially_refunded", false},
		{"submitted refunded", constants.ProcurementStatusSubmitted, "refunded", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := IsStatusConsistent(test.local, test.upstream); got != test.want {
				t.Fatalf("isStatusConsistent(%q, %q)=%v want %v", test.local, test.upstream, got, test.want)
			}
		})
	}
}
