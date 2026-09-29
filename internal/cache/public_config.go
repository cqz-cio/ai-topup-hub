package cache

import (
	"context"
	"fmt"
)

const PublicConfigCachePrefix = "public:config"

func PublicConfigCacheKey(resellerID *uint) string {
	if resellerID == nil {
		return PublicConfigCachePrefix + ":main"
	}
	return fmt.Sprintf("%s:reseller:%d", PublicConfigCachePrefix, *resellerID)
}

func DelAllPublicConfig(ctx context.Context) error {
	if err := Del(ctx, PublicConfigCachePrefix); err != nil {
		return err
	}
	return DelPattern(ctx, PublicConfigCachePrefix+":*")
}
