package productdomain

import (
	"reflect"
	"testing"
)

func TestPaymentChannelIDsRoundTripAndDiscardInvalidValues(t *testing.T) {
	encoded := EncodePaymentChannelIDs([]uint{3, 7})
	if encoded != "[3,7]" {
		t.Fatalf("encoded IDs = %q", encoded)
	}
	if got := DecodePaymentChannelIDs(encoded); !reflect.DeepEqual(got, []uint{3, 7}) {
		t.Fatalf("decoded IDs = %#v", got)
	}
	if got := DecodePaymentChannelIDs(`[0,3,0,7]`); !reflect.DeepEqual(got, []uint{3, 7}) {
		t.Fatalf("filtered IDs = %#v", got)
	}
	for _, raw := range []string{"", "[]", "invalid", "[0]"} {
		if got := DecodePaymentChannelIDs(raw); got != nil {
			t.Fatalf("DecodePaymentChannelIDs(%q) = %#v, want nil", raw, got)
		}
	}
}
