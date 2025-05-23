package stub

import (
	"log"
	// "go-rat/pkg/common" // Not strictly needed for placeholders if not using common.Message types directly in responses
	"github.com/gorilla/websocket"
)

// HandleRemoteDesktop logs that the function was called.
func HandleRemoteDesktop(conn *websocket.Conn, args []string) {
	log.Println("Placeholder: RemoteDesktop command received with args:", args)
	// Example: Send a response back to the server
	// response := common.Message{
	// Type: "response",
	// Payload: common.ResponsePayload{
	// Status: "success",
	// Result: "RemoteDesktop command acknowledged",
	// },
	// }
	// if conn != nil {
	// conn.WriteJSON(response)
	// }
}

// HandleSendDisk logs that the function was called.
func HandleSendDisk(conn *websocket.Conn, args []string) {
	log.Println("Placeholder: SendDisk command received with args:", args)
}

// HandleFileManager logs that the function was called.
// 'action' could be "list", "download", "upload", "delete".
// 'path' is the target path.
// 'data' could be file content for uploads.
func HandleFileManager(conn *websocket.Conn, action string, path string, data []byte) {
	log.Printf("Placeholder: FileManager command received with action: '%s', path: '%s', data_length: %d\n", action, path, len(data))
}

// HandleTaskManager logs that the function was called.
// 'action' could be "list_processes", "kill_process".
// 'processID' is the ID of the process to interact with.
func HandleTaskManager(conn *websocket.Conn, action string, processID string) {
	log.Printf("Placeholder: TaskManager command received with action: '%s', processID: '%s'\n", action, processID)
}

// HandleRemoteShell logs that the function was called.
// 'command' is the shell command to execute.
// 'args_shell' are the arguments for the command.
func HandleRemoteShell(conn *websocket.Conn, command string, args_shell []string) {
	log.Printf("Placeholder: RemoteShell command received with command: '%s', args: %v\n", command, args_shell)
}

// Note: Watchdog functionality will be handled differently, not as a direct command handler.
// It would likely involve persistent monitoring and reconnection logic, potentially
// started from the stub's main or connection management logic if enabled.
