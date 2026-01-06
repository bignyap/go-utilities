package pubsub

type Config struct {
	Type            string
	Enabled         bool
	Namespace       string
	EnableTelemetry bool // Enable OpenTelemetry tracing for pubsub operations
	Redis           *RedisConfig
	Kafka           *KafkaConfig
	RabbitMQ        *RabbitMQConfig
}

type RedisConfig struct {
	URL      string
	Password string
}

type KafkaConfig struct {
	Brokers []string
	GroupID string
	Topic   string
}

type RabbitMQConfig struct {
	URL       string
	QueueName string
}
