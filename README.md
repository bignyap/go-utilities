# Go Utilities

A modular collection of reusable Go components to simplify backend development. It includes robust tools for structured logging, resilient HTTP clients, Kafka integration, database utilities, and server setup—engineered for scalability and maintainability.

---

## 🚀 Features

### 1. **Logger**
- Structured logging with pluggable backends (e.g., `zerolog`).
- Context-aware logging with trace IDs and component tagging.
- Mockable for unit testing.
- 📘 [Logger Documentation](logger/README.md)

### 2. **HTTP Client**
- Built-in circuit breaker and retry mechanism via `heimdall`.
- Supports GET, POST, PUT, and DELETE requests.
- Configurable timeouts, retries, and backoff strategies.
- 📘 [HTTPClient Documentation](httpclient/README.md)

### 3. **Kafka**
- Easy setup for Kafka producers and consumers.
- Compatible with local and AWS MSK environments.
- Configurable compression, batching, retries, and message encoding.
- 📘 [Kafka Documentation](kafka/README.md)

### 4. **Database**
- Connection pooling and transaction support.
- SQL query pagination helpers.
- Common error handling utilities.
- 📘 [Database Documentation](database/README.md)

### 5. **Server**
- Modular HTTP server setup with customizable middleware.
- Graceful shutdown support.
- Built-in middleware for CORS, panic recovery, and request size limits.
- 📘 [Server Documentation](server/README.md)

### 6. **OpenTelemetry**
- Distributed tracing and metrics collection.
- Support for console, OTLP, and Elastic APM exporters.
- Automatic HTTP instrumentation via Gin middleware.
- Context propagation for distributed systems.
- 📘 [OpenTelemetry Documentation](otel/README.md)

---

## 📁 Project Structure

```
go-utilities/
├── database/     # DB connection pooling, transactions, pagination
├── httpclient/   # HTTP client with circuit breaker & retries
├── kafka/        # Kafka producer and consumer implementations
├── logger/       # Structured logging utilities
├── otel/         # OpenTelemetry tracing and metrics
├── server/       # HTTP server setup and middleware
├── go.mod        # Module dependencies
├── LICENSE       # License information
└── README.md     # Project documentation
```

---

## 📦 Installation

Add the module to your project using:

```bash
go get github.com/bignyap/go-utilities
```

---

## 🛠️ Usage Examples

### Logger

```go
import (
    "github.com/bignyap/go-utilities/logger/factory"
    "github.com/bignyap/go-utilities/logger/config"
)

func main() {
    logger := factory.GetGlobalLogger()
    logger.Info("Application started")
}
```

### HTTP Client

```go
import (
    "github.com/bignyap/go-utilities/httpclient"
)

func main() {
    client := httpclient.NewHystixClient("https://api.example.com", httpclient.DefaultConfig(), nil)
    var response map[string]interface{}
    err := client.Get("/endpoint", nil, &response)
    if err != nil {
        panic(err)
    }
}
```

### Kafka Producer

```go
import (
    "github.com/bignyap/go-utilities/kafka"
)

func main() {
    config := &kafka.LocalConfig{
        BrokerSasl: "localhost:9092",
        Topic:      "example-topic",
    }
    producer, _ := kafka.NewLocalProducer(config, nil)
    defer producer.Close()

    producer.SendMessage(map[string]string{"key": "value"})
}
```

### Database

```go
import (
    "log"
    "github.com/bignyap/go-utilities/database"
)

func main() {
    connStr := database.NewConnectionString(
        "localhost", "5432", "user", "password", "dbname", nil,
    )

    db, err := database.NewDatabase(&database.DatabaseConfig{
        Name:             "main-db",            // Optional: useful for logging or multi-DB setups
        Driver:           "postgres",           // Supports: "postgres", "mysql", "sqlite"
        ConnectionString: connStr,
        // ConnectionPoolConfig: nil,           // Optional: uses default pool settings
    })
    if err != nil {
        log.Fatalf("failed to create database: %v", err)
    }

    if err := db.Connection.Connect(); err != nil {
        log.Fatalf("failed to connect: %v", err)
    }
    defer db.Connection.Close()

    log.Println("Database connection established")
}
```

---

## 📝 License

This project is licensed under the MIT License.

---

## 🤝 Contributing

We welcome contributions! To contribute:

1. Fork the repository.
2. Create a feature or fix branch.
3. Submit a pull request with a detailed description.

---

## 📬 Contact

For questions or support, please reach out to the repository owner or open an issue.
