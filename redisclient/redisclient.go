package redisclient

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	UseCluster bool     `json:"use_cluster" env:"REDIS_USE_CLUSTER"`
	Addrs      []string `json:"addrs" env:"REDIS_ADDRS"` // For cluster
	Addr       string   `json:"addr" env:"REDIS_ADDR"`   // For single-node
	Password   string   `json:"password" env:"REDIS_PASSWORD"`
	DB         int      `json:"db" env:"REDIS_DB"`
	PoolSize   int      `json:"pool_size" env:"REDIS_POOL_SIZE"`
}

func DefaultConfig() RedisConfig {
	return RedisConfig{
		Addr:     "localhost:6379",
		Addrs:    []string{"localhost:6379"},
		Password: "",
		DB:       0,
		PoolSize: 10,
	}
}

func (c *RedisConfig) applyDefaults() {
	defaults := DefaultConfig()

	if c.UseCluster && len(c.Addrs) == 0 {
		c.Addrs = defaults.Addrs
	}
	if !c.UseCluster && c.Addr == "" {
		c.Addr = defaults.Addr
	}
	if c.PoolSize == 0 {
		c.PoolSize = defaults.PoolSize
	}
}

var (
	client     redis.UniversalClient // works for both Client and ClusterClient
	once       sync.Once
	clientLock sync.RWMutex
)

func Init(cfg RedisConfig) error {
	var initErr error

	once.Do(func() {
		cfg.applyDefaults()

		var rdb redis.UniversalClient
		if cfg.UseCluster {
			rdb = redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:    cfg.Addrs,
				Password: cfg.Password,
				PoolSize: cfg.PoolSize,
			})
		} else {
			rdb = redis.NewClient(&redis.Options{
				Addr:     cfg.Addr,
				Password: cfg.Password,
				DB:       cfg.DB,
				PoolSize: cfg.PoolSize,
			})
		}

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

func GetRedisClient() (redis.UniversalClient, error) {
	clientLock.RLock()
	defer clientLock.RUnlock()

	if client == nil {
		return nil, ErrNotInitialized
	}
	return client, nil
}

func Close() error {
	clientLock.Lock()
	defer clientLock.Unlock()

	if client != nil {
		err := client.Close()
		client = nil
		return err
	}
	return nil
}
