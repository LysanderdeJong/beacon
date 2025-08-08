// Package api implements the HTTP server, REST API, and SSE endpoints for Beacon.
// Main types: Server.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/LysanderdeJong/beacon/internal/config"
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
	// API routes
	s.mux.HandleFunc("/api/config", s.handleGetConfig)
	s.mux.HandleFunc("/api/services", s.handleGetServices)
	s.mux.HandleFunc("/api/service/", s.handleGetService)
	s.mux.HandleFunc("/.well-known/health", s.handleSelfHealth)

	// SSE route
	s.mux.HandleFunc("/sse/health", s.handleSSE)

	// Static files (frontend)
	if s.staticFS != nil {
		s.mux.Handle("/", http.FileServer(s.staticFS))
	}
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.mux.ServeHTTP(w, r)
}

// handleGetConfig returns the current configuration
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.config); err != nil {
		log.Printf("Error encoding config: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleGetServices returns all service states
func (s *Server) handleGetServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	states := s.store.GetAllServiceStates()

	// Convert map to slice for JSON response
	services := make([]*store.ServiceState, 0, len(states))
	for _, state := range states {
		services = append(services, state)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"services": services,
	}); err != nil {
		log.Printf("Error encoding services: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleGetService returns a specific service state
func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serviceID := r.URL.Path[len("/api/service/"):]
	if serviceID == "" {
		http.Error(w, "Service ID required", http.StatusBadRequest)
		return
	}

	state, exists := s.store.GetServiceState(serviceID)
	if !exists {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		log.Printf("Error encoding service state: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleSelfHealth returns the health status of Beacon itself
func (s *Server) handleSelfHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientCount := s.store.GetClientCount()

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
		"clients":   clientCount,
		"uptime":    time.Since(startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(health); err != nil {
		log.Printf("Error encoding health status: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleSSE handles Server-Sent Events connections
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get flusher for streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Add client to store
	clientID, eventChan := s.store.AddSSEClient()
	defer s.store.RemoveSSEClient(clientID)

	// Send initial snapshot
	s.store.BroadcastSnapshot(clientID)

	// Set up keepalive ticker
	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return // Channel closed
			}

			if err := s.writeSSEEvent(w, event); err != nil {
				log.Printf("Error writing SSE event: %v", err)
				return
			}
			flusher.Flush()

		case <-keepalive.C:
			// Send keepalive comment
			if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			return // Client disconnected
		}
	}
}

// writeSSEEvent writes an SSE event to the response writer
func (s *Server) writeSSEEvent(w http.ResponseWriter, event store.SSEEvent) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
	return err
}

// UpdateConfig updates the server configuration
func (s *Server) UpdateConfig(cfg *config.Config) {
	s.config = cfg
	s.store.BroadcastConfigReload()
}
