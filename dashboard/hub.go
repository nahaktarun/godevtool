package dashboard

import (
	"encoding/json"
	"sync"
)

// Event is a real-time event pushed to dashboard clients via WebSocket.
type Event struct {
	Type string `json:"type"` // "log", "request", "goroutine", "memstats"
	Data any    `json:"data"`
}

// client represents a connected WebSocket client.
type client struct {
	send chan []byte
}

// Hub manages WebSocket client connections and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]bool
}

func newHub() *Hub {
	return &Hub{
		clients: make(map[*client]bool),
	}
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = true
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		close(c.send)
		delete(h.clients, c)
	}
}

// Broadcast sends an event to all connected clients.
func (h *Hub) Broadcast(evt Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// client too slow, skip
		}
	}
}

func (h *Hub) clientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
