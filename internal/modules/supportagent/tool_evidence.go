package supportagent

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Keep general navigation available without duplicating tool-owned business
// facts in every prompt. The deterministic local rules still run first.
func modelPlanningSources(locale string) []Source {
	var result []Source
	for _, source := range baseSources(locale) {
		switch source.ID {
		case "guide:purchase", "guide:redeem", "guide:partners":
			result = append(result, source)
		}
	}
	return result
}

var evidenceChineseTimePattern = regexp.MustCompile(strings.TrimSuffix(strings.TrimPrefix(naturalDatePattern.String(), "^"), "$"))

// Check that the normalized tool time is actually present in a user message.
// Native dates must still contain a year and time; never infer either one.
func hasMomentEvidence(evidence string, wanted time.Time) bool {
	for _, pattern := range []*regexp.Regexp{evidenceTimePattern, evidenceChineseTimePattern} {
		for _, index := range pattern.FindAllStringIndex(evidence, -1) {
			if index[0] > 0 {
				previous, _ := utf8.DecodeLastRuneInString(evidence[:index[0]])
				if unicode.IsDigit(previous) || strings.ContainsRune(".+-", previous) {
					continue
				}
			}
			if index[1] < len(evidence) {
				next, _ := utf8.DecodeRuneInString(evidence[index[1]:])
				if unicode.IsDigit(next) || strings.ContainsRune(".:：+-秒分", next) || (next >= 'A' && next <= 'Z') || (next >= 'a' && next <= 'z') {
					continue
				}
			}
			actual, err := parseMoment(evidence[index[0]:index[1]])
			if err == nil && actual.Equal(wanted) {
				return true
			}
		}
	}
	return false
}
