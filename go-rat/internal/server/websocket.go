package server

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	log.Println("Client attempting to connect:", conn.RemoteAddr())

	// Initial message should be stub authentication
	var authMsg common.Message
	// Set a read deadline for the authentication message
	conn.SetReadDeadline(time.Now().Add(10 * time.Second)) // e.g., 10 seconds for auth
	err := conn.ReadJSON(&authMsg)
	conn.SetReadDeadline(time.Time{}) // Clear the deadline after reading

	if err != nil {
		log.Printf("Error reading auth message from %s: %v. Closing connection.", conn.RemoteAddr(), err)
		conn.Close()
		return
	}

	var client *ClientInfo
	if authMsg.Type == "auth_stub" {
		if payload, ok := authMsg.Payload.(map[string]interface{}); ok {
			stubID, _ := payload["stub_id"].(string)
			osType, _ := payload["os_type"].(string)
			arch, _ := payload["arch"].(string)

			// Add client to manager
			// r.RemoteAddr is available if handleWebSocket is an http.HandlerFunc
			// However, upgrader.Upgrade gives us the conn.RemoteAddr() which is net.Addr
			client = AddClient(conn, stubID, osType, arch, conn.RemoteAddr().String())
			log.Printf("Stub authenticated: ConnID=%s, StubID=%s, OS=%s, Arch=%s, IP=%s", client.ID, stubID, osType, arch, client.IPAddress)
			
			// Configure the close handler for this connection
			ConfigureCloseHandler(conn, client.ID)

		} else {
			log.Printf("Invalid auth_stub payload from %s: %+v. Closing connection.", conn.RemoteAddr(), authMsg.Payload)
			conn.Close()
			return
		}
	} else {
		log.Printf("Unexpected first message type '%s' from %s. Expected 'auth_stub'. Closing connection.", authMsg.Type, conn.RemoteAddr())
		conn.Close()
		return
	}

	// Regular message loop after successful authentication
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			// The close handler (set by ConfigureCloseHandler) will call RemoveClient.
			// Log the read error, the close handler will manage cleanup.
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("Read error (client: %s, stub: %s): %v - connection closed unexpectedly.", client.ID, client.StubID, err)
			} else if err == io.EOF {
				log.Printf("Read error (client: %s, stub: %s): EOF - connection closed gracefully by client.", client.ID, client.StubID)
			} else {
				log.Printf("Read error (client: %s, stub: %s): %v", client.ID, client.StubID, err)
			}
			break // Exit loop, connection will be closed by Gorilla or already is.
		}
		log.Printf("Server received message (client: %s, type %d): %s", client.ID, messageType, p)

		// Future: Process other messages from stub. For now, just logging.
		// e.g., responses to commands, logs from stub, etc.
	}
	// If the loop exits, it means the connection is effectively dead or closing.
	// The close handler should ensure RemoveClient is called.
	// If it exited due to an error not caught by the close handler (e.g. write error),
	// explicitly removing here might be a fallback, but SetCloseHandler is preferred.
	log.Printf("Message loop ended for client: %s, StubID: %s, IP: %s", client.ID, client.StubID, client.IPAddress)
	// Note: RemoveClient is primarily called by the CloseHandler.
	// If the loop breaks for reasons other than connection closure detected by ReadMessage,
	// ensure the connection is closed to trigger the CloseHandler.
	// conn.Close() // This might be redundant if ReadMessage error implies connection is already closed.
}

// RegisterWebSocketRoute registers the WebSocket handler function for the /ws path.
// This function should be called by the main server setup to integrate WebSocket handling.
func RegisterWebSocketRoute() {
	http.HandleFunc("/ws", handleWebSocket)
	log.Println("WebSocket route /ws registered.")
}

// StartDedicatedWebSocketServer starts a dedicated HTTPS server exclusively for WebSocket traffic.
// Note: For a unified server, use RegisterWebSocketRoute() and let the main server handle listening.
func StartDedicatedWebSocketServer(addr string, certFile string, keyFile string) {
	http.HandleFunc("/ws", handleWebSocket) // Ensure handler is registered if this server is used independently
	log.Printf("Starting DEDICATED WebSocket server on %s (not recommended for unified setup)\n", addr)
	err := http.ListenAndServeTLS(addr, certFile, keyFile, nil)
	if err != nil {
		log.Fatal("ListenAndServeTLS error for dedicated WebSocket server: ", err)
	}
}
