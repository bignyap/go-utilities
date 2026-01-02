package websocket

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// OriginChecker is a function that checks if the origin is allowed
type OriginChecker func(r *http.Request) bool

// AllowAllOrigins returns an origin checker that allows all origins
// WARNING: Only use this in development
func AllowAllOrigins() OriginChecker {
	return func(r *http.Request) bool {
		return true
	}
}

// AllowOrigins returns an origin checker that allows specific origins
func AllowOrigins(origins ...string) OriginChecker {
	originSet := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		originSet[origin] = struct{}{}
	}

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		_, ok := originSet[origin]
		return ok
	}
}

// NewUpgrader creates a new WebSocket upgrader
func NewUpgrader(config Config, checkOrigin OriginChecker) *websocket.Upgrader {
	if checkOrigin == nil {
		checkOrigin = AllowAllOrigins()
	}

	return &websocket.Upgrader{
		ReadBufferSize:  config.ReadBufferSize,
		WriteBufferSize: config.WriteBufferSize,
		CheckOrigin:     checkOrigin,
	}
}

// Upgrade upgrades an HTTP connection to a WebSocket connection
func Upgrade(w http.ResponseWriter, r *http.Request, config Config, checkOrigin OriginChecker) (*websocket.Conn, error) {
	upgrader := NewUpgrader(config, checkOrigin)
	return upgrader.Upgrade(w, r, nil)
}

