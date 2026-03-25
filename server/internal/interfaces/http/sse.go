package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"web-analyzer/internal/application"
)

// writeSSEEvent encodes and flushes a single SSE frame to w.
// The frame format is:
//
//	event: <type>\n
//	data: <json>\n\n
func writeSSEEvent(w http.ResponseWriter, event application.SSEEvent) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("marshal SSE data: %w", err)
	}

	if _, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data); err != nil {
		return err
	}

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

// setSSEHeaders writes the required headers for an SSE response.
func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}
