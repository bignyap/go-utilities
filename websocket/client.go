package websocket

import (
	"encoding/json"
	"sync"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/gorilla/websocket"
)

// MessageHandler is a callback for handling incoming messages
type MessageHandler func(client *Client, message []byte)

// DisconnectHandler is a callback for handling client disconnection
type DisconnectHandler func(client *Client)

// Client represents a WebSocket client connection
type Client struct {
	// ID is the unique identifier for this client connection
	ID string
	// UserID is the user identifier
	UserID string
	// TenantID is the tenant identifier
	TenantID string
	// Token stores JWT token for authenticated API calls
	Token string
	// Metadata stores custom key-value data for application use
	Metadata map[string]interface{}

	conn     *websocket.Conn
	send     chan []byte
	hub      HubInterface
	logger   api.Logger
	config   Config
	isClosed bool
	mu       sync.Mutex

	// Handlers
	messageHandler    MessageHandler
	disconnectHandler DisconnectHandler
}

// ClientOption is a functional option for configuring a Client
type ClientOption func(*Client)

// WithMessageHandler sets the message handler for the client
func WithMessageHandler(handler MessageHandler) ClientOption {
	return func(c *Client) {
		c.messageHandler = handler
	}
}

// WithDisconnectHandler sets the disconnect handler for the client
func WithDisconnectHandler(handler DisconnectHandler) ClientOption {
	return func(c *Client) {
		c.disconnectHandler = handler
	}
}

// WithToken sets the JWT token for the client
func WithToken(token string) ClientOption {
	return func(c *Client) {
		c.Token = token
	}
}

// WithMetadata sets custom metadata for the client
func WithMetadata(key string, value interface{}) ClientOption {
	return func(c *Client) {
		if c.Metadata == nil {
			c.Metadata = make(map[string]interface{})
		}
		c.Metadata[key] = value
	}
}

// NewClient creates a new WebSocket client
func NewClient(
	id string,
	userID string,
	tenantID string,
	conn *websocket.Conn,
	hub HubInterface,
	logger api.Logger,
	config Config,
	opts ...ClientOption,
) *Client {
	c := &Client{
		ID:       id,
		UserID:   userID,
		TenantID: tenantID,
		conn:     conn,
		send:     make(chan []byte, config.SendBufferSize),
		hub:      hub,
		logger:   logger.WithComponent("ws-client"),
		config:   config,
		Metadata: make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Send sends a message to the client (non-blocking)
func (c *Client) Send(message []byte) bool {
	c.mu.Lock()
	if c.isClosed {
		c.mu.Unlock()
		return false
	}
	c.mu.Unlock()

	select {
	case c.send <- message:
		return true
	default:
		c.logger.Warn("Client send buffer full",
			api.String("client_id", c.ID),
			api.String("user_id", c.UserID),
		)
		return false
	}
}

// SendJSON marshals and sends a JSON message to the client
func (c *Client) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.Send(data)
	return nil
}

// GetMetadata retrieves metadata value by key
func (c *Client) GetMetadata(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	val, ok := c.Metadata[key]
	return val, ok
}

// SetMetadata sets a metadata value
func (c *Client) SetMetadata(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Metadata[key] = value
}

// Close closes the client connection
func (c *Client) Close() {
	c.mu.Lock()
	if c.isClosed {
		c.mu.Unlock()
		return
	}
	c.isClosed = true
	close(c.send)
	c.mu.Unlock()

	c.conn.Close()
}
