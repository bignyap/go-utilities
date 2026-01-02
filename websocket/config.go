package websocket

import "time"

// Config holds WebSocket configuration
type Config struct {
	// WriteWait is the time allowed to write a message to the peer
	WriteWait time.Duration
	// PongWait is the time allowed to read the next pong message from the peer
	PongWait time.Duration
	// PingPeriod is the period to send pings to the peer (must be less than PongWait)
	PingPeriod time.Duration
	// MaxMessageSize is the maximum message size allowed from peer
	MaxMessageSize int64
	// SendBufferSize is the size of the send channel buffer
	SendBufferSize int
	// ReadBufferSize is the WebSocket read buffer size
	ReadBufferSize int
	// WriteBufferSize is the WebSocket write buffer size
	WriteBufferSize int
}

// DefaultConfig returns default WebSocket configuration
func DefaultConfig() Config {
	return Config{
		WriteWait:       10 * time.Second,
		PongWait:        60 * time.Second,
		PingPeriod:      54 * time.Second, // (60 * 9) / 10
		MaxMessageSize:  512 * 1024,       // 512 KB
		SendBufferSize:  256,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
}

