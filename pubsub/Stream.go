package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// StreamOptions contains options for Redis Stream XADD operations.
type StreamOptions struct {
	MaxLen int64  // Maximum length of the stream (trimmed when exceeded)
	Approx bool   // Use approximate trimming (~ in Redis) for O(1) performance
	ID     string // Message ID, defaults to "*" (Redis auto-generated timestamp-sequence)
}

// StreamOption is a functional option for configuring StreamOptions.
type StreamOption func(*StreamOptions)

// WithMaxLen configures stream capping with MAXLEN.
// When approx is true, Redis uses MAXLEN ~ which is significantly faster.
func WithMaxLen(maxLen int64, approx bool) StreamOption {
	return func(o *StreamOptions) {
		o.MaxLen = maxLen
		o.Approx = approx
	}
}

// WithID specifies a custom message ID (defaults to "*" for auto-generation).
func WithID(id string) StreamOption {
	return func(o *StreamOptions) {
		o.ID = id
	}
}

// StreamPublisher publishes messages to Redis Streams.
type StreamPublisher interface {
	// PublishStream publishes an event or arbitrary payload to a stream.
	// If message is a map[string]interface{} or map[string]string, it is used directly as field-value pairs.
	// If message is []byte or string, it is stored under the "payload" key.
	// Otherwise, message is JSON-marshaled and stored under the "payload" key.
	// Returns the generated message ID.
	PublishStream(ctx context.Context, stream string, message interface{}, opts ...StreamOption) (string, error)

	// PublishStreamMap publishes explicit key-value fields to a stream.
	// Returns the generated message ID.
	PublishStreamMap(ctx context.Context, stream string, values map[string]interface{}, opts ...StreamOption) (string, error)
}

func defaultStreamOptions() StreamOptions {
	return StreamOptions{
		ID: "*",
	}
}

func applyStreamOptions(opts []StreamOption) StreamOptions {
	options := defaultStreamOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if options.ID == "" {
		options.ID = "*"
	}
	return options
}

// redisStreamPublisher wraps a redis.UniversalClient to provide StreamPublisher functionality.
type redisStreamPublisher struct {
	rdb       redis.UniversalClient
	namespace string
}

// NewRedisStreamPublisher creates a standalone StreamPublisher instance using a Redis client.
func NewRedisStreamPublisher(rdb redis.UniversalClient, namespace string) StreamPublisher {
	return &redisStreamPublisher{
		rdb:       rdb,
		namespace: namespace,
	}
}

func (p *redisStreamPublisher) prefixed(stream string) string {
	if p.namespace == "" {
		return stream
	}
	return fmt.Sprintf("%s:%s", p.namespace, stream)
}

func (p *redisStreamPublisher) PublishStream(ctx context.Context, stream string, message interface{}, opts ...StreamOption) (string, error) {
	if message == nil {
		return "", errors.New("stream message cannot be nil")
	}

	switch v := message.(type) {
	case map[string]interface{}:
		return p.PublishStreamMap(ctx, stream, v, opts...)
	case map[string]string:
		values := make(map[string]interface{}, len(v))
		for key, val := range v {
			values[key] = val
		}
		return p.PublishStreamMap(ctx, stream, values, opts...)
	case []byte:
		values := map[string]interface{}{
			"payload": string(v),
		}
		return p.PublishStreamMap(ctx, stream, values, opts...)
	case string:
		values := map[string]interface{}{
			"payload": v,
		}
		return p.PublishStreamMap(ctx, stream, values, opts...)
	default:
		bytes, err := json.Marshal(message)
		if err != nil {
			return "", fmt.Errorf("failed to encode stream message: %w", err)
		}
		values := map[string]interface{}{
			"payload": string(bytes),
		}
		return p.PublishStreamMap(ctx, stream, values, opts...)
	}
}

func (p *redisStreamPublisher) PublishStreamMap(ctx context.Context, stream string, values map[string]interface{}, opts ...StreamOption) (string, error) {
	if len(values) == 0 {
		return "", errors.New("stream values cannot be empty")
	}

	if p.rdb == nil {
		return "", errors.New("redis client is not initialized")
	}

	options := applyStreamOptions(opts)

	args := &redis.XAddArgs{
		Stream: p.prefixed(stream),
		ID:     options.ID,
		Values: values,
	}

	if options.MaxLen > 0 {
		args.MaxLen = options.MaxLen
		args.Approx = options.Approx
	}

	id, err := p.rdb.XAdd(ctx, args).Result()
	if err != nil {
		return "", fmt.Errorf("failed to publish to stream %s: %w", stream, err)
	}

	return id, nil
}
