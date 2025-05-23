package common

// Message defines a generic structure for WebSocket messages
type Message struct {
    Type    string      `json:"type"`    // e.g., "command", "response", "heartbeat"
    Payload interface{} `json:"payload"` // Can be any data relevant to the type
}

// CommandPayload is an example for command messages
type CommandPayload struct {
    Command string   `json:"command"` // e.g., "exec", "ls", "screenshot"
    Args    []string `json:"args"`
}

// ResponsePayload is an example for response messages
type ResponsePayload struct {
    Status  string `json:"status"` // "success", "error"
    Result  string `json:"result"`
    Error   string `json:"error,omitempty"`
}
