package server

import (
	"log"
	"net/http"
)

// StartWebServer sets up handlers and is called by main to start the server.
// Note: The actual ListenAndServeTLS will be called from main.go
func StartWebServer(webAddr string, certFile string, keyFile string) {
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/dashboard", authMiddleware(DashboardHandler))
	http.HandleFunc("/builder", authMiddleware(BuilderHandler))
	// The /ws handler is set up in websocket.go (handleWebSocket)
	// It will be registered with the same server instance in main.go

	log.Printf("Web server handlers configured. HTTPS server will listen on %s", webAddr)
	// The actual http.ListenAndServeTLS is called in cmd/server/main.go
	// to ensure all handlers (web and WebSocket) are registered to the same server.
}
