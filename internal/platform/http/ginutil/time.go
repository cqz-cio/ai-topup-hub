package ginutil

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ParseTimeNullable 解析可空的 RFC3339 时间字符串，空串返回 nil。
func ParseTimeNullable(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// ParseQueryTimeRange 解析查询参数中的 RFC3339 时间范围，空值返回 nil。
func ParseQueryTimeRange(c *gin.Context, fromKey, toKey string) (*time.Time, *time.Time, error) {
	from, err := ParseTimeNullable(strings.TrimSpace(c.Query(fromKey)))
	if err != nil {
		return nil, nil, err
	}
	to, err := ParseTimeNullable(strings.TrimSpace(c.Query(toKey)))
	if err != nil {
		return nil, nil, err
	}
	return from, to, nil
}
