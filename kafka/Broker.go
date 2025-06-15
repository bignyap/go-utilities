package kafka

import (
	"fmt"

	"github.com/bignyap/go-utilities/server"
)

func NewProducer(cfg *BrokerConfig, opts *BaseProducerOptions) (Producer, error) {
	switch cfg.Provider {
	case "local":
		return NewLocalProducer(cfg.Config.(*LocalConfig), opts)
	case "aws":
		return NewAWSProducer(cfg.Config.(*AWSConfig), opts)
	default:
		return nil, server.NewError(
			server.ErrorInternal,
			fmt.Sprintf("unsupported broker provider: %s", cfg.Provider),
			nil,
		)
	}
}

func NewConsumer(cfg *BrokerConfig, opts *BaseConsumerOptions) (Consumer, error) {
	switch cfg.Provider {
	case "local":
		return NewLocalConsumer(cfg.Config.(*LocalConfig), opts)
	case "aws":
		return NewAWSConsumer(cfg.Config.(*AWSConfig), opts)
	default:
		return nil, server.NewError(
			server.ErrorInternal,
			fmt.Sprintf("unsupported broker provider: %s", cfg.Provider),
			nil,
		)
	}
}
