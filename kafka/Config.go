package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/caarlos0/env"
)

// Example of broker configuration structure
// {
// 	"provider": "aws",
// 	"config": {
// 	  "broker_sasl": "kafka-broker:9093",
// 	  "username": "kafka-user",
// 	  "password": "secret",
// 	  "topic": "my-topic",
// 	  "group_id": "my-group"
// 	},
// 	"options": {
// 	  "producer": {
// 		"flush_interval_ms": 200,
// 		"required_acks": 1,
// 		"compression": "snappy"
// 	  },
// 	  "consumer": {
// 		"max_wait_ms": 500,
// 		"initial_offset": "oldest",
// 		"session_timeout_ms": 10000,
// 		"heartbeat_interval_ms": 3000,
// 		"rebalance_timeout_ms": 60000,
// 		"rebalance_retry_max": 4,
// 		"rebalance_retry_backoff_ms": 2000
// 	  }
// 	}
//   }

type BrokerConfig struct {
	Provider string               `json:"provider"`
	Config   BrokerProviderConfig `json:"config"`
	Options  *BrokerOptions       `json:"options,omitempty"`
}

type rawBrokerConfig struct {
	Provider string          `json:"provider"`
	Config   json.RawMessage `json:"config"`
	Options  *BrokerOptions  `json:"options,omitempty"`
}

func (b *BrokerConfig) UnmarshalJSON(data []byte) error {
	var raw rawBrokerConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal broker config wrapper: %w", err)
	}

	b.Provider = raw.Provider
	b.Options = raw.Options
	switch raw.Provider {
	case "aws":
		var cfg AWSConfig
		if err := json.Unmarshal(raw.Config, &cfg); err != nil {
			return fmt.Errorf("failed to unmarshal aws config: %w", err)
		}
		b.Config = &cfg
	case "local":
		var cfg LocalConfig
		if err := json.Unmarshal(raw.Config, &cfg); err != nil {
			return fmt.Errorf("failed to unmarshal local config: %w", err)
		}
		b.Config = &cfg
	default:
		return fmt.Errorf("unsupported broker provider: %s", raw.Provider)
	}

	return nil
}

type BrokerProviderConfig interface {
	GetType() string
	GetBrokerSasl() string
	GetTopic() string
}

type BrokerOptions struct {
	Producer *BaseProducerOptions `json:"producer,omitempty"`
	Consumer *BaseConsumerOptions `json:"consumer,omitempty"`
}

type AWSConfig struct {
	BrokerSasl string `json:"broker_sasl" env:"AWS_BROKER_SASL"`
	Username   string `json:"username" env:"AWS_USERNAME"`
	Password   string `json:"password" env:"AWS_PASSWORD"`
	Topic      string `json:"topic" env:"AWS_TOPIC"`
	GroupID    string `json:"group_id"`
}

func (c AWSConfig) GetType() string       { return "aws" }
func (c AWSConfig) GetBrokerSasl() string { return c.BrokerSasl }
func (c AWSConfig) GetTopic() string      { return c.Topic }

type LocalConfig struct {
	BrokerSasl string `json:"broker_sasl"`
	Topic      string `json:"topic"`
	GroupID    string `json:"group_id"`
}

func (c LocalConfig) GetType() string       { return "local" }
func (c LocalConfig) GetBrokerSasl() string { return c.BrokerSasl }
func (c LocalConfig) GetTopic() string      { return c.Topic }

func NewBrokerProviderConfig(provider string) (BrokerProviderConfig, error) {
	switch provider {
	case "aws":
		cfg := AWSConfig{}
		if err := env.Parse(&cfg); err != nil {
			return nil, fmt.Errorf("failed to load AWS producer config: %w", err)
		}
		return &cfg, nil
	case "local":
		cfg := LocalConfig{}
		if err := env.Parse(&cfg); err != nil {
			return nil, fmt.Errorf("failed to load Local producer config: %w", err)
		}
		return &cfg, nil
	default:
		return nil, fmt.Errorf("unsupported broker provider: %s", provider)
	}
}
