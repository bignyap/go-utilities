package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the minimal database transaction interface satisfied by pgx.Tx,
// *pgxpool.Pool, and *pgx.Conn.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// RecordOptions configures additional metadata for outbox events.
type RecordOptions struct {
	Key     string
	Headers map[string]string
}

// RecordOption is a functional option for configuring RecordOptions.
type RecordOption func(*RecordOptions)

// WithKey sets the partition or routing key for the event stream.
func WithKey(key string) RecordOption {
	return func(o *RecordOptions) {
		o.Key = key
	}
}

// WithHeaders sets metadata headers on the outbox event.
func WithHeaders(headers map[string]string) RecordOption {
	return func(o *RecordOptions) {
		if o.Headers == nil {
			o.Headers = make(map[string]string, len(headers))
		}
		for k, v := range headers {
			o.Headers[k] = v
		}
	}
}

// WithHeader adds a single header key-value pair.
func WithHeader(key, value string) RecordOption {
	return func(o *RecordOptions) {
		if o.Headers == nil {
			o.Headers = make(map[string]string)
		}
		o.Headers[key] = value
	}
}

const insertOutboxSQL = `
INSERT INTO outbox_events (stream_name, event_key, event_type, payload, headers)
VALUES ($1, $2, $3, $4, $5)
`

// Record inserts a domain event into the outbox table within the provided database transaction.
// Because it runs inside the caller's db transaction, event persistence is atomically tied to business data mutations.
func Record(ctx context.Context, db DBTX, stream, eventType string, payload any, opts ...RecordOption) error {
	if db == nil {
		return errors.New("database executor cannot be nil")
	}
	if stream == "" {
		return errors.New("stream name cannot be empty")
	}
	if eventType == "" {
		return errors.New("event type cannot be empty")
	}
	if payload == nil {
		return errors.New("event payload cannot be nil")
	}

	options := RecordOptions{
		Headers: make(map[string]string),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	var payloadJSON []byte
	switch p := payload.(type) {
	case []byte:
		payloadJSON = p
	case json.RawMessage:
		payloadJSON = p
	case string:
		payloadJSON = []byte(p)
	default:
		var err error
		payloadJSON, err = json.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal outbox event payload: %w", err)
		}
	}

	var headersJSON []byte
	if len(options.Headers) > 0 {
		var err error
		headersJSON, err = json.Marshal(options.Headers)
		if err != nil {
			return fmt.Errorf("failed to marshal outbox event headers: %w", err)
		}
	} else {
		headersJSON = []byte("{}")
	}

	_, err := db.Exec(ctx, insertOutboxSQL, stream, options.Key, eventType, payloadJSON, headersJSON)
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

