package contract

import "errors"

var (
	ErrIPBlacklisted               = errors.New("risk: ip blacklisted")
	ErrClientIPUnavailable         = errors.New("risk: client ip unavailable")
	ErrTooManyPendingOrders        = errors.New("risk: too many pending orders")
	ErrProductQuantityLimit        = errors.New("risk: product quantity limit")
	ErrPendingProductQuantityLimit = errors.New("risk: pending product quantity limit")
	ErrOrderRateLimited            = errors.New("risk: order rate limited")
)

// RateLimitedError 携带 Retry-After 秒数。
type RateLimitedError struct {
	RetryAfter int64
}

func (e *RateLimitedError) Error() string {
	return ErrOrderRateLimited.Error()
}

func (e *RateLimitedError) Is(target error) bool {
	return target == ErrOrderRateLimited
}

// GetRetryAfter 从限流错误中提取 Retry-After 秒数。
func GetRetryAfter(err error) int64 {
	var limited *RateLimitedError
	if errors.As(err, &limited) {
		return limited.RetryAfter
	}
	return 0
}
