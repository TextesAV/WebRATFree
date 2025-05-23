package main

import (
	"go-rat/internal/server"
	"log"
	"net/http"
)

func main() {
	log.Println("Starting server application...")

	webServerAddr := "localhost:8443" // Unified server address
	// Paths are relative to the project root (go-rat) when running:
	// `go run cmd/server/main.go` from the `go-rat` directory.
	// If you `cd cmd/server` and run `go run main.go`, these paths would be `../../certs/cert.pem`.
	certPath := "certs/cert.pem"
	keyPath := "certs/key.pem"

	log.Println("Configuring web application routes (login, dashboard, etc.)...")
	// server.StartWebServer (as modified in a previous step) registers handlers like /login, /dashboard.
	// It does NOT start the HTTP server itself.
	server.StartWebServer(webServerAddr, certPath, keyPath)

	log.Println("Loading HTML templates...")
	// Path is relative to project root (go-rat)
	err := server.LoadTemplates("web/templates")
	if err != nil {
		log.Fatalf("Error loading templates: %v", err)
	}

	log.Println("Configuring WebSocket route (/ws)...")
	// The following function, `server.RegisterWebSocketRoute()`, will be created
	// in `go-rat/internal/server/websocket.go` in the next step.
	// It will contain `http.HandleFunc("/ws", handleWebSocketFromPackage) // or similar
	// For now, this call is commented out to ensure `main.go` is valid Go code
	// server.StartWebServer (as modified in a previous step) registers handlers like /login, /dashboard.
	// It does NOT start the HTTP server itself.
	server.StartWebServer(webServerAddr, certPath, keyPath)

	log.Println("Configuring WebSocket route (/ws)...")
	// This function was created in the previous step by refactoring `websocket.go`.
	// It registers the WebSocket handler with `http.DefaultServeMux`.
	server.RegisterWebSocketRoute()

	log.Printf("Starting main HTTPS server for web, API, and WebSocket on %s...", webServerAddr)
	// This server will handle all registered routes (web pages, API, and eventually /ws).
	// It uses http.DefaultServeMux by default, where StartWebServer and (soon) RegisterWebSocketRoute
	// will register their handlers.
	err := http.ListenAndServeTLS(webServerAddr, certPath, keyPath, nil)
	if err != nil {
		log.Fatalf("Failed to start main HTTPS server on %s: %v", webServerAddr, err)
	}
}
