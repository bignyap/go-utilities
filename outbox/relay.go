package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/bignyap/go-utilities/pubsub"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Default settings for outbox relay
const (
	DefaultBatchSize        = 100
	DefaultPollInterval     = 3 * time.Second
	DefaultChannelName      = "outbox_events_channel"
	DefaultMaxRetries       = 5
	DefaultDeleteOnSuccess  = true
)

// StreamPublisher abstracts event publication to Redis Streams or Kafka.
type StreamPublisher interface {
	PublishStream(ctx context.Context, stream string, message interface{}, opts ...pubsub.StreamOption) (string, error)
}

// RelayConfig configures the Outbox Relay worker.
type RelayConfig struct {
	BatchSize       int
	PollInterval    time.Duration
	ChannelName     string
	MaxRetries      int
	DeleteOnSuccess bool
	DLQStream       string
	OnDeadLetter    func(ctx context.Context, rec OutboxEventRecord, err error)
}

// RelayOption is a functional option for configuring Relay.
type RelayOption func(*RelayConfig)

// WithBatchSize sets the maximum number of events to process per batch.
func WithBatchSize(size int) RelayOption {
	return func(c *RelayConfig) {
		if size > 0 {
			c.BatchSize = size
		}
	}
}

// WithPollInterval sets the safety-net polling interval when no push notifications arrive.
func WithPollInterval(interval time.Duration) RelayOption {
	return func(c *RelayConfig) {
		if interval > 0 {
			c.PollInterval = interval
		}
	}
}

// WithChannelName sets the PostgreSQL LISTEN / NOTIFY channel name.
func WithChannelName(name string) RelayOption {
	return func(c *RelayConfig) {
		if name != "" {
			c.ChannelName = name
		}
	}
}

// WithMaxRetries sets the maximum number of failed publish attempts before an event is skipped.
func WithMaxRetries(retries int) RelayOption {
	return func(c *RelayConfig) {
		if retries > 0 {
			c.MaxRetries = retries
		}
	}
}

// WithDeleteOnSuccess configures whether processed events are deleted (true) or marked processed_at = NOW() (false).
func WithDeleteOnSuccess(delete bool) RelayOption {
	return func(c *RelayConfig) {
		c.DeleteOnSuccess = delete
	}
}

// WithDLQStream configures a Redis Stream to publish events that exceed MaxRetries.
func WithDLQStream(stream string) RelayOption {
	return func(c *RelayConfig) {
		c.DLQStream = stream
	}
}

// WithOnDeadLetter registers a callback invoked when an event permanently fails and exceeds MaxRetries.
func WithOnDeadLetter(fn func(ctx context.Context, rec OutboxEventRecord, err error)) RelayOption {
	return func(c *RelayConfig) {
		c.OnDeadLetter = fn
	}
}

// OutboxEventRecord represents an outbox row queried for dispatch.
type OutboxEventRecord struct {
	ID         int64
	StreamName string
	EventKey   string
	EventType  string
	Payload    []byte
	RetryCount int
}

// Relay is the worker responsible for pulling events from the outbox table and pushing them to Redis Streams.
type Relay struct {
	pool      *pgxpool.Pool
	publisher StreamPublisher
	logger    api.Logger
	config    RelayConfig
}

// NewRelay creates a new Outbox Relay instance.
func NewRelay(pool *pgxpool.Pool, publisher StreamPublisher, logger api.Logger, opts ...RelayOption) *Relay {
	cfg := RelayConfig{
		BatchSize:       DefaultBatchSize,
		PollInterval:    DefaultPollInterval,
		ChannelName:     DefaultChannelName,
		MaxRetries:      DefaultMaxRetries,
		DeleteOnSuccess: DefaultDeleteOnSuccess,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	var l api.Logger
	if logger != nil {
		l = logger.WithComponent("outbox-relay")
	}

	return &Relay{
		pool:      pool,
		publisher: publisher,
		logger:    l,
		config:    cfg,
	}
}

// Start runs the relay processing loop until the provided context is canceled.
// It uses PostgreSQL LISTEN / NOTIFY for real-time (<5ms) push wakeups, falling back to a periodic poll.
func (r *Relay) Start(ctx context.Context) error {
	if r.pool == nil {
		return errors.New("database pool cannot be nil")
	}
	if r.publisher == nil {
		return errors.New("stream publisher cannot be nil")
	}

	r.logInfo(ctx, "Starting Outbox Relay worker",
		api.String("channel", r.config.ChannelName),
		api.Int("batch_size", r.config.BatchSize),
		api.Duration("fallback_interval", r.config.PollInterval),
	)

	for {
		if ctx.Err() != nil {
			r.logInfo(ctx, "Outbox Relay worker stopped gracefully", api.String("reason", ctx.Err().Error()))
			return ctx.Err()
		}

		err := r.listenAndDrain(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			r.logWarn(ctx, "Outbox notification listener encountered error, reconnecting in 1s", api.String("error", err.Error()))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
			}
		}
	}
}

// listenAndDrain acquires a dedicated connection, listens on the notification channel, and processes events.
func (r *Relay) listenAndDrain(ctx context.Context) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire listener connection: %w", err)
	}
	defer conn.Release()

	listenSQL := fmt.Sprintf("LISTEN %s", pgx.Identifier{r.config.ChannelName}.Sanitize())
	if _, err := conn.Exec(ctx, listenSQL); err != nil {
		return fmt.Errorf("failed to execute LISTEN: %w", err)
	}

	r.logDebug(ctx, "Listening for outbox notification signals", api.String("channel", r.config.ChannelName))

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Drain any existing unprocessed events first
		for {
			count, drainErr := r.drainBatch(ctx)
			if drainErr != nil {
				r.logError(ctx, "Error draining outbox batch", drainErr)
				break
			}
			if count < r.config.BatchSize {
				break
			}
		}

		// Wait for notification push or safety-net timeout
		waitCtx, cancel := context.WithTimeout(ctx, r.config.PollInterval)
		_, waitErr := conn.Conn().WaitForNotification(waitCtx)
		cancel()

		if waitErr != nil {
			// Timeout is normal (safety-net poll trigger)
			if errors.Is(waitErr, context.DeadlineExceeded) {
				continue
			}
			if errors.Is(waitErr, context.Canceled) {
				return waitErr
			}
			return fmt.Errorf("notification wait failed: %w", waitErr)
		}
	}
}

const selectBatchSQL = `
SELECT id, stream_name, COALESCE(event_key, ''), event_type, payload, retry_count
FROM outbox_events
WHERE processed_at IS NULL AND retry_count < $2
ORDER BY id ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;
`

const deleteBatchSQL = `
DELETE FROM outbox_events
WHERE id = ANY($1);
`

const updateProcessedSQL = `
UPDATE outbox_events
SET processed_at = NOW()
WHERE id = ANY($1);
`

const updateRetrySQL = `
UPDATE outbox_events
SET retry_count = retry_count + 1, last_error = $2
WHERE id = $1;
`

// drainBatch selects and publishes a batch of pending events.
func (r *Relay) drainBatch(ctx context.Context) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to start drain transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, selectBatchSQL, r.config.BatchSize, r.config.MaxRetries)
	if err != nil {
		return 0, fmt.Errorf("failed to query outbox batch: %w", err)
	}
	defer rows.Close()

	var records []OutboxEventRecord
	for rows.Next() {
		var rec OutboxEventRecord
		if err := rows.Scan(&rec.ID, &rec.StreamName, &rec.EventKey, &rec.EventType, &rec.Payload, &rec.RetryCount); err != nil {
			return 0, fmt.Errorf("failed to scan outbox record: %w", err)
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("rows iteration error: %w", err)
	}

	if len(records) == 0 {
		return 0, nil
	}

	var successfulIDs []int64
	for _, rec := range records {
		var streamOpts []pubsub.StreamOption
		if rec.EventKey != "" {
			streamOpts = append(streamOpts, pubsub.WithKey(rec.EventKey))
		}

		// If payload is a JSON object, unpack into map[string]interface{} so Redis Stream fields are preserved
		var streamPayload any = rec.Payload
		var mapPayload map[string]interface{}
		if err := json.Unmarshal(rec.Payload, &mapPayload); err == nil && len(mapPayload) > 0 {
			streamPayload = mapPayload
		}

		_, pubErr := r.publisher.PublishStream(ctx, rec.StreamName, streamPayload, streamOpts...)
		if pubErr != nil {
			r.logWarn(ctx, "Failed to publish outbox event to stream",
				api.Int64("event_id", rec.ID),
				api.String("stream", rec.StreamName),
				api.String("event_type", rec.EventType),
				api.String("error", pubErr.Error()),
			)

			// If the event has reached max retries, move to dead letter queue
			if rec.RetryCount+1 >= r.config.MaxRetries {
				r.logError(ctx, "Outbox event exceeded max retries, moving to dead-letter queue", pubErr,
					api.Int64("event_id", rec.ID),
					api.String("stream", rec.StreamName),
					api.String("event_type", rec.EventType),
				)
				if r.config.DLQStream != "" {
					dlqPayload := map[string]interface{}{
						"original_id":     rec.ID,
						"original_stream": rec.StreamName,
						"event_type":      rec.EventType,
						"event_key":       rec.EventKey,
						"payload":         string(rec.Payload),
						"error":           pubErr.Error(),
						"dead_lettered_at": time.Now().UTC().Format(time.RFC3339),
					}
					_, _ = r.publisher.PublishStream(ctx, r.config.DLQStream, dlqPayload)
				}
				if r.config.OnDeadLetter != nil {
					r.config.OnDeadLetter(ctx, rec, pubErr)
				}
				deadLetterErr := fmt.Sprintf("DEAD_LETTER: %v", pubErr)
				if _, execErr := tx.Exec(ctx, updateRetrySQL, rec.ID, deadLetterErr); execErr != nil {
					r.logError(ctx, "Failed to update outbox event dead-letter status", execErr, api.Int64("event_id", rec.ID))
				}
			} else {
				// Increment retry count inside the transaction
				if _, execErr := tx.Exec(ctx, updateRetrySQL, rec.ID, pubErr.Error()); execErr != nil {
					r.logError(ctx, "Failed to update outbox event retry count", execErr, api.Int64("event_id", rec.ID))
				}
			}
			continue
		}

		// If publisher also supports PubSub channel broadcast, publish to channel as well
		// for backward compatibility with existing Redis Pub/Sub subscribers
		if broadcaster, ok := r.publisher.(interface {
			Publish(ctx context.Context, channel string, message interface{}) error
		}); ok {
			if bErr := broadcaster.Publish(ctx, rec.StreamName, rec.Payload); bErr != nil {
				r.logWarn(ctx, "Failed to broadcast outbox event to pubsub channel",
					api.Int64("event_id", rec.ID),
					api.String("channel", rec.StreamName),
					api.String("error", bErr.Error()),
				)
			}
		}

		successfulIDs = append(successfulIDs, rec.ID)
	}

	if len(successfulIDs) > 0 {
		if r.config.DeleteOnSuccess {
			if _, err := tx.Exec(ctx, deleteBatchSQL, successfulIDs); err != nil {
				return 0, fmt.Errorf("failed to delete processed outbox records: %w", err)
			}
		} else {
			if _, err := tx.Exec(ctx, updateProcessedSQL, successfulIDs); err != nil {
				return 0, fmt.Errorf("failed to update processed outbox records: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit outbox drain transaction: %w", err)
	}

	r.logDebug(ctx, "Successfully processed outbox batch",
		api.Int("batch_size", len(records)),
		api.Int("success_count", len(successfulIDs)),
	)

	return len(records), nil
}

func (r *Relay) logInfo(ctx context.Context, msg string, fields ...api.Field) {
	if r.logger != nil {
		r.logger.Info(ctx, msg, fields...)
	}
}

func (r *Relay) logDebug(ctx context.Context, msg string, fields ...api.Field) {
	if r.logger != nil {
		r.logger.Debug(ctx, msg, fields...)
	}
}

func (r *Relay) logWarn(ctx context.Context, msg string, fields ...api.Field) {
	if r.logger != nil {
		r.logger.Warn(ctx, msg, fields...)
	}
}

func (r *Relay) logError(ctx context.Context, msg string, err error, fields ...api.Field) {
	if r.logger != nil {
		r.logger.Error(ctx, msg, err, fields...)
	}
}
