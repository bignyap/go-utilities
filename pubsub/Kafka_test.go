package pubsub

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
)

func TestKafkaStreamPublisherWithMock(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	mockProducer := mocks.NewSyncProducer(t, config)
	mockProducer.ExpectSendMessageWithCheckerFunctionAndSucceed(func(val []byte) error {
		return nil
	})

	pub := NewKafkaStreamPublisher(mockProducer, "kgb")
	ctx := context.Background()

	// Test publishing with partition key
	id, err := pub.PublishStream(ctx, "events", map[string]interface{}{
		"event_id": "evt-123",
		"action":   "created",
	}, WithKey("tenant-xyz"))

	if err != nil {
		t.Fatalf("unexpected error publishing to kafka: %v", err)
	}

	// Mock producer assigns partition 13, offset 1 by default
	if id != "13:1" {
		t.Errorf("expected message id '13:1', got '%s'", id)
	}

	if err := mockProducer.Close(); err != nil {
		t.Errorf("failed to close mock producer: %v", err)
	}
}

func TestKafkaStreamPublisherValidation(t *testing.T) {
	pub := NewKafkaStreamPublisher(nil, "kgb")
	ctx := context.Background()

	// Nil message validation
	_, err := pub.PublishStream(ctx, "topic", nil)
	if err == nil {
		t.Errorf("expected error for nil message, got nil")
	}

	// Nil producer validation
	_, err = pub.PublishStream(ctx, "topic", "test-payload")
	if err == nil {
		t.Errorf("expected error for nil kafka producer, got nil")
	}

	// Empty map validation
	_, err = pub.PublishStreamMap(ctx, "topic", map[string]interface{}{})
	if err == nil {
		t.Errorf("expected error for empty map, got nil")
	}
}

func TestNewKafkaPubSubConfigValidation(t *testing.T) {
	// Missing Kafka config
	_, err := NewPubSub(Config{
		Type:    "kafka",
		Enabled: true,
		Kafka:   nil,
	})
	if err == nil {
		t.Errorf("expected error for missing Kafka config, got nil")
	}

	// Empty brokers
	_, err = NewPubSub(Config{
		Type:    "kafka",
		Enabled: true,
		Kafka: &KafkaConfig{
			Brokers: []string{},
		},
	})
	if err == nil {
		t.Errorf("expected error for empty brokers, got nil")
	}

	// Disabled should return noop
	client, err := NewPubSub(Config{
		Type:    "kafka",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("unexpected error for disabled pubsub: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil noop client")
	}
}
