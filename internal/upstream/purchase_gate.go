package upstream

import (
	"os"
	"strings"
)

// A deployed shop can refresh inventory while balance purchases are disabled.
// Unset preserves existing installations; an explicit value only enables
// purchases when it is "true", so a misspelled deployment value fails closed.
func purchasesEnabled() bool {
	value, configured := os.LookupEnv("UPSTREAM_PURCHASE_ENABLED")
	return !configured || strings.EqualFold(strings.TrimSpace(value), "true")
}

func purchaseDisabledResponse() *CreateUpstreamOrderResp {
	return &CreateUpstreamOrderResp{
		ErrorCode:    "purchase_disabled",
		ErrorMessage: "upstream purchases are disabled for this deployment",
	}
}
