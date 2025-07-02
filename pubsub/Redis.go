package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisPubSub struct {
	rdb       *redis.Client
	namespace string
}

func NewRedisPubSub(cfg Config) (PubSubClient, error) {
	if !cfg.Enabled {
		return &noopPubSub{}, nil
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.URL,
		Password: cfg.Redis.Password,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisPubSub{
		rdb:       rdb,
		namespace: cfg.Namespace,
	}, nil
}

func (r *RedisPubSub) prefixed(channel string) string {
	if r.namespace == "" {
		return channel
	}
	return fmt.Sprintf("%s:%s", r.namespace, channel)
}

func (r *RedisPubSub) Publish(ctx context.Context, channel string, message interface{}) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}
	return r.rdb.Publish(ctx, r.prefixed(channel), bytes).Err()
}

func (r *RedisPubSub) Subscribe(ctx context.Context, channel string, handler MessageHandler) error {
	sub := r.rdb.Subscribe(ctx, r.prefixed(channel))
	ch := sub.Channel()

	go func() {
		for msg := range ch {
			err := handler(ctx, []byte(msg.Payload))
			if err != nil {
				log.Printf("pubsub handler error on channel %s: %v", channel, err)
			}
		}
	}()
	return nil
}

func (r *RedisPubSub) Close() error {
	return r.rdb.Close()
}
