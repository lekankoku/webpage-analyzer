package application

// SSEEvent is the unit emitted by the application layer and forwarded to HTTP clients.
// Type is one of: "phase", "progress", "result", "error".
// Data is a JSON-serialisable payload whose shape depends on Type.
type SSEEvent struct {
	Type string
	Data any
}
