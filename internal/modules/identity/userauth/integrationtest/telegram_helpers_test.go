package integrationtest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func buildUserAuthTestTelegramMiniAppInitData(t *testing.T, botToken string, authDate int64, userJSON string) string {
	t.Helper()
	values := url.Values{}
	values.Set("auth_date", strconv.FormatInt(authDate, 10))
	values.Set("query_id", "AAE-test-query")
	values.Set("user", userJSON)

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	dataCheckString := strings.Join(parts, "\n")

	secretMac := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMac.Write([]byte(botToken))
	secret := secretMac.Sum(nil)
	hashMac := hmac.New(sha256.New, secret)
	_, _ = hashMac.Write([]byte(dataCheckString))
	values.Set("hash", hex.EncodeToString(hashMac.Sum(nil)))
	return values.Encode()
}
