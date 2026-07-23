package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"cyrbaby/pkg/protocol"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for RMM agents
	},
}

// Client represents a connected agent.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	uuid     string
	token    string
	approved bool
	mu       sync.Mutex
}

// Hub maintains the set of active clients.
type Hub struct {
	clients    map[string]*Client // Key: UUID
	clientsMu  sync.RWMutex
	register   chan *Client
	unregister chan *Client
	db         *DB

	// Pending command responses
	pendingCmds   map[string]chan *protocol.CommandResponse
	pendingCmdsMu sync.Mutex

	// Active file transfers
	transfers   map[string]chan *protocol.FileGetChunk
	transfersMu sync.Mutex
}

func NewHub(db *DB) *Hub {
	return &Hub{
		clients:     make(map[string]*Client),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		db:          db,
		pendingCmds: make(map[string]chan *protocol.CommandResponse),
		transfers:   make(map[string]chan *protocol.FileGetChunk),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			// If there's an existing connection for this UUID, close it
			if oldClient, exists := h.clients[client.uuid]; exists {
				log.Printf("Closing duplicate connection for agent %s", client.uuid)
				oldClient.conn.Close()
				delete(h.clients, client.uuid)
			}
			h.clients[client.uuid] = client
			h.clientsMu.Unlock()
			log.Printf("Agent connected: %s (approved=%v)", client.uuid, client.approved)

		case client := <-h.unregister:
			h.clientsMu.Lock()
			if _, ok := h.clients[client.uuid]; ok {
				delete(h.clients, client.uuid)
				close(client.send)
				log.Printf("Agent disconnected: %s", client.uuid)
			}
			h.clientsMu.Unlock()
		}
	}
}

// Broadcast sends a message to all approved connected agents.
func (h *Hub) Broadcast(msg []byte) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	for _, client := range h.clients {
		if client.approved {
			select {
			case client.send <- msg:
			default:
				close(client.send)
				h.clientsMu.RUnlock()
				h.unregister <- client
				h.clientsMu.RLock()
			}
		}
	}
}

// SendCommand sends a command to a specific agent and waits for the response.
func (h *Hub) SendCommand(uuid string, command string, issuedBy string) (*protocol.CommandResponse, error) {
	h.clientsMu.RLock()
	client, exists := h.clients[uuid]
	h.clientsMu.RUnlock()

	if !exists || !client.approved {
		return nil, errors.New("agent not connected or not approved")
	}

	cmdID := fmt.Sprintf("cmd_%d", time.Now().UnixNano())
	req := protocol.CommandRequest{
		CommandID: cmdID,
		Command:   command,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	msg := protocol.Message{
		Type:    protocol.TypeCommandRequest,
		Id:      cmdID,
		Payload: payload,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	// Create completion channel
	respChan := make(chan *protocol.CommandResponse, 1)
	h.pendingCmdsMu.Lock()
	h.pendingCmds[cmdID] = respChan
	h.pendingCmdsMu.Unlock()

	defer func() {
		h.pendingCmdsMu.Lock()
		delete(h.pendingCmds, cmdID)
		h.pendingCmdsMu.Unlock()
	}()

	// Log execution start
	if err := h.db.LogCommandStart(cmdID, uuid, issuedBy, command); err != nil {
		log.Printf("Failed to log command start: %v", err)
	}

	// Send to client
	client.send <- msgBytes

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		// Log completion
		resultStr := fmt.Sprintf("Exit Code: %d\nOutput: %s", resp.ExitCode, resp.Output)
		if resp.Error != "" {
			resultStr += "\nError: " + resp.Error
		}
		if err := h.db.LogCommandComplete(cmdID, resultStr); err != nil {
			log.Printf("Failed to log command completion: %v", err)
		}
		return resp, nil
	case <-time.After(30 * time.Second):
		if err := h.db.LogCommandComplete(cmdID, "Command timed out after 30 seconds"); err != nil {
			log.Printf("Failed to log command timeout: %v", err)
		}
		return nil, errors.New("command timeout")
	}
}

// ApprovePendingAgent approves an agent connection.
func (h *Hub) ApprovePendingAgent(uuid string) error {
	token, err := h.db.ApproveServer(uuid)
	if err != nil {
		return err
	}

	h.clientsMu.RLock()
	client, exists := h.clients[uuid]
	h.clientsMu.RUnlock()

	if exists {
		client.mu.Lock()
		client.approved = true
		client.token = token
		client.mu.Unlock()

		// Send success response with token
		resp := protocol.RegisterResponse{
			Success: true,
			Status:  "approved",
			Token:   token,
			Message: "Approved by administrator",
		}
		payload, _ := json.Marshal(resp)
		msg := protocol.Message{
			Type:    protocol.TypeRegisterResponse,
			Payload: payload,
		}
		msgBytes, _ := json.Marshal(msg)
		client.send <- msgBytes
	}

	return nil
}

// RejectPendingAgent rejects a pending agent and sends uninstall signal.
func (h *Hub) RejectPendingAgent(uuid string) error {
	if err := h.db.RejectServer(uuid); err != nil {
		return err
	}

	h.clientsMu.RLock()
	client, exists := h.clients[uuid]
	h.clientsMu.RUnlock()

	if exists {
		msg := protocol.Message{
			Type: protocol.TypeUninstallSignal,
		}
		msgBytes, _ := json.Marshal(msg)
		client.send <- msgBytes
		client.conn.Close()
	}

	return nil
}

// DecommissionAgent decommissions an agent.
func (h *Hub) DecommissionAgent(uuid string) error {
	if err := h.db.DecommissionServer(uuid); err != nil {
		return err
	}

	h.clientsMu.RLock()
	client, exists := h.clients[uuid]
	h.clientsMu.RUnlock()

	if exists {
		msg := protocol.Message{
			Type: protocol.TypeUninstallSignal,
		}
		msgBytes, _ := json.Marshal(msg)
		client.send <- msgBytes
		client.conn.Close()
	}

	return nil
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(50 * 1024 * 1024) // 50MB max file chunk size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		var msg protocol.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("JSON unmarshal error for agent message: %v", err)
			continue
		}

		if err := c.handleMessage(msg); err != nil {
			log.Printf("Error handling agent message: %v", err)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg protocol.Message) error {
	switch msg.Type {
	case protocol.TypeRegisterRequest:
		var req protocol.RegisterRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			return err
		}

		c.mu.Lock()
		c.uuid = req.UUID
		c.mu.Unlock()

		status, dbToken, isNew, err := c.hub.db.RegisterOrUpdateServer(
			req.UUID, req.Hostname, req.OSVersion, req.CPUModel,
			req.CPUCores, req.CPUThreads, req.RAMTotalMB, req.DiskTotalMB, req.AgentVersion,
		)
		if err != nil {
			return err
		}

		if isNew && tgBot != nil {
			s, err := c.hub.db.GetServer(req.UUID)
			if err == nil && s != nil {
				tgBot.NotifyNewDevice(s)
			}
		}

		if status == "rejected" || status == "removed" {
			resp := protocol.RegisterResponse{Success: false, Status: status, Message: "Access denied"}
			p, _ := json.Marshal(resp)
			msgBytes, _ := json.Marshal(protocol.Message{Type: protocol.TypeRegisterResponse, Payload: p})
			c.send <- msgBytes
			time.Sleep(1 * time.Second)
			c.conn.Close()
			return nil
		}

		if status == "approved" {
			// Token validation
			if req.Token != dbToken {
				resp := protocol.RegisterResponse{Success: false, Status: "unauthorized", Message: "Invalid token"}
				p, _ := json.Marshal(resp)
				msgBytes, _ := json.Marshal(protocol.Message{Type: protocol.TypeRegisterResponse, Payload: p})
				c.send <- msgBytes
				time.Sleep(1 * time.Second)
				c.conn.Close()
				return fmt.Errorf("agent %s sent invalid token", req.UUID)
			}
			c.mu.Lock()
			c.approved = true
			c.token = dbToken
			c.mu.Unlock()
		}

		h := c.hub
		h.register <- c

		resp := protocol.RegisterResponse{
			Success: true,
			Status:  status,
			Token:   dbToken,
		}
		p, _ := json.Marshal(resp)
		msgBytes, _ := json.Marshal(protocol.Message{Type: protocol.TypeRegisterResponse, Payload: p})
		c.send <- msgBytes

	case protocol.TypeHeartbeat:
		if !c.approved {
			return errors.New("heartbeat received from unapproved client")
		}

		var hb protocol.Heartbeat
		if err := json.Unmarshal(msg.Payload, &hb); err != nil {
			return err
		}

		// Update last seen
		_ = c.hub.db.UpdateServerLastSeen(c.uuid)

		// Save metrics to DB
		if err := c.hub.db.SaveMetricSample(c.uuid, hb.CPULoadPct, hb.RAMUsedMB, hb.DiskUsedMB, hb.UptimeSeconds); err != nil {
			log.Printf("Failed to save metrics for %s: %v", c.uuid, err)
		}

	case protocol.TypeCommandResponse:
		var resp protocol.CommandResponse
		if err := json.Unmarshal(msg.Payload, &resp); err != nil {
			return err
		}

		c.hub.pendingCmdsMu.Lock()
		ch, exists := c.hub.pendingCmds[resp.CommandID]
		c.hub.pendingCmdsMu.Unlock()

		if exists {
			ch <- &resp
		}

	case protocol.TypeFileGetChunk:
		var chunk protocol.FileGetChunk
		if err := json.Unmarshal(msg.Payload, &chunk); err != nil {
			return err
		}

		c.hub.transfersMu.Lock()
		ch, exists := c.hub.transfers[chunk.TransferID]
		c.hub.transfersMu.Unlock()

		if exists {
			ch <- &chunk
		}
	}

	return nil
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket: %v", err)
		return
	}

	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		approved: false,
	}

	go client.writePump()
	go client.readPump()
}
