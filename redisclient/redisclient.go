package redisclient

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr     string `json:"addr" env:"REDIS_ADDR" envDefault:"localhost:6379"`
	Password string `json:"password" env:"REDIS_PASSWORD"`
	DB       int    `json:"db" env:"REDIS_DB" envDefault:"0"`
	PoolSize int    `json:"pool_size" env:"REDIS_POOL_SIZE" envDefault:"10"`
}

func DefaultConfig() RedisConfig {
	return RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		PoolSize: 10,
	}
}

func (c *RedisConfig) applyDefaults() {
	defaults := DefaultConfig()

	if c.Addr == "" {
		c.Addr = defaults.Addr
	}
	if c.PoolSize == 0 {
		c.PoolSize = defaults.PoolSize
	}
	// Password and DB can remain zero-value if user intends so
}

var (
	client     *redis.Client
	once       sync.Once
	clientLock sync.RWMutex
)

// Init initializes the Redis client using the provided config.
// It's safe to call multiple times; it will only initialize once.
func Init(cfg RedisConfig) error {
	var initErr error

	once.Do(func() {
		cfg.applyDefaults()

		rdb := redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
			PoolSize: cfg.PoolSize,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := rdb.Ping(ctx).Err(); err != nil {
			initErr = errors.New("failed to connect to Redis: " + err.Error())
			return
		}

		clientLock.Lock()
		defer clientLock.Unlock()
		client = rdb
	})

	return initErr
}

var ErrNotInitialized = errors.New("redis client not initialized. call Init() first")

func GetRedisClient() (*redis.Client, error) {
	clientLock.RLock()
	defer clientLock.RUnlock()

	if client == nil {
		return nil, ErrNotInitialized
	}
	return client, nil
}

// Close gracefully closes the Redis client.
func Close() error {
	clientLock.Lock()
	defer clientLock.Unlock()

	if client != nil {
		err := client.Close()
		client = nil // allow reinitialization if needed
		return err
	}
	return nil
}
