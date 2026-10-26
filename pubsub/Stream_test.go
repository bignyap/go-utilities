package pubsub

import (
	"context"
	"testing"
	"time"
)

type sampleTestEvent struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Timestamp int64  `json:"timestamp"`
}

func TestStreamOptions(t *testing.T) {
	// Test defaults
	opts := applyStreamOptions(nil)
	if opts.ID != "*" {
		t.Errorf("expected default ID '*', got '%s'", opts.ID)
	}
	if opts.MaxLen != 0 {
		t.Errorf("expected default MaxLen 0, got %d", opts.MaxLen)
	}
	if opts.Approx {
		t.Errorf("expected default Approx false, got true")
	}
	if opts.Key != "" {
		t.Errorf("expected default Key '', got '%s'", opts.Key)
	}

	// Test custom options
	customOpts := applyStreamOptions([]StreamOption{
		WithMaxLen(5000, true),
		WithID("1234567890-0"),
		WithKey("tenant-123"),
	})
	if customOpts.ID != "1234567890-0" {
		t.Errorf("expected ID '1234567890-0', got '%s'", customOpts.ID)
	}
	if customOpts.MaxLen != 5000 {
		t.Errorf("expected MaxLen 5000, got %d", customOpts.MaxLen)
	}
	if !customOpts.Approx {
		t.Errorf("expected Approx true, got false")
	}
	if customOpts.Key != "tenant-123" {
		t.Errorf("expected Key 'tenant-123', got '%s'", customOpts.Key)
	}
}

func TestNoopStreamPublisher(t *testing.T) {
	pub := NewNoopStreamPublisher()
	ctx := context.Background()

	// Struct payload
	id, err := pub.PublishStream(ctx, "events", sampleTestEvent{
		ID:        "evt-1",
		Action:    "created",
		Timestamp: 123456,
	}, WithMaxLen(100, true), WithKey("tenant-abc"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "0-0" {
		t.Errorf("expected id '0-0', got '%s'", id)
	}

	// Map payload
	id, err = pub.PublishStreamMap(ctx, "tasks", map[string]interface{}{
		"task_id": "t-1",
		"status":  "pending",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "0-0" {
		t.Errorf("expected id '0-0', got '%s'", id)
	}
}

func TestRedisStreamPublisherValidation(t *testing.T) {
	pub := NewRedisStreamPublisher(nil, "kgb")
	ctx := context.Background()

	// Nil message validation
	_, err := pub.PublishStream(ctx, "stream", nil)
	if err == nil {
		t.Errorf("expected error for nil message, got nil")
	}

	// Empty map validation
	_, err = pub.PublishStreamMap(ctx, "stream", map[string]interface{}{})
	if err == nil {
		t.Errorf("expected error for empty map, got nil")
	}

	// Nil redis client validation
	_, err = pub.PublishStreamMap(ctx, "stream", map[string]interface{}{"foo": "bar"})
	if err == nil {
		t.Errorf("expected error for nil redis client, got nil")
	}
}

func TestNoopPubSubImplementsBoth(t *testing.T) {
	// Verify noopPubSub implements PubSubClient and StreamPublisher
	var client PubSubClient = &noopPubSub{}
	if client == nil {
		t.Fatal("expected client to not be nil")
	}

	var streamPub StreamPublisher = client
	if streamPub == nil {
		t.Fatal("expected streamPub to not be nil")
	}

	ctx := context.Background()
	id, err := streamPub.PublishStream(ctx, "test", "hello")
	if err != nil || id != "0-0" {
		t.Errorf("expected id '0-0' and nil error, got id='%s', err=%v", id, err)
	}
}

func TestStreamMessage_PayloadBytes(t *testing.T) {
	// String payload
	msg1 := &StreamMessage{
		ID:     "1-0",
		Stream: "test",
		Values: map[string]interface{}{"payload": "hello world"},
	}
	if string(msg1.PayloadBytes()) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(msg1.PayloadBytes()))
	}

	// Bytes payload
	msg2 := &StreamMessage{
		ID:     "2-0",
		Stream: "test",
		Values: map[string]interface{}{"payload": []byte("byte slice")},
	}
	if string(msg2.PayloadBytes()) != "byte slice" {
		t.Errorf("expected 'byte slice', got '%s'", string(msg2.PayloadBytes()))
	}

	// Arbitrary values fallback
	msg3 := &StreamMessage{
		ID:     "3-0",
		Stream: "test",
		Values: map[string]interface{}{"key": "val"},
	}
	if len(msg3.PayloadBytes()) == 0 {
		t.Errorf("expected non-empty bytes for fallback payload")
	}
}

func TestConsumerOptions(t *testing.T) {
	opts := ConsumerOptions{
		BatchSize:   10,
		BlockTime:   2 * time.Second,
		AutoAck:     true,
		StartFromID: "$",
	}

	optFuncs := []ConsumerOption{
		WithConsumerBatchSize(50),
		WithConsumerBlockTime(5 * time.Second),
		WithConsumerAutoAck(false),
		WithConsumerStartFromID("0"),
	}

	for _, fn := range optFuncs {
		fn(&opts)
	}

	if opts.BatchSize != 50 {
		t.Errorf("expected BatchSize 50, got %d", opts.BatchSize)
	}
	if opts.BlockTime != 5*time.Second {
		t.Errorf("expected BlockTime 5s, got %v", opts.BlockTime)
	}
	if opts.AutoAck != false {
		t.Errorf("expected AutoAck false, got true")
	}
	if opts.StartFromID != "0" {
		t.Errorf("expected StartFromID '0', got '%s'", opts.StartFromID)
	}
}
