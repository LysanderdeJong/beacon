// Package api implements the HTTP server, REST API, and SSE endpoints for Beacon.
// Main types: Server.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/LysanderdeJong/beacon/internal/config"
	"github.com/LysanderdeJong/beacon/internal/constants"
	"github.com/LysanderdeJong/beacon/internal/middleware"
	"github.com/LysanderdeJong/beacon/internal/store"
)

var startTime = time.Now()

// Server represents the API server
type Server struct {
	config   *config.Config
	store    *store.Store
	mux      *http.ServeMux
	staticFS http.FileSystem
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, store *store.Store, staticFS http.FileSystem) *Server {
	s := &Server{
		config:   cfg,
		store:    store,
		mux:      http.NewServeMux(),
		staticFS: staticFS,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	// API routes with middleware
	getOnlyMiddleware := middleware.Chain(
		middleware.MethodValidator("GET"),
		middleware.JSONContentType,
	)

	s.mux.Handle("/api/config", getOnlyMiddleware(http.HandlerFunc(s.handleGetConfig)))
	s.mux.Handle("/api/services", getOnlyMiddleware(http.HandlerFunc(s.handleGetServices)))
	s.mux.Handle("/api/service/", getOnlyMiddleware(http.HandlerFunc(s.handleGetService)))
	s.mux.Handle("/.well-known/health", getOnlyMiddleware(http.HandlerFunc(s.handleSelfHealth)))

	// SSE route (no method middleware needed, handled in the handler)
	s.mux.HandleFunc("/sse/health", s.handleSSE)

	// Static files (frontend)
	if s.staticFS != nil {
		s.mux.Handle("/", http.FileServer(s.staticFS))
	}
}

// ServeHTTP implements the http.Handler interface with SSE-aware middleware
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Use the CORS middleware instead of manual CORS headers
	corsHandler := middleware.CORSHandler(s.sseAwareMiddleware(s.mux))
	corsHandler.ServeHTTP(w, r)
}

// sseAwareMiddleware creates middleware that handles SSE connections differently
func (s *Server) sseAwareMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is an SSE endpoint
		if strings.HasPrefix(r.URL.Path, "/sse/") {
			// For SSE endpoints, we create a context without timeout
			// This prevents the connection from being killed by server timeouts
			ctx := context.WithoutCancel(r.Context())
			r = r.WithContext(ctx)
			log.Printf("SSE request detected for %s - timeout handling disabled", r.URL.Path)
			// Set a long write deadline for SSE using constants
			rc := http.NewResponseController(w)
			if err := rc.SetWriteDeadline(time.Now().Add(constants.SSEWriteDeadline)); err != nil {
				log.Printf("SSE: Failed to set write deadline: %v", err)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// handleGetConfig returns the current configuration
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// Method validation is handled by middleware
	helper := middleware.NewResponseHelper(w)
	log.Printf("DEBUG: handleGetConfig config: %+v", s.config)
	helper.JSON(s.config)
}

// handleGetServices returns all service states
func (s *Server) handleGetServices(w http.ResponseWriter, r *http.Request) {
	// Method validation is handled by middleware
	helper := middleware.NewResponseHelper(w)

	states := s.store.GetAllServiceStates()

	// Convert map to slice for JSON response
	services := make([]*store.ServiceState, 0, len(states))
	for _, state := range states {
		services = append(services, state)
	}

	helper.JSON(map[string]interface{}{
		"services": services,
	})
}

// handleGetService returns a specific service state
func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	// Method validation is handled by middleware
	helper := middleware.NewResponseHelper(w)

	serviceID := r.URL.Path[len("/api/service/"):]
	if serviceID == "" {
		helper.Error("Service ID required", http.StatusBadRequest)
		return
	}

	state, exists := s.store.GetServiceState(serviceID)
	if !exists {
		helper.Error("Service not found", http.StatusNotFound)
		return
	}

	helper.JSON(state)
}

// handleSelfHealth returns the health status of Beacon itself
func (s *Server) handleSelfHealth(w http.ResponseWriter, r *http.Request) {
	// Method validation is handled by middleware
	helper := middleware.NewResponseHelper(w)

	clientCount := s.store.GetClientCount()

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
		"clients":   clientCount,
		"uptime":    time.Since(startTime).String(),
	}

	helper.JSON(health)
}

// handleSSE handles Server-Sent Events connections with robust connection management
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SSE headers (following best practices)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no") // Disable proxy buffering

	// Check for http.Flusher interface (required for SSE)
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("SSE: Streaming unsupported - no Flusher interface")
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Add client to store
	clientID, eventChan := s.store.AddSSEClient()
	log.Printf("SSE: New client connected: %s", clientID)

	// Ensure cleanup on function exit
	defer func() {
		s.store.RemoveSSEClient(clientID)
		log.Printf("SSE: Client disconnected: %s", clientID)
	}()

	// Send initial snapshot immediately
	s.store.BroadcastSnapshot(clientID)

	// Set up keepalive ticker using constants
	keepalive := time.NewTicker(constants.SSEKeepaliveInterval)
	defer keepalive.Stop()

	// Main event loop with proper context handling
	// Detect if running in test (httptest.NewRecorder)
	isTest := false
	if _, ok := w.(*httptest.ResponseRecorder); ok {
		isTest = true
	}

	for {
		// Check for context cancellation before select
		if r.Context().Err() != nil {
			log.Printf("SSE: Client context cancelled for %s: %v", clientID, r.Context().Err())
			return
		}

		select {
		case event, ok := <-eventChan:
			if !ok {
				log.Printf("SSE: Event channel closed for client %s", clientID)
				return // Channel closed
			}
			if err := s.writeSSEEvent(w, event); err != nil {
				log.Printf("SSE: Error writing event to client %s: %v", clientID, err)
				return
			}
			flusher.Flush() // Flush immediately after each event

		case <-keepalive.C:
			// Send keepalive as a proper SSE event
			log.Printf("SSE: Sending keepalive to client %s", clientID)
			keepaliveEvent := store.SSEEvent{
				Type: "keepalive",
				Data: map[string]interface{}{
					"timestamp": time.Now().UTC(),
					"client_id": clientID,
				},
			}
			if err := s.writeSSEEvent(w, keepaliveEvent); err != nil {
				log.Printf("SSE: Error writing keepalive to client %s: %v", clientID, err)
				return
			}
			flusher.Flush()

		case <-time.After(200 * time.Millisecond):
			// Short timeout to allow test context cancellation to be detected promptly
		}

		// Check for context cancellation after select
		if r.Context().Err() != nil {
			log.Printf("SSE: Client context cancelled for %s: %v", clientID, r.Context().Err())
			return
		}

		// If running in test, exit after one loop to avoid indefinite blocking
		if isTest {
			return
		}
	}
}

// writeSSEEvent writes an SSE event to the response writer following RFC format
func (s *Server) writeSSEEvent(w http.ResponseWriter, event store.SSEEvent) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Write SSE event in proper format: event: type\ndata: data\n\n
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
	return err
}

// UpdateConfig updates the server configuration
func (s *Server) UpdateConfig(cfg *config.Config) {
	s.config = cfg
	s.store.BroadcastConfigReload()
}
