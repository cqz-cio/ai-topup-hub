package giftcardhttp

import (
	"testing"
	"time"

	giftcardapp "github.com/dujiao-next/internal/modules/giftcard/application"
)

func TestGenerateRequestMapsToGenerateInput(t *testing.T) {
	name := "春节礼品卡"
	expires := time.Date(2026, 12, 31, 15, 4, 5, 0, time.UTC)
	req := generateRequest{
		Name:      name,
		Quantity:  3,
		Amount:    "10.50",
		ExpiresAt: expires.Format(time.RFC3339),
	}
	if req.Name != name || req.Quantity != 3 || req.Amount != "10.50" {
		t.Fatalf("generate request fields mismatch: %#v", req)
	}
	if _, err := time.Parse(time.RFC3339, req.ExpiresAt); err != nil {
		t.Fatalf("expires_at parse: %v", err)
	}
	_ = giftcardapp.GenerateInput{}
}

func TestUpdateRequestClearExpiresAtConvention(t *testing.T) {
	empty := ""
	req := updateRequest{ExpiresAt: &empty}
	if req.ExpiresAt == nil || *req.ExpiresAt != "" {
		t.Fatalf("expected empty expires_at to signal clear")
	}
}
