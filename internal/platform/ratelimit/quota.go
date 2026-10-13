// Package ratelimit provides atomic reservations for costly operations.
package ratelimit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrUnavailable = errors.New("quota store unavailable")

type Limit struct {
	Key    string
	Max    int
	Window time.Duration
}

type Reserver interface {
	Reserve(context.Context, []Limit) (time.Duration, error)
}

// All keys must share a Redis cluster hash tag. Rejected reservations consume
// nothing; accepted reservations remain charged even if the external send fails.
var reserveScript = redis.NewScript(`
local retry = 0
for i, key in ipairs(KEYS) do
  local count = tonumber(redis.call('GET', key) or '0')
  if count >= tonumber(ARGV[i * 2 - 1]) then
    local ttl = redis.call('PTTL', key)
    if ttl < 1 then ttl = tonumber(ARGV[i * 2]) end
    retry = math.max(retry, ttl)
  end
end
if retry > 0 then return retry end
for i, key in ipairs(KEYS) do
  if redis.call('INCR', key) == 1 then
    redis.call('PEXPIRE', key, ARGV[i * 2])
  end
end
return 0
`)

type entry struct {
	count   int
	expires time.Time
}

type Quota struct {
	client       *redis.Client
	requireRedis bool
	mu           sync.Mutex
	entries      map[string]entry
}

func New(client *redis.Client, requireRedis bool) *Quota {
	return &Quota{client: client, requireRedis: requireRedis, entries: make(map[string]entry)}
}

func (q *Quota) Reserve(ctx context.Context, limits []Limit) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, ErrUnavailable
	}
	seen := make(map[string]bool, len(limits))
	for _, limit := range limits {
		if limit.Key == "" || limit.Max < 1 || limit.Window < time.Millisecond || seen[limit.Key] {
			return 0, ErrUnavailable
		}
		seen[limit.Key] = true
	}
	if len(limits) == 0 {
		return 0, ErrUnavailable
	}
	if q.client != nil {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		keys := make([]string, 0, len(limits))
		args := make([]interface{}, 0, 2*len(limits))
		for _, limit := range limits {
			keys = append(keys, limit.Key)
			args = append(args, limit.Max, limit.Window.Milliseconds())
		}
		retry, err := reserveScript.Run(ctx, q.client, keys, args...).Int64()
		if err != nil || retry < 0 {
			return 0, ErrUnavailable
		}
		return time.Duration(retry) * time.Millisecond, nil
	}
	// Explicitly disabled Redis supports a bounded, single-process local mode.
	// A configured Redis outage must never reset mail budgets to an empty map.
	if q.requireRedis {
		return 0, ErrUnavailable
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	var retry time.Duration
	for _, limit := range limits {
		v := q.entries[limit.Key]
		if now.Before(v.expires) && v.count >= limit.Max && v.expires.Sub(now) > retry {
			retry = v.expires.Sub(now)
		}
	}
	if retry > 0 {
		return retry, nil
	}
	if len(q.entries)+len(limits) > 10000 {
		for key, v := range q.entries {
			if !now.Before(v.expires) {
				delete(q.entries, key)
			}
		}
		missing := 0
		for _, limit := range limits {
			if _, ok := q.entries[limit.Key]; !ok {
				missing++
			}
		}
		if len(q.entries)+missing > 10000 {
			return 0, ErrUnavailable
		}
	}
	for _, limit := range limits {
		v := q.entries[limit.Key]
		if !now.Before(v.expires) {
			v = entry{expires: now.Add(limit.Window)}
		}
		v.count++
		q.entries[limit.Key] = v
	}
	return 0, nil
}
