package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client
var redisPrefix string
var redisEnabled bool

// ErrUnavailable indicates that a cache-backed security primitive cannot be
// used because Redis is disabled or was not initialized. Callers that require
// atomic, single-use state must fail closed on this error.
var ErrUnavailable = errors.New("redis cache unavailable")

// InitRedis 初始化 Redis 客户端
func InitRedis(cfg *config.RedisConfig) error {
	if cfg == nil || !cfg.Enabled {
		redisEnabled = false
		return nil
	}
	addr := strings.TrimSpace(cfg.Host)
	if addr == "" {
		addr = "127.0.0.1"
	}
	port := cfg.Port
	if port <= 0 {
		port = 6379
	}
	redisPrefix = strings.TrimSpace(cfg.Prefix)
	if redisPrefix == "" {
		redisPrefix = constants.RedisPrefixDefault
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", addr, port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	redisEnabled = true
	return nil
}

// Enabled 判断缓存是否启用
func Enabled() bool {
	return redisEnabled && redisClient != nil
}

// Client 获取 Redis 客户端
func Client() *redis.Client {
	if !Enabled() {
		return nil
	}
	return redisClient
}

// GetJSON 获取 JSON 缓存
func GetJSON(ctx context.Context, key string, dest interface{}) (bool, error) {
	if !Enabled() {
		return false, nil
	}
	val, err := redisClient.Get(ctx, buildKey(key)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return false, err
	}
	return true, nil
}

// SetJSON 写入 JSON 缓存
func SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if !Enabled() {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return redisClient.Set(ctx, buildKey(key), payload, ttl).Err()
}

// SetJSONRequired writes security-sensitive JSON state and fails closed when
// Redis is unavailable. It is intentionally separate from the best-effort
// cache helpers used by non-critical paths.
func SetJSONRequired(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if !Enabled() {
		return ErrUnavailable
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return redisClient.Set(ctx, buildKey(key), payload, ttl).Err()
}

// GetDelJSONRequired atomically reads and deletes a JSON value. Atomic GETDEL
// is required for one-time OAuth redirect intents and handoffs; a GET followed
// by DEL would permit concurrent replay.
func GetDelJSONRequired(ctx context.Context, key string, dest interface{}) (bool, error) {
	if !Enabled() {
		return false, ErrUnavailable
	}
	val, err := redisClient.GetDel(ctx, buildKey(key)).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return false, err
	}
	return true, nil
}

// GetString 获取字符串缓存
func GetString(ctx context.Context, key string) (string, error) {
	if !Enabled() {
		return "", nil
	}
	val, err := redisClient.Get(ctx, buildKey(key)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// SetString 写入字符串缓存
func SetString(ctx context.Context, key string, value string, ttl time.Duration) error {
	if !Enabled() {
		return nil
	}
	return redisClient.Set(ctx, buildKey(key), value, ttl).Err()
}

// Del 删除缓存
func Del(ctx context.Context, key string) error {
	if !Enabled() {
		return nil
	}
	return redisClient.Del(ctx, buildKey(key)).Err()
}

// DelPattern 删除匹配 pattern 的缓存键。pattern 会自动追加项目 Redis 前缀。
func DelPattern(ctx context.Context, pattern string) error {
	if !Enabled() {
		return nil
	}
	iter := redisClient.Scan(ctx, 0, buildKey(pattern), 100).Iterator()
	for iter.Next(ctx) {
		if err := redisClient.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// SetNX 原子写入（键不存在时写入并设置过期）
func SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if !Enabled() {
		return true, nil
	}
	_, err := redisClient.SetArgs(ctx, buildKey(key), value, redis.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func buildKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return redisPrefix
	}
	return fmt.Sprintf("%s:%s", redisPrefix, trimmed)
}
