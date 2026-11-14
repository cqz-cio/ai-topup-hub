// Package slug defines readable, bounded catalog identifiers without changing IDs.
package slug

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
)

var ErrInvalid = errors.New("slug must be 1-80 lowercase letters, digits or single hyphens")
var valid = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var separators = regexp.MustCompile(`[^a-z0-9]+`)

func Validate(value string) error {
	if len(value) < 1 || len(value) > 80 || !valid.MatchString(value) {
		return ErrInvalid
	}
	return nil
}

// Generate preserves ASCII brand names and transliterates Han characters.
func Generate(title string) string {
	var out strings.Builder
	args := pinyin.NewArgs()
	for _, r := range title {
		if unicode.Is(unicode.Han, r) {
			out.WriteByte('-')
			out.WriteString(strings.Join(pinyin.LazyPinyin(string(r), args), "-"))
			out.WriteByte('-')
		} else {
			out.WriteRune(unicode.ToLower(r))
		}
	}
	base := strings.Trim(separators.ReplaceAllString(out.String(), "-"), "-")
	if len(base) > 80 {
		base = strings.TrimRight(base[:80], "-")
	}
	if base == "" {
		return "item"
	}
	return base
}

// Unique never treats a failed occupancy query as an available name.
func Unique(base string, occupied func(string) (bool, error)) (string, error) {
	base = Generate(base)
	for n := 1; n <= 10000; n++ {
		candidate := base
		if n > 1 {
			suffix := "-" + strconv.Itoa(n)
			prefix := base
			if len(prefix)+len(suffix) > 80 {
				prefix = strings.TrimRight(prefix[:80-len(suffix)], "-")
			}
			candidate = prefix + suffix
		}
		used, err := occupied(candidate)
		if err != nil {
			return "", err
		}
		if !used {
			return candidate, nil
		}
	}
	return "", errors.New("no available catalog slug")
}
