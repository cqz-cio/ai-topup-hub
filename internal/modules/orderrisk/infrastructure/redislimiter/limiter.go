package redislimiter

import (
	"context"
	"fmt"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/logger"
	orderriskcontract "github.com/dujiao-next/internal/modules/orderrisk/contract"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"

	"github.com/redis/go-redis/v9"
)

var orderRateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
	redis.call("EXPIRE", KEYS[1], ARGV[1])
end
if tonumber(ARGV[2]) > 0 and tonumber(ARGV[3]) > 0 and current == tonumber(ARGV[2]) + 1 then
	redis.call("EXPIRE", KEYS[1], ARGV[3])
end
local ttl = redis.call("TTL", KEYS[1])
return {current, ttl}
`)

// Limiter 使用 Redis 固定窗口计数实现下单频率限制。
type Limiter struct{}

var _ orderriskcontract.RateLimiter = (*Limiter)(nil)

func New() *Limiter { return &Limiter{} }

func (l *Limiter) Check(input orderriskcontract.CheckInput, config settingssecurity.OrderRateLimitConfig) error {
	client := cache.Client()
	if client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if input.IsGuest && !input.SkipIPCheck && input.RiskIP != "" {
		if err := checkSingle(ctx, client, fmt.Sprintf("dj:risk:order_rate:guest_ip:%s", input.RiskIP), config); err != nil {
			return err
		}
	} else if !input.IsGuest && input.UserID > 0 {
		if err := checkSingle(ctx, client, fmt.Sprintf("dj:risk:order_rate:user:%d", input.UserID), config); err != nil {
			return err
		}
	}
	return nil
}

func checkSingle(ctx context.Context, client *redis.Client, key string, config settingssecurity.OrderRateLimitConfig) error {
	result, err := orderRateLimitScript.Run(ctx, client, []string{key},
		config.WindowSeconds, config.MaxRequests, config.BlockSeconds,
	).Result()
	if err != nil {
		logger.Warnw("risk_control_rate_limit_script_error", "key", key, "error", err)
		return nil
	}
	values, ok := result.([]interface{})
	if !ok || len(values) < 2 {
		return nil
	}
	current, _ := values[0].(int64)
	ttl, _ := values[1].(int64)
	if current <= int64(config.MaxRequests) {
		return nil
	}
	if ttl < 0 {
		ttl = 0
	}
	return &orderriskcontract.RateLimitedError{RetryAfter: ttl}
}
