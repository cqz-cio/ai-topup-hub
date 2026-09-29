package application

import (
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	codePrefix  = "GC"
	batchPrefix = "GCB"
)

func normalizeIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return []uint{}
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func normalizeExpireAt(raw *time.Time) *time.Time {
	if raw == nil || raw.IsZero() {
		return nil
	}
	value := raw.UTC()
	return &value
}

func formatNullableTime(raw *time.Time) string {
	if raw == nil || raw.IsZero() {
		return ""
	}
	return raw.Format(time.RFC3339)
}

func generateBatchNo(now time.Time) string {
	return strings.ToUpper(fmt.Sprintf("%s%s%s", batchPrefix, now.Format("20060102150405"), randomHex(4)))
}

func generateCode(now time.Time, index int) string {
	return strings.ToUpper(fmt.Sprintf("%s%s%04d%s", codePrefix, now.Format("060102150405"), index%10000, randomHex(5)))
}

func randomHex(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	if _, err := crand.Read(buf); err != nil {
		fallback := make([]byte, n)
		for i := range fallback {
			fallback[i] = byte('A' + (i % 26))
		}
		return hex.EncodeToString(fallback)
	}
	return hex.EncodeToString(buf)
}
