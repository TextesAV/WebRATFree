package common

// Message defines the structure for communication between server and stub.
type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}
