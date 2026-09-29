package presenter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestProfileOmitsSensitiveFields(t *testing.T) {
	profile := &affiliatedomain.Profile{
		ID:            1,
		UserID:        99,
		AffiliateCode: "AFF-001",
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	resp := NewProfile(profile)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	if strings.Contains(jsonStr, `"user_id"`) {
		t.Error("user_id should not appear")
	}
	if strings.Contains(jsonStr, `"updated_at"`) {
		t.Error("updated_at should not appear")
	}
	if resp.AffiliateCode != "AFF-001" {
		t.Errorf("expected code AFF-001, got %s", resp.AffiliateCode)
	}
}

func TestCommissionOmitsSensitiveFields(t *testing.T) {
	commission := &affiliatedomain.Commission{
		ID:                 1,
		AffiliateProfileID: 5,
		OrderID:            10,
		CommissionType:     "order",
		BaseAmount:         newMoney("100.00"),
		RatePercent:        newMoney("10.00"),
		CommissionAmount:   newMoney("10.00"),
		Status:             "available",
		CreatedAt:          time.Now(),
	}

	resp := NewCommission(commission)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	sensitiveFields := []string{"affiliate_profile_id", "base_amount", "rate_percent", "invalid_reason"}
	for _, field := range sensitiveFields {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("sensitive field %q should not appear", field)
		}
	}

	if !strings.Contains(jsonStr, `"commission_amount"`) {
		t.Error("commission_amount should appear")
	}
}

func TestWithdrawOmitsSensitiveFields(t *testing.T) {
	withdraw := &affiliatedomain.WithdrawRequest{
		ID:                 1,
		AffiliateProfileID: 5,
		Amount:             newMoney("50.00"),
		Channel:            "alipay",
		Account:            "user@alipay.com",
		Status:             "pending",
		ProcessedBy:        ptrUint(3),
		CreatedAt:          time.Now(),
	}

	resp := NewWithdraw(withdraw)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	if strings.Contains(jsonStr, `"affiliate_profile_id"`) {
		t.Error("affiliate_profile_id should not appear")
	}
	if strings.Contains(jsonStr, `"processed_by"`) {
		t.Error("processed_by should not appear")
	}
	if !strings.Contains(jsonStr, `"channel"`) {
		t.Error("channel should appear")
	}
}

func ptrUint(v uint) *uint {
	return &v
}

func newMoney(raw string) money.Amount {
	return money.FromDecimal(decimal.RequireFromString(raw))
}
