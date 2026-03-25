package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"

	"web-analyzer/internal/application"
)

// Handler holds the shared dependencies for both HTTP endpoints.
type Handler struct {
	store   *application.JobStore
	useCase *application.AnalyzePageUseCase
	sem     chan struct{} // counting semaphore for max concurrent analyses
	rootCtx context.Context
}

// NewHandler wires the handler with its dependencies.
func NewHandler(
	store *application.JobStore,
	useCase *application.AnalyzePageUseCase,
	sem chan struct{},
	rootCtx context.Context,
) *Handler {
	return &Handler{
		store:   store,
		useCase: useCase,
		sem:     sem,
		rootCtx: rootCtx,
	}
}

// RegisterRoutes registers the two API endpoints on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /analyze", h.handleAnalyze)
	mux.HandleFunc("GET /analyze/stream", h.handleStream)
	mux.HandleFunc("GET /openapi.yaml", h.handleOpenAPI)
	mux.HandleFunc("GET /docs", h.handleDocs)
}

// handleAnalyze accepts POST /analyze, validates the URL, creates a job,
// and launches the analysis goroutine. Returns 202 with the job ID.
func (h *Handler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate URL has an http/https scheme before consuming a semaphore slot.
	if !isValidHTTPURL(req.URL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid URL: missing scheme (must be http or https)",
		})
		return
	}

	// Acquire semaphore — 429 if at capacity.
	select {
	case h.sem <- struct{}{}:
	default:
		writeJSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "too many concurrent analyses, try again shortly",
		})
		return
	}

	job := h.store.Create()
	h.store.SetRunning(job.ID)

	go func() {
		defer func() { <-h.sem }()

		// Publish routes SSE events to all active subscribers.
		// Publish also handles terminal state updates (JobDone/JobFailed) and
		// closes subscriber channels after result/error events.
		h.useCase.Execute(h.rootCtx, job.ID, req.URL, func(event application.SSEEvent) { //nolint:errcheck
			h.store.Publish(job.ID, event)
		})
	}()

	// The goroutine above calls store.Publish("result", ...) which closes subscribers.
	// We also update SetDone via the emit function below — see execute wrapper.
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": job.ID})
}

// handleStream accepts GET /analyze/stream?id=<jobID> and opens an SSE stream.
func (h *Handler) handleStream(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id parameter"})
		return
	}

	if _, ok := h.store.Get(id); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}

	setSSEHeaders(w)

	ch, unsub := h.store.Subscribe(id)
	defer unsub()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return // channel closed — terminal event already sent
			}
			if err := writeSSEEvent(w, event); err != nil {
				log.Printf("SSE write error job=%s: %v", id, err)
				return
			}
			// After a terminal event, the channel will be closed on the next
			// iteration, which causes a clean exit.
		case <-r.Context().Done():
			return // client disconnected
		}
	}
}

// isValidHTTPURL returns true when rawURL can be parsed with an http/https scheme and a host.
func isValidHTTPURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func (h *Handler) handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	spec, err := os.ReadFile("docs/openapi.yaml")
	if err != nil {
		http.Error(w, "openapi spec not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(spec)
}

func (h *Handler) handleDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(swaggerUIHTML))
}

const swaggerUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Web Analyzer API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
      });
    };
  </script>
</body>
</html>
`
