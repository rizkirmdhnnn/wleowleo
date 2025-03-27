package websocket

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// Client represents a WebSocket client connection
type Client struct {
	ID      string
	Send    chan []byte
	Hub     *Hub
	mu      sync.Mutex
	closeCh chan struct{}
	closed  bool
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Channel for broadcasting messages to all clients
	broadcast chan []byte

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Logger
	log *logrus.Logger

	// Mutex for thread safety
	mu sync.Mutex
}

// NewHub creates a new Hub instance
func NewHub(log *logrus.Logger) *Hub {
	return &Hub{
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		log:        log,
	}
}

// Run starts the hub's message handling loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.log.Infof("New client connected. Total clients: %d", len(h.clients))
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.mu.Lock()
				if !client.closed {
					close(client.Send)
					client.closed = true
				}
				client.mu.Unlock()
				h.log.Infof("Client disconnected. Total clients: %d", len(h.clients))
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.Send <- message:
					// Message sent successfully
				default:
					// Failed to send message, unregister client
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// Close closes a client connection
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		c.Hub.unregister <- c
		close(c.closeCh)
		c.closed = true
	}
}
