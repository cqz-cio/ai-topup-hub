package upstream

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"time"
)

const (
	// HeaderApiKey API Key header
	HeaderApiKey = "Dujiao-Next-Api-Key"
	// HeaderTimestamp 时间戳 header
	HeaderTimestamp = "Dujiao-Next-Timestamp"
	// HeaderSignature 签名 header
	HeaderSignature = "Dujiao-Next-Signature"

	// MaxTimestampSkew 最大时间戳偏差（秒）
	MaxTimestampSkew = 60
)

// Sign 生成 HMAC-SHA256 签名
// signString = "{method}\n{path}\n{timestamp}\n{body_md5}"
func Sign(secret, method, path string, timestamp int64, body []byte) string {
	bodyMD5 := md5Hex(body)
	signString := fmt.Sprintf("%s\n%s\n%d\n%s", method, path, timestamp, bodyMD5)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signString))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify 验证签名
func Verify(secret, method, path, signature string, timestamp int64, body []byte) bool {
	expected := Sign(secret, method, path, timestamp, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// IsTimestampValid 检查时间戳是否在有效范围内
func IsTimestampValid(timestamp int64) bool {
	now := time.Now().Unix()
	return math.Abs(float64(now-timestamp)) <= MaxTimestampSkew
}

// ParseTimestamp 解析时间戳字符串
func ParseTimestamp(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func md5Hex(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
