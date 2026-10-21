package cache

import (
	"context"
	"fmt"
)

// Bump the namespace when the public config shape or startup normalization
// changes. Existing cached config may contain an older settlement currency;
// keeping it out of the read path avoids serving stale values without deleting
// arbitrary Redis data during startup.
const PublicConfigCachePrefix = "public:config:v2"

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
