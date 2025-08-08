// Package store manages service state and SSE broadcasting for Beacon.
// Main types: Store, ServiceState, SSEEvent.
package store

import (
	"fmt"
	"sync"
	"time"
)

// ServiceStatus represents the current status of a service
type ServiceStatus string

const (
	StatusUnknown  ServiceStatus = "unknown"
	StatusUp       ServiceStatus = "up"
	StatusDown     ServiceStatus = "down"
	StatusDegraded ServiceStatus = "degraded"
)

// ServiceState represents the current state of a service
type ServiceState struct {
	ID             string        `json:"id"`
	Status         ServiceStatus `json:"status"`
	LastChecked    time.Time     `json:"lastChecked"`
	ResponseTimeMs int64         `json:"responseTimeMs"`
	HTTPStatus     int           `json:"httpStatus"`
	Error          string        `json:"error,omitempty"`
}

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Store manages the application state and SSE broadcasting
type Store struct {
	mu         sync.RWMutex
	services   map[string]*ServiceState
	clients    map[string]chan SSEEvent
	clientsMu  sync.RWMutex
	nextClient int64
}

// NewStore creates a new store instance
func NewStore() *Store {
	return &Store{
		services: make(map[string]*ServiceState),
		clients:  make(map[string]chan SSEEvent),
	}
}

// GetServiceState returns the current state of a service
func (s *Store) GetServiceState(serviceID string) (*ServiceState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, exists := s.services[serviceID]
	if !exists {
		return nil, false
	}
	// Return a copy to prevent concurrent access issues
	stateCopy := *state
	return &stateCopy, true
}

// GetAllServiceStates returns all current service states
func (s *Store) GetAllServiceStates() map[string]*ServiceState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*ServiceState)
	for id, state := range s.services {
		// Return copies to prevent concurrent access issues
		stateCopy := *state
		result[id] = &stateCopy
	}
	return result
}

// UpdateServiceState updates the state of a service and broadcasts the change
func (s *Store) UpdateServiceState(state *ServiceState) {
	s.mu.Lock()
	s.services[state.ID] = state
	s.mu.Unlock()

	// Broadcast the change to all SSE clients
	s.broadcast(SSEEvent{
		Type: "service",
		Data: state,
	})
}

// InitializeService initializes a service with unknown status
func (s *Store) InitializeService(serviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.services[serviceID]; !exists {
		s.services[serviceID] = &ServiceState{
			ID:     serviceID,
			Status: StatusUnknown,
		}
	}
}

// AddSSEClient adds a new SSE client and returns the client ID and event channel
func (s *Store) AddSSEClient() (string, chan SSEEvent) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	s.nextClient++
	clientID := fmt.Sprintf("client_%d", s.nextClient)
	eventChan := make(chan SSEEvent, 10) // Buffered channel to prevent blocking
	s.clients[clientID] = eventChan

	return clientID, eventChan
}

// RemoveSSEClient removes an SSE client
func (s *Store) RemoveSSEClient(clientID string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	if eventChan, exists := s.clients[clientID]; exists {
		close(eventChan)
		delete(s.clients, clientID)
	}
}

// BroadcastSnapshot sends the current state of all services to a specific client
func (s *Store) BroadcastSnapshot(clientID string) {
	s.clientsMu.RLock()
	eventChan, exists := s.clients[clientID]
	s.clientsMu.RUnlock()

	if !exists {
		return
	}

	allStates := s.GetAllServiceStates()
	states := make([]*ServiceState, 0, len(allStates))
	for _, state := range allStates {
		states = append(states, state)
	}

	event := SSEEvent{
		Type: "snapshot",
		Data: map[string]interface{}{
			"services": states,
		},
	}

	select {
	case eventChan <- event:
	default:
		// Client channel is full, remove the client
		s.RemoveSSEClient(clientID)
	}
}

// BroadcastConfigReload broadcasts a config reload event to all clients
func (s *Store) BroadcastConfigReload() {
	s.broadcast(SSEEvent{
		Type: "config",
		Data: map[string]interface{}{
			"message":   "config reloaded",
			"timestamp": time.Now().UTC(),
		},
	})
}

// broadcast sends an event to all connected SSE clients
func (s *Store) broadcast(event SSEEvent) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for clientID, eventChan := range s.clients {
		select {
		case eventChan <- event:
		default:
			// Client channel is full, remove the client
			go s.RemoveSSEClient(clientID)
		}
	}
}

// GetClientCount returns the number of connected SSE clients
func (s *Store) GetClientCount() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}
