package kafka

import (
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/bignyap/go-utilities/server"
)

// ++++++++++++++++++    BASE PRODUCER   +++++++++++++++++++++

type Producer interface {
	Init() error
	Close() error
	SendMessage(msg interface{}) error
}

type BaseProducer struct {
	producer sarama.SyncProducer
	topic    string
}

func (bp *BaseProducer) SendMessage(msg interface{}) error {
	tq := TopicQueue{Producer: bp.producer, Topic: bp.topic}
	return tq.SendMessage(msg)
}

func (bp *BaseProducer) Init() error  { return nil }
func (bp *BaseProducer) Close() error { return bp.producer.Close() }

// BaseProducerOptions defines options for BaseProducerConfig
type BaseProducerOptions struct {
	IncludeFlushConfigs bool                    `json:"include_flush_configs" env:"BROKER_INCLUDE_FLUSH_CONFIGS"`
	Compression         sarama.CompressionCodec `json:"compression" env:"BROKER_COMPRESSION"`
	EnableIdempotence   bool                    `json:"enable_idempotence" env:"BROKER_ENABLE_IDEMPOTENCE"`
	ClientID            string                  `json:"client_id" env:"BROKER_CLIENT_ID"`
	MaxMessageBytes     int                     `json:"max_message_bytes" env:"BROKER_MAX_MESSAGE_BYTES"`
}

func BaseProducerConfig(userOpts *BaseProducerOptions) *sarama.Config {
	defaultOpts := BaseProducerOptions{
		IncludeFlushConfigs: true,
		Compression:         sarama.CompressionSnappy,
		EnableIdempotence:   true,
		ClientID:            "default-producer",
		MaxMessageBytes:     1000000,
	}

	// Override defaults with user-specified options
	if userOpts != nil {
		if userOpts.ClientID != "" {
			defaultOpts.ClientID = userOpts.ClientID
		}
		if userOpts.Compression != 0 {
			defaultOpts.Compression = userOpts.Compression
		}
		if userOpts.MaxMessageBytes != 0 {
			defaultOpts.MaxMessageBytes = userOpts.MaxMessageBytes
		}
		defaultOpts.IncludeFlushConfigs = userOpts.IncludeFlushConfigs
		defaultOpts.EnableIdempotence = userOpts.EnableIdempotence
	}

	config := sarama.NewConfig()
	config.Version = sarama.V1_1_0_0
	config.Net.TLS.Enable = true
	config.Net.SASL.Enable = true

	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Retry.Max = 10
	config.Producer.Retry.Backoff = 200 * time.Millisecond

	config.ClientID = defaultOpts.ClientID
	config.Producer.Compression = defaultOpts.Compression
	config.Producer.Idempotent = defaultOpts.EnableIdempotence
	config.Producer.MaxMessageBytes = defaultOpts.MaxMessageBytes

	if defaultOpts.IncludeFlushConfigs {
		config.Producer.Flush.Frequency = 100 * time.Millisecond
		config.Producer.Flush.Messages = 500
		config.Producer.Flush.Bytes = 1048576 // 1MB
	}

	return config
}

func SafeClose(p Producer) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic in producer: %v\n", r)
			_ = p.Close()
		}
	}()
	_ = p.Close()
}

// ++++++++++++++++++    AWS PRODUCER   +++++++++++++++++++++

type AWSProducer struct {
	BaseProducer
	config AWSConfig
}

func NewAWSProducer(cfg *AWSConfig, opts *BaseProducerOptions) (*AWSProducer, error) {
	if cfg == nil {
		return nil, server.NewError(server.ErrorInternal, "aws config is required", nil)
	}

	acfg := NewAWSProducerConfig(cfg.Username, cfg.Password, opts)

	brokers := getBrokerAddresses(cfg.BrokerSasl)
	prod, err := sarama.NewSyncProducer(brokers, acfg)
	if err != nil {
		return nil, server.NewError(server.ErrorInternal, "failed to create aws producer", err)
	}

	return &AWSProducer{
		BaseProducer: BaseProducer{
			producer: prod,
			topic:    cfg.Topic,
		},
		config: *cfg,
	}, nil
}

func NewAWSProducerConfig(username string, password string, opts *BaseProducerOptions) *sarama.Config {
	config := BaseProducerConfig(opts)
	config.Net.SASL.User = username
	config.Net.SASL.Password = password
	return config
}

// ++++++++++++++++++    LOCAL PRODUCER   +++++++++++++++++++++

type LocalProducer struct {
	BaseProducer
	config LocalConfig
}

func NewLocalProducer(config *LocalConfig, opts *BaseProducerOptions) (*LocalProducer, error) {
	if config == nil {
		return nil, server.NewError(
			server.ErrorInternal,
			"local config is required",
			nil,
		)
	}

	localConfig := BaseProducerConfig(opts)
	brokers := getBrokerAddresses(config.BrokerSasl)

	producer, err := sarama.NewSyncProducer(brokers, localConfig)
	if err != nil {
		return nil, server.NewError(
			server.ErrorInternal,
			"failed to create local producer",
			err,
		)
	}

	return &LocalProducer{
		BaseProducer: BaseProducer{
			producer: producer,
			topic:    config.Topic,
		},
		config: *config,
	}, nil
}
