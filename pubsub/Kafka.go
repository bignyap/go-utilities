package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
)

// kafkaStreamPublisher wraps a sarama.SyncProducer to implement StreamPublisher.
type kafkaStreamPublisher struct {
	producer  sarama.SyncProducer
	namespace string
}

// NewKafkaStreamPublisher creates a StreamPublisher using an existing sarama.SyncProducer.
func NewKafkaStreamPublisher(producer sarama.SyncProducer, namespace string) StreamPublisher {
	return &kafkaStreamPublisher{
		producer:  producer,
		namespace: namespace,
	}
}

// NewKafkaStreamPublisherFromConfig creates a standalone Kafka StreamPublisher from a KafkaConfig.
func NewKafkaStreamPublisherFromConfig(cfg KafkaConfig, namespace string) (StreamPublisher, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("missing or empty Kafka brokers in config")
	}

	saramaConfig := newSaramaProducerConfig(cfg)
	producer, err := sarama.NewSyncProducer(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka sync producer: %w", err)
	}

	return NewKafkaStreamPublisher(producer, namespace), nil
}

func (p *kafkaStreamPublisher) prefixed(topic string) string {
	if p.namespace == "" {
		return topic
	}
	return fmt.Sprintf("%s:%s", p.namespace, topic)
}

func (p *kafkaStreamPublisher) PublishStream(ctx context.Context, stream string, message interface{}, opts ...StreamOption) (string, error) {
	if message == nil {
		return "", errors.New("stream message cannot be nil")
	}
	if p.producer == nil {
		return "", errors.New("kafka producer is not initialized")
	}

	var data []byte
	switch v := message.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		var err error
		data, err = json.Marshal(message)
		if err != nil {
			return "", fmt.Errorf("failed to encode kafka stream message: %w", err)
		}
	}

	options := applyStreamOptions(opts)
	topic := p.prefixed(stream)

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	}

	if options.Key != "" {
		msg.Key = sarama.StringEncoder(options.Key)
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		return "", fmt.Errorf("failed to publish to kafka topic %s: %w", topic, err)
	}

	return fmt.Sprintf("%d:%d", partition, offset), nil
}

func (p *kafkaStreamPublisher) PublishStreamMap(ctx context.Context, stream string, values map[string]interface{}, opts ...StreamOption) (string, error) {
	if len(values) == 0 {
		return "", errors.New("stream values cannot be empty")
	}
	return p.PublishStream(ctx, stream, values, opts...)
}

func newSaramaProducerConfig(cfg KafkaConfig) *sarama.Config {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Retry.Max = 5
	config.Producer.Retry.Backoff = 200 * time.Millisecond
	config.Consumer.Return.Errors = true

	// Handle SASL / Auth
	if cfg.Username != "" && cfg.Password != "" {
		config.Net.SASL.Enable = true
		config.Net.SASL.User = cfg.Username
		config.Net.SASL.Password = cfg.Password
	}

	// Handle TLS
	if cfg.UseTLS {
		config.Net.TLS.Enable = true
	}

	return config
}

// KafkaPubSub implements PubSubClient using Apache Kafka.
type KafkaPubSub struct {
	producer      sarama.SyncProducer
	consumerGroup sarama.ConsumerGroup
	namespace     string
	streamPub     StreamPublisher
}

// NewKafkaPubSub creates a new PubSubClient backed by Kafka.
func NewKafkaPubSub(cfg Config) (PubSubClient, error) {
	if !cfg.Enabled {
		return &noopPubSub{}, nil
	}
	if cfg.Kafka == nil || len(cfg.Kafka.Brokers) == 0 {
		return nil, errors.New("missing or empty Kafka brokers in config")
	}

	saramaConfig := newSaramaProducerConfig(*cfg.Kafka)

	producer, err := sarama.NewSyncProducer(cfg.Kafka.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka sync producer: %w", err)
	}

	var consumerGroup sarama.ConsumerGroup
	if cfg.Kafka.GroupID != "" {
		cg, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, cfg.Kafka.GroupID, saramaConfig)
		if err != nil {
			_ = producer.Close()
			return nil, fmt.Errorf("failed to create kafka consumer group: %w", err)
		}
		consumerGroup = cg
	}

	return &KafkaPubSub{
		producer:      producer,
		consumerGroup: consumerGroup,
		namespace:     cfg.Namespace,
		streamPub:     NewKafkaStreamPublisher(producer, cfg.Namespace),
	}, nil
}

func (k *KafkaPubSub) prefixed(channel string) string {
	if k.namespace == "" {
		return channel
	}
	return fmt.Sprintf("%s:%s", k.namespace, channel)
}

func (k *KafkaPubSub) Publish(ctx context.Context, channel string, message interface{}) error {
	_, err := k.streamPub.PublishStream(ctx, channel, message)
	return err
}

func (k *KafkaPubSub) Subscribe(ctx context.Context, channel string, handler MessageHandler) error {
	if k.consumerGroup == nil {
		return errors.New("cannot subscribe: kafka consumer group is not configured (GroupID is empty)")
	}

	topic := k.prefixed(channel)
	cgh := &kafkaConsumerGroupHandler{
		ctx:     ctx,
		handler: handler,
	}

	go func() {
		for {
			if err := k.consumerGroup.Consume(ctx, []string{topic}, cgh); err != nil {
				log.Printf("kafka consumer group error on topic %s: %v", topic, err)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	return nil
}

func (k *KafkaPubSub) PublishStream(ctx context.Context, stream string, message interface{}, opts ...StreamOption) (string, error) {
	return k.streamPub.PublishStream(ctx, stream, message, opts...)
}

func (k *KafkaPubSub) PublishStreamMap(ctx context.Context, stream string, values map[string]interface{}, opts ...StreamOption) (string, error) {
	return k.streamPub.PublishStreamMap(ctx, stream, values, opts...)
}

func (k *KafkaPubSub) Producer() sarama.SyncProducer {
	return k.producer
}

func (k *KafkaPubSub) ConsumerGroup() sarama.ConsumerGroup {
	return k.consumerGroup
}

func (k *KafkaPubSub) Close() error {
	var errs []error
	if k.producer != nil {
		if err := k.producer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if k.consumerGroup != nil {
		if err := k.consumerGroup.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing kafka pubsub: %v", errs)
	}
	return nil
}

type kafkaConsumerGroupHandler struct {
	ctx     context.Context
	handler MessageHandler
}

func (h *kafkaConsumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *kafkaConsumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *kafkaConsumerGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		err := h.handler(h.ctx, msg.Value)
		if err != nil {
			log.Printf("kafka message handler error on topic %s: %v", msg.Topic, err)
		}
		sess.MarkMessage(msg, "")
	}
	return nil
}
