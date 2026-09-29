package promotionhttp

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestBuildCreatePromotionInputFromRequestMapsFieldsAndParsesTimes(t *testing.T) {
	isActive := false
	req := CreatePromotionRequest{
		Name:       "Summer sale",
		Type:       "fixed",
		ScopeRefID: 42,
		Value:      8.765,
		MinAmount:  99.994,
		StartsAt:   "2026-06-01T10:00:00Z",
		EndsAt:     "2026-06-02T10:00:00Z",
		IsActive:   &isActive,
	}

	input, err := buildCreatePromotionInputFromRequest(req)
	if err != nil {
		t.Fatalf("buildCreatePromotionInputFromRequest: %v", err)
	}
	if input.Name != req.Name || input.Type != req.Type || input.ScopeRefID != req.ScopeRefID {
		t.Fatalf("basic fields mismatch: %#v", input)
	}
	if !input.Value.Decimal.Equal(decimal.RequireFromString("8.77")) {
		t.Fatalf("value mismatch: %s", input.Value.String())
	}
	if !input.MinAmount.Decimal.Equal(decimal.RequireFromString("99.99")) {
		t.Fatalf("min_amount mismatch: %s", input.MinAmount.String())
	}
	if input.StartsAt == nil || input.StartsAt.Format(time.RFC3339) != req.StartsAt {
		t.Fatalf("starts_at mismatch: %#v", input.StartsAt)
	}
	if input.EndsAt == nil || input.EndsAt.Format(time.RFC3339) != req.EndsAt {
		t.Fatalf("ends_at mismatch: %#v", input.EndsAt)
	}
	if input.IsActive == nil || *input.IsActive != isActive {
		t.Fatalf("is_active mismatch: %#v", input.IsActive)
	}
}

func TestBuildUpdatePromotionInputFromRequestMapsFieldsAndParsesTimes(t *testing.T) {
	isActive := true
	req := CreatePromotionRequest{
		Name:       "Member deal",
		Type:       "special_price",
		ScopeRefID: 7,
		Value:      39.9,
		MinAmount:  0,
		StartsAt:   "2026-07-01T10:00:00Z",
		EndsAt:     "2026-07-02T10:00:00Z",
		IsActive:   &isActive,
	}

	input, err := buildUpdatePromotionInputFromRequest(req)
	if err != nil {
		t.Fatalf("buildUpdatePromotionInputFromRequest: %v", err)
	}
	if input.Name != req.Name || input.Type != req.Type || input.ScopeRefID != req.ScopeRefID {
		t.Fatalf("basic fields mismatch: %#v", input)
	}
	if !input.Value.Decimal.Equal(decimal.RequireFromString("39.90")) {
		t.Fatalf("value mismatch: %s", input.Value.String())
	}
	if input.StartsAt == nil || input.StartsAt.Format(time.RFC3339) != req.StartsAt {
		t.Fatalf("starts_at mismatch: %#v", input.StartsAt)
	}
	if input.EndsAt == nil || input.EndsAt.Format(time.RFC3339) != req.EndsAt {
		t.Fatalf("ends_at mismatch: %#v", input.EndsAt)
	}
	if input.IsActive == nil || *input.IsActive != isActive {
		t.Fatalf("is_active mismatch: %#v", input.IsActive)
	}
}

func TestBuildCreatePromotionInputFromRequestRejectsInvalidTime(t *testing.T) {
	_, err := buildCreatePromotionInputFromRequest(CreatePromotionRequest{
		Name:     "Summer sale",
		Type:     "fixed",
		Value:    10,
		StartsAt: "not-a-time",
	})
	if err == nil {
		t.Fatalf("expected invalid time error")
	}
}
