package cache

import (
	"context"
	"fmt"
	"time"
)

const (
	resellerDomainCachePrefix      = "reseller:domain"
	resellerDomainNotFoundPrefix   = "reseller:domain:not_found"
	ResellerDomainCacheTTL         = 5 * time.Minute
	ResellerDomainNegativeCacheTTL = 60 * time.Second
)

type ResellerDomainCacheValue struct {
	ResellerID         uint   `json:"reseller_id"`
	ResellerUserID     uint   `json:"reseller_user_id"`
	Domain             string `json:"domain"`
	PrimaryDomain      string `json:"primary_domain"`
	Status             string `json:"status"`
	VerificationStatus string `json:"verification_status"`
}

func ResellerDomainCacheKey(host string) string {
	return fmt.Sprintf("%s:%s", resellerDomainCachePrefix, host)
}

func ResellerDomainNotFoundCacheKey(host string) string {
	return fmt.Sprintf("%s:%s", resellerDomainNotFoundPrefix, host)
}

func GetResellerDomain(ctx context.Context, host string, dest *ResellerDomainCacheValue) (bool, error) {
	return GetJSON(ctx, ResellerDomainCacheKey(host), dest)
}

func SetResellerDomain(ctx context.Context, host string, value ResellerDomainCacheValue) error {
	return SetJSON(ctx, ResellerDomainCacheKey(host), value, ResellerDomainCacheTTL)
}

func GetResellerDomainNotFound(ctx context.Context, host string) (bool, error) {
	val, err := GetString(ctx, ResellerDomainNotFoundCacheKey(host))
	return val != "", err
}

func SetResellerDomainNotFound(ctx context.Context, host string) error {
	return SetString(ctx, ResellerDomainNotFoundCacheKey(host), "1", ResellerDomainNegativeCacheTTL)
}

func DelResellerDomain(ctx context.Context, host string) error {
	if err := Del(ctx, ResellerDomainCacheKey(host)); err != nil {
		return err
	}
	return Del(ctx, ResellerDomainNotFoundCacheKey(host))
}
