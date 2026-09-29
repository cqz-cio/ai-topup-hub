package contract

import "testing"

func TestRateLimitedErrorContract(t *testing.T) {
	err := &RateLimitedError{RetryAfter: 60}
	if err.Error() != ErrOrderRateLimited.Error() {
		t.Fatalf("expected error message match")
	}
	if !err.Is(ErrOrderRateLimited) {
		t.Fatal("expected rate-limited error identity")
	}
	if retryAfter := GetRetryAfter(err); retryAfter != 60 {
		t.Fatalf("expected retry-after 60, got %d", retryAfter)
	}
	if retryAfter := GetRetryAfter(ErrIPBlacklisted); retryAfter != 0 {
		t.Fatalf("expected zero retry-after for unrelated error, got %d", retryAfter)
	}
}

func TestNormalizeRiskIP(t *testing.T) {
	tests := map[string]string{
		"1.2.3.4":                  "1.2.3.4",
		"::ffff:1.2.3.4":           "1.2.3.4",
		"2001:db8:1234:5678::1":    "2001:db8:1234:5678::/64",
		"2001:db8:1234:5678::abcd": "2001:db8:1234:5678::/64",
		"not-an-ip":                "",
		"":                         "",
	}
	for input, expected := range tests {
		if actual := NormalizeRiskIP(input); actual != expected {
			t.Fatalf("NormalizeRiskIP(%q)=%q, want %q", input, actual, expected)
		}
	}
}
