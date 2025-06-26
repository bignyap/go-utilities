package memcache

import (
	"time"

	"github.com/patrickmn/go-cache"
)

type Config struct {
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
}

var (
	c      *cache.Cache
	config Config
)

func Init(cfg Config) {
	config = cfg
	c = cache.New(cfg.DefaultTTL, cfg.CleanupInterval)
}

func Set(key string, val interface{}, ttl ...time.Duration) {
	expiry := config.DefaultTTL
	if len(ttl) > 0 {
		expiry = ttl[0]
	}
	c.Set(key, val, expiry)
}

func Get(key string) (interface{}, bool) {
	return c.Get(key)
}

func Invalidate(key string) {
	c.Delete(key)
}

func Flush() {
	c.Flush()
}

func Stats() int {
	return c.ItemCount()
}
