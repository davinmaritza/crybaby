package main

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow cross-origin for local dashboard access
	},
}

// Hub maintains the set of active WebSocket dashboard clients and broadcasts telemetry.
type Hub struct {
	mu         sync.Mutex
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

// NewHub initializes an event hub for real-time telemetry distribution.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

// Run begins the event loop handling client registration and broadcasting.
func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				_ = conn.Close()
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			for conn := range h.clients {
				err := conn.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					_ = conn.Close()
					delete(h.clients, conn)
				}
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastJSON serializes any payload and dispatches it to all connected dashboard browsers.
func (h *Hub) BroadcastJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err == nil {
		select {
		case h.broadcast <- data:
		default:
			// Non-blocking drop if channel is saturated
		}
	}
}
