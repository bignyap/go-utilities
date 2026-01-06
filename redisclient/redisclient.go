package redisclient

import (
	"context"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	UseCluster      bool     `json:"use_cluster" env:"REDIS_USE_CLUSTER"`
	Addrs           []string `json:"addrs" env:"REDIS_ADDRS"` // For cluster
	Addr            string   `json:"addr" env:"REDIS_ADDR"`   // For single-node
	Password        string   `json:"password" env:"REDIS_PASSWORD"`
	DB              int      `json:"db" env:"REDIS_DB"`
	PoolSize        int      `json:"pool_size" env:"REDIS_POOL_SIZE"`
	EnableTelemetry bool     `json:"enable_telemetry" env:"REDIS_ENABLE_TELEMETRY"` // Enable OpenTelemetry tracing
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

func New(ctx context.Context, cfg RedisConfig) (redis.UniversalClient, error) {

	cfg.applyDefaults()

	var client redis.UniversalClient
	if cfg.UseCluster {
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    cfg.Addrs,
			Password: cfg.Password,
			PoolSize: cfg.PoolSize,
		})
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
			PoolSize: cfg.PoolSize,
		})
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(ctxTimeout).Err(); err != nil {
		return nil, err
	}

	// Add OpenTelemetry instrumentation if enabled
	if cfg.EnableTelemetry {
		if err := redisotel.InstrumentTracing(client); err != nil {
			return nil, err
		}
		if err := redisotel.InstrumentMetrics(client); err != nil {
			return nil, err
		}
	}

	return client, nil
}
