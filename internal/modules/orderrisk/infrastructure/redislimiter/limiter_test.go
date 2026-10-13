package redislimiter

import (
	"context"
	"errors"
	"testing"
	"time"

	orderriskcontract "github.com/dujiao-next/internal/modules/orderrisk/contract"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	"github.com/redis/go-redis/v9"
)

func TestOrderLimiterFailsClosedWhenRedisMissingOrUnavailable(t *testing.T) {
	cfg := settingssecurity.OrderRateLimitConfig{WindowSeconds: 60, MaxRequests: 5, BlockSeconds: 60}
	if err := New().Check(orderriskcontract.CheckInput{UserID: 1}, cfg); !errors.Is(err, orderriskcontract.ErrProtectionUnavailable) {
		t.Fatalf("missing Redis: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := checkSingle(ctx, client, "security-test-unavailable", cfg); !errors.Is(err, orderriskcontract.ErrProtectionUnavailable) {
		t.Fatalf("Redis outage: %v", err)
	}
}
