package kafka

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"github.com/bignyap/go-utilities/server"
)

// ++++++++++++++++++    BASE CONSUMER   +++++++++++++++++++++

type HandlerFunc func(msg *sarama.ConsumerMessage) error

type Consumer interface {
	Start(context.Context, string, HandlerFunc) error
	Close() error
}

type BaseConsumer struct {
	consumerGroup sarama.ConsumerGroup
}

func (bc *BaseConsumer) Start(ctx context.Context, topic string, handler HandlerFunc) error {
	cgh := &consumerGroupHandler{handler: handler}
	for {
		if err := bc.consumerGroup.Consume(ctx, []string{topic}, cgh); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func (bc *BaseConsumer) Close() error {
	return bc.consumerGroup.Close()
}

// BaseConsumerOptions allows customizing consumer behavior
type BaseConsumerOptions struct {
	ClientID              string        `json:"client_id" env:"BROKER_CLIENT_ID"`
	AutoCommitInterval    time.Duration `json:"auto_commit_interval" env:"BROKER_AUTO_COMMIT_INTERVAL"`
	MaxWaitTime           time.Duration `json:"max_wait_time" env:"BROKER_MAX_WAIT_TIME"`
	InitialOffset         int64         `json:"initial_offset" env:"BROKER_INITIAL_OFFSET"` // e.g. -2 for newest, -1 for oldest
	SessionTimeout        time.Duration `json:"session_timeout" env:"BROKER_SESSION_TIMEOUT"`
	HeartbeatInterval     time.Duration `json:"heartbeat_interval" env:"BROKER_HEARTBEAT_INTERVAL"`
	RebalanceTimeout      time.Duration `json:"rebalance_timeout" env:"BROKER_REBALANCE_TIMEOUT"`
	RebalanceRetryMax     int           `json:"rebalance_retry_max" env:"BROKER_REBALANCE_RETRY_MAX"`
	RebalanceRetryBackoff time.Duration `json:"rebalance_retry_backoff" env:"BROKER_REBALANCE_RETRY_BACKOFF"`
}

func BaseConsumerConfig(opts *BaseConsumerOptions) *sarama.Config {
	config := sarama.NewConfig()
	config.Version = sarama.V1_1_0_0
	config.Net.TLS.Enable = true
	config.Net.SASL.Enable = true
	config.Consumer.Return.Errors = true

	defaults := BaseConsumerOptions{
		ClientID:              "default-consumer",
		AutoCommitInterval:    250 * time.Millisecond,
		MaxWaitTime:           500 * time.Millisecond,
		InitialOffset:         sarama.OffsetOldest,
		SessionTimeout:        10 * time.Second,
		HeartbeatInterval:     3 * time.Second,
		RebalanceTimeout:      60 * time.Second,
		RebalanceRetryMax:     4,
		RebalanceRetryBackoff: 2 * time.Second,
	}

	if opts != nil {
		if opts.ClientID != "" {
			defaults.ClientID = opts.ClientID
		}
		if opts.AutoCommitInterval > 0 {
			defaults.AutoCommitInterval = opts.AutoCommitInterval
		}
		if opts.MaxWaitTime > 0 {
			defaults.MaxWaitTime = opts.MaxWaitTime
		}
		if opts.InitialOffset != 0 {
			defaults.InitialOffset = opts.InitialOffset
		}
		if opts.SessionTimeout > 0 {
			defaults.SessionTimeout = opts.SessionTimeout
		}
		if opts.HeartbeatInterval > 0 {
			defaults.HeartbeatInterval = opts.HeartbeatInterval
		}
		if opts.RebalanceTimeout > 0 {
			defaults.RebalanceTimeout = opts.RebalanceTimeout
		}
		if opts.RebalanceRetryMax > 0 {
			defaults.RebalanceRetryMax = opts.RebalanceRetryMax
		}
		if opts.RebalanceRetryBackoff > 0 {
			defaults.RebalanceRetryBackoff = opts.RebalanceRetryBackoff
		}
	}

	config.ClientID = defaults.ClientID
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Offsets.AutoCommit.Interval = defaults.AutoCommitInterval
	config.Consumer.MaxWaitTime = defaults.MaxWaitTime
	config.Consumer.Offsets.Initial = defaults.InitialOffset
	config.Consumer.Group.Session.Timeout = defaults.SessionTimeout
	config.Consumer.Group.Heartbeat.Interval = defaults.HeartbeatInterval
	config.Consumer.Group.Rebalance.Timeout = defaults.RebalanceTimeout
	config.Consumer.Group.Rebalance.Retry.Max = defaults.RebalanceRetryMax
	config.Consumer.Group.Rebalance.Retry.Backoff = defaults.RebalanceRetryBackoff

	return config
}

// ++++++++++++++++++    AWS CONSUMER   +++++++++++++++++++++

type AWSConsumer struct {
	BaseConsumer
	config AWSConfig
}

func NewAWSConsumerConfig(username, password string, opts *BaseConsumerOptions) *sarama.Config {
	config := BaseConsumerConfig(opts)
	config.Net.SASL.User = username
	config.Net.SASL.Password = password
	return config
}

func NewAWSConsumer(cfg *AWSConfig, opts *BaseConsumerOptions) (*AWSConsumer, error) {
	if cfg == nil {
		return nil, server.NewError(server.ErrorInternal, "aws config is required", nil)
	}
	groupID := cfg.GroupID
	if groupID == "" {
		groupID = "default-group"
	}
	config := NewAWSConsumerConfig(cfg.Username, cfg.Password, opts)
	brokers := getBrokerAddresses(cfg.BrokerSasl)

	grp, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, server.NewError(server.ErrorInternal, "failed to create aws consumer", err)
	}

	return &AWSConsumer{
		BaseConsumer: BaseConsumer{
			consumerGroup: grp,
		},
		config: *cfg,
	}, nil
}

// ++++++++++++++++++    LOCAL CONSUMER   +++++++++++++++++++++

type LocalConsumer struct {
	BaseConsumer
	config LocalConfig
}

func NewLocalConsumer(cfg *LocalConfig, opts *BaseConsumerOptions) (*LocalConsumer, error) {
	if cfg == nil {
		return nil, server.NewError(server.ErrorInternal, "local config is required", nil)
	}
	groupID := cfg.GroupID
	if groupID == "" {
		groupID = "default-group"
	}
	config := BaseConsumerConfig(opts)
	brokers := getBrokerAddresses(cfg.BrokerSasl)

	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, server.NewError(server.ErrorInternal, "failed to create local consumer", err)
	}

	return &LocalConsumer{
		BaseConsumer: BaseConsumer{
			consumerGroup: consumerGroup,
		},
		config: *cfg,
	}, nil
}
