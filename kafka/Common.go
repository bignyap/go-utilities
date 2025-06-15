package kafka

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IBM/sarama"
	"github.com/bignyap/go-utilities/server"
)

type TopicQueue struct {
	Producer sarama.SyncProducer
	Topic    string
}

func (tq *TopicQueue) GenerateKafkaMessage(payload interface{}) (*sarama.ProducerMessage, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, server.NewError(server.ErrorInternal, "failed to marshal message", err)
	}
	return &sarama.ProducerMessage{
		Topic: tq.Topic,
		Value: sarama.ByteEncoder(data),
	}, nil
}

func (tq *TopicQueue) SendMessage(payload interface{}) error {
	msg, err := tq.GenerateKafkaMessage(payload)
	if err != nil {
		return server.NewError(server.ErrorInternal, "failed to generate Kafka message", err)
	}
	_, _, err = tq.Producer.SendMessage(msg)
	if err != nil {
		return server.NewError(server.ErrorInternal, "failed to send message", err)
	}
	return nil
}

func getBrokerAddresses(brokerSasl string) []string {
	return strings.Split(brokerSasl, ",")
}

type consumerGroupHandler struct {
	handler HandlerFunc
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *consumerGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		err := h.handler(msg)
		if err != nil {
			fmt.Printf("Handler error: %v\n", err)
		}
		sess.MarkMessage(msg, "")
	}
	return nil
}
