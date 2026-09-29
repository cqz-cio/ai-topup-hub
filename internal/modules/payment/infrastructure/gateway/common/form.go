package common

import (
	"strings"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

func FormValue(form map[string][]string, key string) string {
	values := form[key]
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func FormToJSON(form map[string][]string) jsonmap.JSON {
	out := make(jsonmap.JSON, len(form))
	for key, values := range form {
		if len(values) > 0 {
			out[key] = values[0]
		}
	}
	return out
}
