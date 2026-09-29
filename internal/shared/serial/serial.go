package serial

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Generate 生成“前缀 + 秒级时间戳 + 六位随机数字”的业务流水号。
func Generate(prefix string) string {
	return fmt.Sprintf("%s%s%s", strings.TrimSpace(prefix), time.Now().Format("20060102150405"), randomNumeric(6))
}

func randomNumeric(length int) string {
	if length <= 0 {
		return ""
	}
	var result strings.Builder
	for i := 0; i < length; i++ {
		value, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			result.WriteByte('0')
			continue
		}
		fmt.Fprintf(&result, "%d", value.Int64())
	}
	return result.String()
}
