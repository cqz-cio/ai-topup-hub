package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// UpstreamPurchaseWindow reads an optional, persistent resumption boundary.
// Both values must be present together. A malformed boundary must never widen
// purchasing permissions by falling back to the legacy unrestricted behavior.
func UpstreamPurchaseWindow() (uint, *time.Time, error) {
	id, hasID := os.LookupEnv("UPSTREAM_PURCHASE_ORDER_ID_FLOOR")
	stamp, hasTime := os.LookupEnv("UPSTREAM_PURCHASE_PAID_AFTER")
	if !hasID && !hasTime {
		return 0, nil, nil
	}
	if !hasID || !hasTime {
		return 0, nil, fmt.Errorf("both upstream purchase window fields are required")
	}
	floor, err := strconv.ParseUint(strings.TrimSpace(id), 10, strconv.IntSize)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid upstream purchase order boundary")
	}
	after, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(stamp))
	if err != nil || after.IsZero() {
		return 0, nil, fmt.Errorf("invalid upstream purchase payment boundary")
	}
	return uint(floor), &after, nil
}
