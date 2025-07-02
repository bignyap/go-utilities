package pubsub

import "context"

type noopPubSub struct{}

func (n *noopPubSub) Publish(ctx context.Context, channel string, message interface{}) error {
	return nil
}

func (n *noopPubSub) Subscribe(ctx context.Context, channel string, handler MessageHandler) error {
	return nil
}

func (n *noopPubSub) Close() error {
	return nil
}
