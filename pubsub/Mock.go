package pubsub

import "context"

type noopPubSub struct{}

func (n *noopPubSub) Publish(ctx context.Context, channel string, message interface{}) error {
	return nil
}

func (n *noopPubSub) Subscribe(ctx context.Context, channel string, handler MessageHandler) error {
	return nil
}

func (n *noopPubSub) PublishStream(ctx context.Context, stream string, message interface{}, opts ...StreamOption) (string, error) {
	return "0-0", nil
}

func (n *noopPubSub) PublishStreamMap(ctx context.Context, stream string, values map[string]interface{}, opts ...StreamOption) (string, error) {
	return "0-0", nil
}

func (n *noopPubSub) ConsumeStream(ctx context.Context, stream, group, consumer string, handler StreamMessageHandler, opts ...ConsumerOption) error {
	return nil
}

func (n *noopPubSub) Close() error {
	return nil
}

// NewNoopStreamPublisher returns a no-op StreamPublisher that discards all messages.
func NewNoopStreamPublisher() StreamPublisher {
	return &noopPubSub{}
}
