package memcache

import (
	"time"

	"github.com/patrickmn/go-cache"
)

type Client struct {
	c *cache.Cache
}

type Config struct {
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
}

func New(cfg Config) *Client {
	return &Client{
		c: cache.New(cfg.DefaultTTL, cfg.CleanupInterval),
	}
}

func (mc *Client) Set(key string, val interface{}, ttl time.Duration) {
	mc.c.Set(key, val, ttl)
}

func (mc *Client) Get(key string) (interface{}, bool) {
	return mc.c.Get(key)
}

func (mc *Client) Delete(key string) {
	mc.c.Delete(key)
}

func (mc *Client) Flush() {
	mc.c.Flush()
}

func (mc *Client) Stats() int {
	return mc.c.ItemCount()
}
