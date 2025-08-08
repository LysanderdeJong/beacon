package store

import (
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	store := NewStore()

	if store == nil {
		t.Fatal("NewStore returned nil")
	}

	if store.services == nil {
		t.Error("Store services map is nil")
	}

	if store.clients == nil {
		t.Error("Store clients map is nil")
	}
}

func TestServiceStateOperations(t *testing.T) {
	store := NewStore()

	// Test getting non-existent service
	_, exists := store.GetServiceState("non-existent")
	if exists {
		t.Error("Expected false for non-existent service")
	}

	// Test initializing service
	store.InitializeService("test-service")

	state, exists := store.GetServiceState("test-service")
	if !exists {
		t.Fatal("Service should exist after initialization")
	}

	if state.ID != "test-service" {
		t.Errorf("Expected service ID 'test-service', got '%s'", state.ID)
	}

	if state.Status != StatusUnknown {
		t.Errorf("Expected status %s, got %s", StatusUnknown, state.Status)
	}

	// Test updating service state
	now := time.Now().UTC()
	newState := &ServiceState{
		ID:             "test-service",
		Status:         StatusUp,
		LastChecked:    now,
		ResponseTimeMs: 150,
		HTTPStatus:     200,
	}

	store.UpdateServiceState(newState)

	retrieved, exists := store.GetServiceState("test-service")
	if !exists {
		t.Fatal("Service should exist after update")
	}

	if retrieved.Status != StatusUp {
		t.Errorf("Expected status %s, got %s", StatusUp, retrieved.Status)
	}

	if retrieved.ResponseTimeMs != 150 {
		t.Errorf("Expected response time 150ms, got %d", retrieved.ResponseTimeMs)
	}

	if retrieved.HTTPStatus != 200 {
		t.Errorf("Expected HTTP status 200, got %d", retrieved.HTTPStatus)
	}

	if !retrieved.LastChecked.Equal(now) {
		t.Errorf("Expected last checked %v, got %v", now, retrieved.LastChecked)
	}
}

func TestGetAllServiceStates(t *testing.T) {
	store := NewStore()

	// Test empty store
	states := store.GetAllServiceStates()
	if len(states) != 0 {
		t.Errorf("Expected 0 services, got %d", len(states))
	}

	// Add multiple services
	services := []string{"service1", "service2", "service3"}
	for _, serviceID := range services {
		store.InitializeService(serviceID)
	}

	states = store.GetAllServiceStates()
	if len(states) != len(services) {
		t.Errorf("Expected %d services, got %d", len(services), len(states))
	}

	// Check all services are present
	for _, serviceID := range services {
		if _, exists := states[serviceID]; !exists {
			t.Errorf("Service %s not found in states", serviceID)
		}
	}
}

func TestSSEClientManagement(t *testing.T) {
	store := NewStore()

	// Test initial client count
	if store.GetClientCount() != 0 {
		t.Errorf("Expected 0 clients initially, got %d", store.GetClientCount())
	}

	// Add first client
	clientID1, eventChan1 := store.AddSSEClient()
	if clientID1 == "" {
		t.Error("Client ID should not be empty")
	}

	if eventChan1 == nil {
		t.Error("Event channel should not be nil")
	}

	if store.GetClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", store.GetClientCount())
	}

	// Add second client
	clientID2, _ := store.AddSSEClient()
	if clientID2 == "" {
		t.Error("Client ID should not be empty")
	}

	if clientID1 == clientID2 {
		t.Error("Client IDs should be unique")
	}

	if store.GetClientCount() != 2 {
		t.Errorf("Expected 2 clients, got %d", store.GetClientCount())
	}

	// Remove first client
	store.RemoveSSEClient(clientID1)

	if store.GetClientCount() != 1 {
		t.Errorf("Expected 1 client after removal, got %d", store.GetClientCount())
	}

	// Check that the channel was closed
	select {
	case _, ok := <-eventChan1:
		if ok {
			t.Error("Event channel should be closed after client removal")
		}
	default:
		// Channel might not have been read from, so it's still closed
	}

	// Remove second client
	store.RemoveSSEClient(clientID2)

	if store.GetClientCount() != 0 {
		t.Errorf("Expected 0 clients after removing all, got %d", store.GetClientCount())
	}

	// Test removing non-existent client (should not panic)
	store.RemoveSSEClient("non-existent")
}

func TestBroadcastSnapshot(t *testing.T) {
	store := NewStore()

	// Add some services
	store.InitializeService("service1")
	store.InitializeService("service2")

	// Update one service
	store.UpdateServiceState(&ServiceState{
		ID:             "service1",
		Status:         StatusUp,
		LastChecked:    time.Now().UTC(),
		ResponseTimeMs: 100,
		HTTPStatus:     200,
	})

	// Add client
	clientID, eventChan := store.AddSSEClient()

	// Broadcast snapshot
	store.BroadcastSnapshot(clientID)

	// Check that snapshot event was sent
	select {
	case event := <-eventChan:
		if event.Type != "snapshot" {
			t.Errorf("Expected event type 'snapshot', got '%s'", event.Type)
		}

		// Check event data structure
		data, ok := event.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Event data should be a map")
		}

		services, ok := data["services"]
		if !ok {
			t.Fatal("Event data should contain 'services' key")
		}

		serviceList, ok := services.([]*ServiceState)
		if !ok {
			t.Fatal("Services should be a slice of ServiceState")
		}

		if len(serviceList) != 2 {
			t.Errorf("Expected 2 services in snapshot, got %d", len(serviceList))
		}

	case <-time.After(1 * time.Second):
		t.Fatal("Expected to receive snapshot event within 1 second")
	}

	store.RemoveSSEClient(clientID)
}

func TestBroadcastConfigReload(t *testing.T) {
	store := NewStore()

	// Add client
	clientID, eventChan := store.AddSSEClient()

	// Broadcast config reload
	store.BroadcastConfigReload()

	// Check that config event was sent
	select {
	case event := <-eventChan:
		if event.Type != "config" {
			t.Errorf("Expected event type 'config', got '%s'", event.Type)
		}

		// Check event data structure
		data, ok := event.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Event data should be a map")
		}

		message, ok := data["message"]
		if !ok {
			t.Fatal("Event data should contain 'message' key")
		}

		if message != "config reloaded" {
			t.Errorf("Expected message 'config reloaded', got '%v'", message)
		}

		_, ok = data["timestamp"]
		if !ok {
			t.Fatal("Event data should contain 'timestamp' key")
		}

	case <-time.After(1 * time.Second):
		t.Fatal("Expected to receive config event within 1 second")
	}

	store.RemoveSSEClient(clientID)
}

func TestServiceStateImmutability(t *testing.T) {
	store := NewStore()

	// Initialize service
	store.InitializeService("test-service")

	// Get service state
	state1, _ := store.GetServiceState("test-service")

	// Modify the returned state
	state1.Status = StatusUp
	state1.ResponseTimeMs = 999

	// Get service state again
	state2, _ := store.GetServiceState("test-service")

	// The original state should not be affected
	if state2.Status != StatusUnknown {
		t.Errorf("Original state was modified: expected %s, got %s", StatusUnknown, state2.Status)
	}

	if state2.ResponseTimeMs != 0 {
		t.Errorf("Original state was modified: expected 0, got %d", state2.ResponseTimeMs)
	}
}
