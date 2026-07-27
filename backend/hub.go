package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
	sendBuf    = 16
)

type envelope struct {
	Type    string       `json:"type"`
	State   *PublicState `json:"state,omitempty"`
	Message string       `json:"message,omitempty"`
}

type clientMsg struct {
	Action string `json:"action"`
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	mu      sync.Mutex
	clients map[*Client]struct{}
	store   *Store
}

func NewHub(store *Store) *Hub {
	return &Hub{
		clients: make(map[*Client]struct{}),
		store:   store,
	}
}

func (h *Hub) add(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
	_ = c.conn.Close()
}

func (h *Hub) broadcast(state PublicState) {
	payload, err := json.Marshal(envelope{Type: "state", State: &state})
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- payload:
		default:
			// Slow client — drop connection rather than block the hub.
			delete(h.clients, c)
			close(c.send)
			_ = c.conn.Close()
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump() {
	defer c.hub.remove(c)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg clientMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		var (
			next  PublicState
			opErr error
		)
		switch msg.Action {
		case "start":
			next, opErr = c.hub.store.Start()
		case "stop":
			next, opErr = c.hub.store.Stop()
		case "advance":
			next, opErr = c.hub.store.Advance()
		case "close":
			next, opErr = c.hub.store.Close()
		default:
			continue
		}
		if opErr != nil {
			payload, _ := json.Marshal(envelope{Type: "error", Message: opErr.Error()})
			select {
			case c.send <- payload:
			default:
			}
			continue
		}
		c.hub.broadcast(next)
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, sendBuf),
	}
	h.add(client)

	state := h.store.View()
	payload, _ := json.Marshal(envelope{Type: "state", State: &state})
	client.send <- payload

	go client.writePump()
	client.readPump()
}

func (h *Hub) tickLoop(stop <-chan struct{}) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	var wasAwaiting bool
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			state := h.store.View()
			if state.Status == StatusRunning || state.AwaitingAdvance || wasAwaiting {
				h.broadcast(state)
			}
			wasAwaiting = state.AwaitingAdvance
		}
	}
}
