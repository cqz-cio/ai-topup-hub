package slug

import (
	"errors"
	"strings"
	"testing"
)

func TestGenerateAndReserveNames(t *testing.T) {
	if got := Generate("ChatGPT Plus 充值"); got != "chatgpt-plus-chong-zhi" {
		t.Fatal(got)
	}
	if got := Generate("  Grok / Ultra___2026 "); got != "grok-ultra-2026" {
		t.Fatal(got)
	}
	for _, value := range []string{"", "-grok", "grok-", "grok--pro", "Grok", "中文", "a/b", strings.Repeat("a", 81)} {
		if Validate(value) == nil {
			t.Fatal("accepted invalid slug", value)
		}
	}
	value, err := Unique("Gemini", func(s string) (bool, error) { return s == "gemini" || s == "gemini-2", nil })
	if err != nil || value != "gemini-3" {
		t.Fatal(value, err)
	}
	value, err = Unique(strings.Repeat("x", 80), func(s string) (bool, error) { return len(s) == 80 && !strings.HasSuffix(s, "-2"), nil })
	if err != nil || len(value) != 80 || !strings.HasSuffix(value, "-2") {
		t.Fatal(value, err)
	}
	failure := errors.New("database unavailable")
	if _, err := Unique("grok", func(string) (bool, error) { return false, failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
