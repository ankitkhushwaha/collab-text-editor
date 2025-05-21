package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // Allow all origins
}

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

var (
	clients   = make(map[*Client]bool)
	clientsMu sync.Mutex
)

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}

	client := &Client{conn: conn, send: make(chan []byte, 256)}
	clientsMu.Lock()
	clients[client] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, client)
		clientsMu.Unlock()
		conn.Close()
	}()

	// Broadcast messages to all clients
	go client.writePump()
	client.readPump()
}

func (c *Client) readPump() {
	defer c.conn.Close()
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		broadcast(msg)
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}

func broadcast(msg []byte) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for client := range clients {
		select {
		case client.send <- msg:
		default:
			close(client.send)
			delete(clients, client)
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	http.Handle("/", http.FileServer(http.Dir("./client")))
	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
