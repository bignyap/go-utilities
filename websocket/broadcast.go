package websocket

import (
	"context"
	"encoding/json"

	"github.com/bignyap/go-utilities/logger/api"
)

// SendToUser sends a message to all connections of a specific user
func (h *Hub) SendToUser(userID string, message []byte) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ctx := context.Background()
	count := 0
	if userClients, ok := h.clients[userID]; ok {
		for _, client := range userClients {
			if client.Send(message) {
				count++
			}
		}
		h.logger.Debug(ctx, "Message sent to user",
			api.String("user_id", userID),
			api.Int("client_count", count),
		)
	} else {
		h.logger.Debug(ctx, "No clients found for user",
			api.String("user_id", userID),
		)
	}
	return count
}

// SendToUserJSON marshals and sends a JSON message to a user
func (h *Hub) SendToUserJSON(userID string, v interface{}) (int, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	return h.SendToUser(userID, data), nil
}

// SendToGroup sends a message to all clients in a group
func (h *Hub) SendToGroup(groupID string, message []byte) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	sentClients := make(map[string]struct{})

	if groupUsers, ok := h.groups[groupID]; ok {
		for _, userClients := range groupUsers {
			for clientID, client := range userClients {
				if _, sent := sentClients[clientID]; sent {
					continue
				}
				if client.Send(message) {
					sentClients[clientID] = struct{}{}
					count++
				}
			}
		}
	}
	return count
}

// SendToGroupJSON marshals and sends a JSON message to a group
func (h *Hub) SendToGroupJSON(groupID string, v interface{}) (int, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	return h.SendToGroup(groupID, data), nil
}

// SendToGroupExcept sends a message to all clients in a group except specified user
func (h *Hub) SendToGroupExcept(groupID string, excludeUserID string, message []byte) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	if groupUsers, ok := h.groups[groupID]; ok {
		for userID, userClients := range groupUsers {
			if userID == excludeUserID {
				continue
			}
			for _, client := range userClients {
				if client.Send(message) {
					count++
				}
			}
		}
	}
	return count
}

// SendToTenant sends a message to all clients in a tenant
func (h *Hub) SendToTenant(tenantID string, message []byte) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	sentClients := make(map[string]struct{})

	for _, userClients := range h.clients {
		for clientID, client := range userClients {
			if client.TenantID != tenantID {
				continue
			}
			if _, sent := sentClients[clientID]; sent {
				continue
			}
			if client.Send(message) {
				sentClients[clientID] = struct{}{}
				count++
			}
		}
	}
	return count
}

// SendToTenantJSON marshals and sends a JSON message to a tenant
func (h *Hub) SendToTenantJSON(tenantID string, v interface{}) (int, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	return h.SendToTenant(tenantID, data), nil
}

// BroadcastAll sends a message to all connected clients
func (h *Hub) BroadcastAll(message []byte) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, userClients := range h.clients {
		for _, client := range userClients {
			if client.Send(message) {
				count++
			}
		}
	}
	return count
}

// GetClient returns a client by userID and clientID
func (h *Hub) GetClient(userID, clientID string) (*Client, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if userClients, ok := h.clients[userID]; ok {
		client, exists := userClients[clientID]
		return client, exists
	}
	return nil, false
}

// GetUserClients returns all clients for a user
func (h *Hub) GetUserClients(userID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var clients []*Client
	if userClients, ok := h.clients[userID]; ok {
		for _, client := range userClients {
			clients = append(clients, client)
		}
	}
	return clients
}

// GetGroupClients returns all clients in a group
func (h *Hub) GetGroupClients(groupID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var clients []*Client
	if groupUsers, ok := h.groups[groupID]; ok {
		for _, userClients := range groupUsers {
			for _, client := range userClients {
				clients = append(clients, client)
			}
		}
	}
	return clients
}

// GetGroupUserIDs returns all user IDs in a group
func (h *Hub) GetGroupUserIDs(groupID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var userIDs []string
	if groupUsers, ok := h.groups[groupID]; ok {
		for userID := range groupUsers {
			userIDs = append(userIDs, userID)
		}
	}
	return userIDs
}

// HasActiveConnection checks if a user has any active connections
func (h *Hub) HasActiveConnection(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if userClients, ok := h.clients[userID]; ok {
		return len(userClients) > 0
	}
	return false
}

// GetConnectedUserIDs returns all connected user IDs (optionally filtered by tenant)
func (h *Hub) GetConnectedUserIDs(tenantID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userIDSet := make(map[string]struct{})
	for userID, userClients := range h.clients {
		for _, client := range userClients {
			if tenantID == "" || client.TenantID == tenantID {
				userIDSet[userID] = struct{}{}
				break
			}
		}
	}

	userIDs := make([]string, 0, len(userIDSet))
	for userID := range userIDSet {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

// DisconnectUser closes all connections for a user
func (h *Hub) DisconnectUser(userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if userClients, ok := h.clients[userID]; ok {
		for _, client := range userClients {
			client.Close()
		}
		delete(h.clients, userID)
	}

	// Remove from all groups
	for groupID, groupUsers := range h.groups {
		delete(groupUsers, userID)
		if len(groupUsers) == 0 {
			delete(h.groups, groupID)
		}
	}

	h.logger.Info(context.Background(), "Disconnected all clients for user",
		api.String("user_id", userID),
	)
}
