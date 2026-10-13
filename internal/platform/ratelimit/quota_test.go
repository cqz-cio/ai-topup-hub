package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testReservations(t *testing.T, quota *Quota, prefix string) {
	t.Helper()
	ctx := context.Background()
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			retry, err := quota.Reserve(ctx, []Limit{{prefix + "recipient", 1, time.Minute}, {prefix + "global", 2, time.Minute}})
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			if retry == 0 {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("concurrent sends accepted=%d, want 1", accepted.Load())
	}
	retry, err := quota.Reserve(ctx, []Limit{{prefix + "other", 1, time.Minute}, {prefix + "global", 2, time.Minute}})
	if err != nil || retry != 0 {
		t.Fatalf("rejected requests consumed global quota: retry=%v err=%v", retry, err)
	}
	retry, err = quota.Reserve(ctx, []Limit{{prefix + "third", 1, time.Minute}, {prefix + "global", 2, time.Minute}})
	if err != nil || retry <= 0 {
		t.Fatalf("global budget bypass: retry=%v err=%v", retry, err)
	}
}

func TestLocalReservationIsAtomic(t *testing.T) { testReservations(t, New(nil, false), "test:") }

func TestRedisReservationIsAtomic(t *testing.T) {
	addr := os.Getenv("SECURITY_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set SECURITY_TEST_REDIS_ADDR to run against local Redis")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, Password: os.Getenv("SECURITY_TEST_REDIS_PASSWORD"), MaxRetries: -1, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second, ContextTimeoutEnabled: true})
	t.Cleanup(func() { _ = client.Close() })
	prefix := fmt.Sprintf("{security-test-%d}:", time.Now().UnixNano())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Del(ctx, prefix+"recipient", prefix+"global", prefix+"other", prefix+"third").Err()
	})
	testReservations(t, New(client, true), prefix)
}

func TestConfiguredRedisCannotFallBackToFreshBudget(t *testing.T) {
	if _, err := New(nil, true).Reserve(context.Background(), []Limit{{"key", 1, time.Minute}}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing Redis must reject: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
	defer client.Close()
	if _, err := New(client, true).Reserve(context.Background(), []Limit{{"key", 1, time.Minute}}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Redis outage must reject: %v", err)
	}
}

func TestLocalCapacityDoesNotEvictBudgets(t *testing.T) {
	q := New(nil, false)
	for i := 0; i < 10000; i++ {
		q.entries[fmt.Sprint(i)] = entry{count: 1, expires: time.Now().Add(time.Minute)}
	}
	if _, err := q.Reserve(context.Background(), []Limit{{"new", 1, time.Minute}}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("capacity must reject: %v", err)
	}
	if len(q.entries) != 10000 {
		t.Fatal("live entries changed")
	}
}
