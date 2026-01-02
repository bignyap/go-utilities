package websocket

import (
	"sync"

	"github.com/bignyap/go-utilities/logger/api"
)

// HubInterface defines the interface for a WebSocket hub
type HubInterface interface {
	Register(client *Client)
	Unregister(client *Client)
	Run()
}

// Hub manages WebSocket client connections
type Hub struct {
	// clients maps userID -> clientID -> Client
	clients map[string]map[string]*Client

	// groups maps groupID -> userID -> clientID -> Client
	// Used for rooms, calls, channels, etc.
	groups map[string]map[string]map[string]*Client

	// Channels for thread-safe operations
	register   chan *Client
	unregister chan *Client

	// Mutex for direct access operations
	mu sync.RWMutex

	logger api.Logger
}

// NewHub creates a new WebSocket hub
func NewHub(logger api.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]map[string]*Client),
		groups:     make(map[string]map[string]map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger.WithComponent("ws-hub"),
	}
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		}
	}
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.UserID]; !ok {
		h.clients[client.UserID] = make(map[string]*Client)
	}
	h.clients[client.UserID][client.ID] = client

	h.logger.Info("Client registered",
		api.String("client_id", client.ID),
		api.String("user_id", client.UserID),
		api.String("tenant_id", client.TenantID),
	)
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove from user clients
	if userClients, ok := h.clients[client.UserID]; ok {
		if _, exists := userClients[client.ID]; exists {
			client.Close()
			delete(userClients, client.ID)
			if len(userClients) == 0 {
				delete(h.clients, client.UserID)
			}
		}
	}

	// Remove from all groups
	for groupID, groupUsers := range h.groups {
		if userClients, ok := groupUsers[client.UserID]; ok {
			delete(userClients, client.ID)
			if len(userClients) == 0 {
				delete(groupUsers, client.UserID)
			}
			if len(groupUsers) == 0 {
				delete(h.groups, groupID)
			}
		}
	}

	h.logger.Info("Client unregistered",
		api.String("client_id", client.ID),
		api.String("user_id", client.UserID),
	)
}

// JoinGroup adds a client to a group (room, call, etc.)
func (h *Hub) JoinGroup(groupID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.groups[groupID]; !ok {
		h.groups[groupID] = make(map[string]map[string]*Client)
	}
	if _, ok := h.groups[groupID][client.UserID]; !ok {
		h.groups[groupID][client.UserID] = make(map[string]*Client)
	}
	h.groups[groupID][client.UserID][client.ID] = client

	h.logger.Debug("Client joined group",
		api.String("client_id", client.ID),
		api.String("user_id", client.UserID),
		api.String("group_id", groupID),
	)
}

// LeaveGroup removes a client from a group
func (h *Hub) LeaveGroup(groupID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if groupUsers, ok := h.groups[groupID]; ok {
		if userClients, ok := groupUsers[client.UserID]; ok {
			delete(userClients, client.ID)
			if len(userClients) == 0 {
				delete(groupUsers, client.UserID)
			}
		}
		if len(groupUsers) == 0 {
			delete(h.groups, groupID)
		}
	}

	h.logger.Debug("Client left group",
		api.String("client_id", client.ID),
		api.String("user_id", client.UserID),
		api.String("group_id", groupID),
	)
}
