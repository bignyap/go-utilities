package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDBTX struct {
	execFn func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	calls  []mockCall
}

type mockCall struct {
	sql  string
	args []any
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	m.calls = append(m.calls, mockCall{sql: sql, args: arguments})
	if m.execFn != nil {
		return m.execFn(ctx, sql, arguments...)
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func TestRecord_Validation(t *testing.T) {
	ctx := context.Background()
	mock := &mockDBTX{}

	t.Run("nil db", func(t *testing.T) {
		err := Record(ctx, nil, "stream", "type", "payload")
		assert.ErrorContains(t, err, "database executor cannot be nil")
	})

	t.Run("empty stream", func(t *testing.T) {
		err := Record(ctx, mock, "", "type", "payload")
		assert.ErrorContains(t, err, "stream name cannot be empty")
	})

	t.Run("empty event type", func(t *testing.T) {
		err := Record(ctx, mock, "stream", "", "payload")
		assert.ErrorContains(t, err, "event type cannot be empty")
	})

	t.Run("nil payload", func(t *testing.T) {
		err := Record(ctx, mock, "stream", "type", nil)
		assert.ErrorContains(t, err, "event payload cannot be nil")
	})
}

func TestRecord_PayloadTypes(t *testing.T) {
	ctx := context.Background()

	t.Run("struct payload", func(t *testing.T) {
		mock := &mockDBTX{}
		type Sample struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		data := Sample{Name: "kgb", Count: 42}

		err := Record(ctx, mock, "events", "SampleCreated", data, WithKey("sample-1"))
		require.NoError(t, err)
		require.Len(t, mock.calls, 1)

		args := mock.calls[0].args
		assert.Equal(t, "events", args[0])
		assert.Equal(t, "sample-1", args[1])
		assert.Equal(t, "SampleCreated", args[2])

		payloadBytes, ok := args[3].([]byte)
		require.True(t, ok)
		var unmarshaled Sample
		require.NoError(t, json.Unmarshal(payloadBytes, &unmarshaled))
		assert.Equal(t, data, unmarshaled)
	})

	t.Run("json raw message payload", func(t *testing.T) {
		mock := &mockDBTX{}
		raw := json.RawMessage(`{"status":"active"}`)

		err := Record(ctx, mock, "status-stream", "StatusUpdated", raw)
		require.NoError(t, err)
		require.Len(t, mock.calls, 1)

		payloadBytes := mock.calls[0].args[3].([]byte)
		assert.JSONEq(t, string(raw), string(payloadBytes))
	})

	t.Run("headers options", func(t *testing.T) {
		mock := &mockDBTX{}
		err := Record(ctx, mock, "stream", "type", "data",
			WithHeader("trace_id", "trace-123"),
			WithHeaders(map[string]string{"tenant_id": "tenant-456"}),
		)
		require.NoError(t, err)
		require.Len(t, mock.calls, 1)

		headersBytes := mock.calls[0].args[4].([]byte)
		var headers map[string]string
		require.NoError(t, json.Unmarshal(headersBytes, &headers))
		assert.Equal(t, "trace-123", headers["trace_id"])
		assert.Equal(t, "tenant-456", headers["tenant_id"])
	})

	t.Run("database error propagation", func(t *testing.T) {
		expectedErr := errors.New("unique constraint violation")
		mock := &mockDBTX{
			execFn: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, expectedErr
			},
		}

		err := Record(ctx, mock, "stream", "type", "payload")
		assert.ErrorIs(t, err, expectedErr)
	})
}

