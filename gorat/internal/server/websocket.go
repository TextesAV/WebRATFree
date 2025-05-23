package server

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	// "gorat/pkg/common" // ClientManager will handle common.Message
)

// Upgrader is exported to be used by AdminWebSocketHandler as well
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all connections by default
		return true
	},
}

// HandleWebSocket connections for stubs
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade stub connection:", err)
		return
	}
	// defer conn.Close() // ClientManager will handle closing the connection

	clientInfo := ClientMgr.RegisterClient(conn)
	log.Printf("Stub client %s connected from %s", clientInfo.ID, clientInfo.RemoteAddr)

	// Each client connection will be handled in its own goroutine by the ClientManager
	ClientMgr.HandleClientMessages(clientInfo)
}
