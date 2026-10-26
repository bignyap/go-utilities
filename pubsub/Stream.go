package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// StreamOptions contains options for Redis Stream XADD and Kafka message operations.
type StreamOptions struct {
	MaxLen int64  // Maximum length of the stream (trimmed when exceeded, Redis only)
	Approx bool   // Use approximate trimming (~ in Redis) for O(1) performance
	ID     string // Message ID, defaults to "*" (Redis auto-generated timestamp-sequence)
	Key    string // Partition key for Kafka (or key field in Redis)
}

// StreamOption is a functional option for configuring StreamOptions.
type StreamOption func(*StreamOptions)

// WithMaxLen configures stream capping with MAXLEN (Redis only).
// When approx is true, Redis uses MAXLEN ~ which is significantly faster.
func WithMaxLen(maxLen int64, approx bool) StreamOption {
	return func(o *StreamOptions) {
		o.MaxLen = maxLen
		o.Approx = approx
	}
}

// WithID specifies a custom message ID (defaults to "*" for auto-generation in Redis).
func WithID(id string) StreamOption {
	return func(o *StreamOptions) {
		o.ID = id
	}
}

// WithKey sets the message key (used as partition key in Kafka, or key field in Redis).
func WithKey(key string) StreamOption {
	return func(o *StreamOptions) {
		o.Key = key
	}
}

// StreamPublisher publishes messages to Redis Streams or Kafka topics.
type StreamPublisher interface {
	// PublishStream publishes an event or arbitrary payload to a stream/topic.
	// If message is a map[string]interface{} or map[string]string, it is used directly as field-value pairs.
	// If message is []byte or string, it is stored under the "payload" key.
	// Otherwise, message is JSON-marshaled and stored under the "payload" key.
	// Returns the generated message ID (e.g. "1719324567890-0" for Redis, "0:1042" for Kafka).
	PublishStream(ctx context.Context, stream string, message interface{}, opts ...StreamOption) (string, error)

	// PublishStreamMap publishes explicit key-value fields to a stream/topic.
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

	if options.Key != "" && values["key"] == nil {
		values["key"] = options.Key
	}

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

// ============================================================================
// Stream Consumer (Consumer Groups)
// ============================================================================

// StreamMessage represents a message read from a stream.
type StreamMessage struct {
	ID     string
	Stream string
	Values map[string]interface{}
}

// PayloadBytes extracts the "payload" field as []byte.
func (m *StreamMessage) PayloadBytes() []byte {
	if m == nil || m.Values == nil {
		return nil
	}
	if p, ok := m.Values["payload"]; ok {
		switch v := p.(type) {
		case string:
			return []byte(v)
		case []byte:
			return v
		default:
			b, _ := json.Marshal(v)
			return b
		}
	}
	b, _ := json.Marshal(m.Values)
	return b
}

// StreamMessageHandler processes messages consumed from a stream.
type StreamMessageHandler func(ctx context.Context, msg *StreamMessage) error

// ConsumerOptions configures stream consumer behavior.
type ConsumerOptions struct {
	BatchSize   int64
	BlockTime   time.Duration
	AutoAck     bool
	StartFromID string
}

// ConsumerOption is a functional option for ConsumerOptions.
type ConsumerOption func(*ConsumerOptions)

// WithConsumerBatchSize sets the max message batch count per poll.
func WithConsumerBatchSize(batchSize int64) ConsumerOption {
	return func(o *ConsumerOptions) {
		if batchSize > 0 {
			o.BatchSize = batchSize
		}
	}
}

// WithConsumerBlockTime sets the timeout for blocking read.
func WithConsumerBlockTime(block time.Duration) ConsumerOption {
	return func(o *ConsumerOptions) {
		if block > 0 {
			o.BlockTime = block
		}
	}
}

// WithConsumerAutoAck configures whether messages are automatically ACKed on success.
func WithConsumerAutoAck(autoAck bool) ConsumerOption {
	return func(o *ConsumerOptions) {
		o.AutoAck = autoAck
	}
}

// WithConsumerStartFromID sets the starting ID when creating the group (e.g. "0", "$", ">").
func WithConsumerStartFromID(startID string) ConsumerOption {
	return func(o *ConsumerOptions) {
		if startID != "" {
			o.StartFromID = startID
		}
	}
}

// StreamConsumer provides consumer group consumption on Redis Streams.
type StreamConsumer interface {
	ConsumeStream(ctx context.Context, stream, group, consumer string, handler StreamMessageHandler, opts ...ConsumerOption) error
}

type redisStreamConsumer struct {
	rdb       redis.UniversalClient
	namespace string
}

// NewRedisStreamConsumer creates a new Redis-backed StreamConsumer.
func NewRedisStreamConsumer(rdb redis.UniversalClient, namespace string) StreamConsumer {
	return &redisStreamConsumer{
		rdb:       rdb,
		namespace: namespace,
	}
}

func (c *redisStreamConsumer) prefixed(stream string) string {
	if c.namespace == "" {
		return stream
	}
	return fmt.Sprintf("%s:%s", c.namespace, stream)
}

func (c *redisStreamConsumer) ConsumeStream(ctx context.Context, stream, group, consumer string, handler StreamMessageHandler, opts ...ConsumerOption) error {
	options := ConsumerOptions{
		BatchSize:   10,
		BlockTime:   2 * time.Second,
		AutoAck:     true,
		StartFromID: "$",
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	prefixedStream := c.prefixed(stream)

	// Ensure group exists
	if err := c.rdb.XGroupCreateMkStream(ctx, prefixedStream, group, options.StartFromID).Err(); err != nil {
		if !errors.Is(err, redis.Nil) && !strings.Contains(err.Error(), "BUSYGROUP") {
			return fmt.Errorf("failed to create consumer group: %w", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{prefixedStream, ">"},
			Count:    options.BatchSize,
			Block:    options.BlockTime,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			if errors.Is(err, context.Canceled) {
				return err
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for _, s := range res {
			for _, m := range s.Messages {
				msg := &StreamMessage{
					ID:     m.ID,
					Stream: stream,
					Values: m.Values,
				}

				handleErr := handler(ctx, msg)
				if handleErr == nil && options.AutoAck {
					_ = c.rdb.XAck(ctx, prefixedStream, group, m.ID).Err()
				}
			}
		}
	}
}
