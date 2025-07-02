package pubsub

import (
	"context"
	"errors"
	"fmt"
)

func NewPubSub(cfg Config) (PubSubClient, error) {
	if !cfg.Enabled {
		return &noopPubSub{}, nil
	}

	switch cfg.Type {
	case "redis":
		if cfg.Redis == nil {
			return nil, errors.New("missing Redis config")
		}
		return NewRedisPubSub(cfg)
	// case "kafka":
	// 	if cfg.Kafka == nil {
	// 		return nil, errors.New("missing Kafka config")
	// 	}
	// 	return NewKafkaPubSub(cfg)
	// case "rabbitmq":
	// 	if cfg.RabbitMQ == nil {
	// 		return nil, errors.New("missing RabbitMQ config")
	// 	}
	// 	return NewRabbitMQPubSub(cfg)
	case "none":
		return &noopPubSub{}, nil
	default:
		return nil, fmt.Errorf("unsupported pubsub type: %s", cfg.Type)
	}
}

type PubSubClient interface {
	Publish(ctx context.Context, channel string, message interface{}) error
	Subscribe(ctx context.Context, channel string, handler MessageHandler) error
	Close() error
}

type MessageHandler func(ctx context.Context, payload []byte) error
