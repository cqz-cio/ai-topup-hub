package domain

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
)

func TestGenerateRedemptionCode(t *testing.T) {
	pattern := regexp.MustCompile(`^[A-HJ-NP-Z2-9]{5}(-[A-HJ-NP-Z2-9]{5}){4}$`)
	for i := 0; i < 64; i++ {
		code, err := GenerateRedemptionCode()
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(code) || !strings.ContainsAny(code, "23456789") || !strings.ContainsAny(code, "ABCDEFGHJKLMNPQRSTUVWXYZ") {
			t.Fatal("generated code violates format")
		}
		got, err := NormalizeRedemptionCode(code)
		if err != nil || got != code {
			t.Fatal("generated code does not round-trip")
		}
	}
}

func TestGenerateRedemptionCodeRejectsSingleCharacterTypeAndEntropyFailure(t *testing.T) {
	// All-letter and all-digit samples are rejected; the third sample is mixed.
	samples := append(bytes.Repeat([]byte{0}, 25), bytes.Repeat([]byte{24}, 25)...)
	mixed := bytes.Repeat([]byte{0}, 25)
	mixed[0] = 24
	samples = append(samples, mixed...)
	code, err := generateRedemptionCode(bytes.NewReader(samples))
	if err != nil || code != "2AAAA-AAAAA-AAAAA-AAAAA-AAAAA" {
		t.Fatal("single-type samples were not rejected", err)
	}
	code, err = generateRedemptionCode(bytes.NewReader(nil))
	if !errors.Is(err, io.EOF) || code != "" {
		t.Fatal("entropy failure must not produce a fallback code")
	}
}

func TestNormalizeRedemptionCode(t *testing.T) {
	const canonical = "K7M2P-R8W4X-6NQ9T-H3V5C-Y2D8F"
	for _, input := range []string{canonical, strings.ToLower(canonical), strings.ReplaceAll(canonical, "-", ""), " \t" + canonical + "\r\n"} {
		got, err := NormalizeRedemptionCode(input)
		if err != nil || got != canonical {
			t.Fatalf("valid input rejected: %q", input)
		}
	}
	for _, input := range []string{
		"", canonical[:28], canonical + "A",
		"K7M2-PR8W4X-6NQ9T-H3V5C-Y2D8F",
		strings.Replace(canonical, "-", "_", 1),
		strings.Replace(canonical, "-", "", 1),
		strings.Replace(canonical, "K", "I", 1),
		strings.Replace(canonical, "K", "O", 1),
		strings.Replace(canonical, "7", "0", 1),
		strings.Replace(canonical, "7", "1", 1),
		strings.Replace(canonical, "K", "Ｋ", 1),
		"AAAAA-AAAAA-AAAAA-AAAAA-AAAAA",
		"22222-22222-22222-22222-22222",
	} {
		if _, err := NormalizeRedemptionCode(input); !errors.Is(err, ErrInvalidRedemptionCode) {
			t.Fatalf("invalid input accepted: %q", input)
		}
	}
}
