package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/bignyap/go-utilities/pubsub"
	"github.com/stretchr/testify/assert"
)

type mockStreamPublisher struct {
	published []mockPublishedMessage
	err       error
}

type mockPublishedMessage struct {
	stream  string
	message any
	opts    []pubsub.StreamOption
}

func (m *mockStreamPublisher) PublishStream(ctx context.Context, stream string, message interface{}, opts ...pubsub.StreamOption) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	m.published = append(m.published, mockPublishedMessage{
		stream:  stream,
		message: message,
		opts:    opts,
	})
	return "msg-123", nil
}

func TestNewRelay_Options(t *testing.T) {
	pub := &mockStreamPublisher{}

	t.Run("default options", func(t *testing.T) {
		relay := NewRelay(nil, pub, nil)
		assert.Equal(t, DefaultBatchSize, relay.config.BatchSize)
		assert.Equal(t, DefaultPollInterval, relay.config.PollInterval)
		assert.Equal(t, DefaultChannelName, relay.config.ChannelName)
		assert.Equal(t, DefaultMaxRetries, relay.config.MaxRetries)
		assert.True(t, relay.config.DeleteOnSuccess)
	})

	t.Run("custom options", func(t *testing.T) {
		relay := NewRelay(nil, pub, nil,
			WithBatchSize(250),
			WithPollInterval(5*time.Second),
			WithChannelName("custom_outbox"),
			WithMaxRetries(10),
			WithDeleteOnSuccess(false),
		)
		assert.Equal(t, 250, relay.config.BatchSize)
		assert.Equal(t, 5*time.Second, relay.config.PollInterval)
		assert.Equal(t, "custom_outbox", relay.config.ChannelName)
		assert.Equal(t, 10, relay.config.MaxRetries)
		assert.False(t, relay.config.DeleteOnSuccess)
	})

	t.Run("start validates nil pool or publisher", func(t *testing.T) {
		ctx := context.Background()

		rNilPool := NewRelay(nil, pub, nil)
		assert.ErrorContains(t, rNilPool.Start(ctx), "database pool cannot be nil")

		rNilPub := NewRelay(nil, nil, nil)
		assert.ErrorContains(t, rNilPub.Start(ctx), "database pool cannot be nil")
	})
}

