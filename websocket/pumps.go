package websocket

import (
	"time"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/gorilla/websocket"
)

// ReadPump pumps messages from the WebSocket connection to the message handler
// This should be run in a goroutine
func (c *Client) ReadPump() {
	defer func() {
		if c.disconnectHandler != nil {
			c.disconnectHandler(c)
		}
		if c.hub != nil {
			c.hub.Unregister(c)
		}
		c.conn.Close()
	}()

	c.conn.SetReadLimit(c.config.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("WebSocket read error", err,
					api.String("client_id", c.ID),
					api.String("user_id", c.UserID),
				)
			}
			break
		}

		if c.messageHandler != nil {
			c.messageHandler(c, message)
		}
	}
}

// WritePump pumps messages from the send channel to the WebSocket connection
// This should be run in a goroutine
func (c *Client) WritePump() {
	ticker := time.NewTicker(c.config.PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if !ok {
				// Channel was closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send each message as a separate WebSocket frame
			// This ensures each JSON message is received individually by the client
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Send any queued messages as separate frames
			n := len(c.send)
			for i := 0; i < n; i++ {
				if err := c.conn.WriteMessage(websocket.TextMessage, <-c.send); err != nil {
					return
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Start begins the read and write pumps for this client
// This should be called after the client is registered with the hub
func (c *Client) Start() {
	go c.WritePump()
	go c.ReadPump()
}
