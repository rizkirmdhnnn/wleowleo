package websocket

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development, should be restricted in production
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WebSocketHandler handles WebSocket connections
func WebSocketHandler(hub *Hub, log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Errorf("Failed to upgrade connection to WebSocket: %v", err)
			return
		}

		client := &Client{
			ID:      uuid.New().String(),
			Hub:     hub,
			Send:    make(chan []byte, 256),
			closeCh: make(chan struct{}),
		}

		// Register client
		client.Hub.register <- client

		// Start goroutines for pumping messages
		go writePump(client, conn, log)
		go readPump(client, conn, log)

		log.Infof("New WebSocket connection established: %s", client.ID)
	}
}

// readPump pumps messages from the WebSocket connection to the hub
func readPump(client *Client, conn *websocket.Conn, log *logrus.Logger) {
	defer func() {
		client.Hub.unregister <- client
		conn.Close()
		log.Infof("WebSocket connection closed: %s", client.ID)
	}()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Errorf("WebSocket read error: %v", err)
			}
			break
		}
		// We're not processing incoming messages from clients for now
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func writePump(client *Client, conn *websocket.Conn, log *logrus.Logger) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Errorf("Error getting next writer: %v", err)
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(client.Send)
			for i := 0; i < n; i++ {
				w.Write(<-client.Send)
			}

			if err := w.Close(); err != nil {
				log.Errorf("Error closing writer: %v", err)
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Errorf("Error sending ping: %v", err)
				return
			}
		case <-client.closeCh:
			return
		}
	}
}
