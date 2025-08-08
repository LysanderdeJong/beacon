package health

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LysanderdeJong/beacon/internal/config"
	"github.com/LysanderdeJong/beacon/internal/store"
)

func TestNewChecker(t *testing.T) {
	serviceStore := store.NewStore()
	checker := NewChecker(serviceStore, 10)

	if checker == nil {
		t.Fatal("NewChecker returned nil")
	}

	if checker.store != serviceStore {
		t.Error("Checker store not set correctly")
	}
}

func TestStartChecker(t *testing.T) {
	// Create test servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server2.Close()

	// Create services
	services := []config.Service{
		{
			ID:   "service1",
			Name: "Service 1",
			URL:  server1.URL,
			Health: config.HealthSpec{
				Timeout:        5 * time.Second,
				Interval:       100 * time.Millisecond,
				ExpectedStatus: 200,
			},
		},
		{
			ID:   "service2",
			Name: "Service 2",
			URL:  server2.URL,
			Health: config.HealthSpec{
				Timeout:        5 * time.Second,
				Interval:       100 * time.Millisecond,
				ExpectedStatus: 200,
			},
		},
	}

	serviceStore := store.NewStore()
	checker := NewChecker(serviceStore, 10)

	// Initialize services in store
	for _, service := range services {
		serviceStore.InitializeService(service.ID)
	}

	// Start checker
	go checker.Start(services)

	// Wait for initial checks
	time.Sleep(200 * time.Millisecond)

	// Verify services are checked
	state1, exists1 := serviceStore.GetServiceState("service1")
	if !exists1 {
		t.Fatal("Service 1 state not found")
	}

	state2, exists2 := serviceStore.GetServiceState("service2")
	if !exists2 {
		t.Fatal("Service 2 state not found")
	}

	if state1.Status != store.StatusUp {
		t.Errorf("Expected service1 status %s, got %s", store.StatusUp, state1.Status)
	}

	if state2.Status != store.StatusUp {
		t.Errorf("Expected service2 status %s, got %s", store.StatusUp, state2.Status)
	}

	// Stop checker
	checker.Stop()
}

func TestHealthCheckDown(t *testing.T) {
	// Create a test server that returns 500 Internal Server Error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	service := config.Service{
		ID:   "test-service",
		Name: "Test Service",
		URL:  server.URL,
		Health: config.HealthSpec{
			Timeout:        5 * time.Second,
			Interval:       100 * time.Millisecond,
			ExpectedStatus: 200, // Expect 200 but server returns 500
		},
	}

	serviceStore := store.NewStore()
	checker := NewChecker(serviceStore, 10)

	// Initialize service in store
	serviceStore.InitializeService(service.ID)

	// Start checker
	go checker.Start([]config.Service{service})

	// Wait for check
	time.Sleep(200 * time.Millisecond)

	// Verify the result
	state, exists := serviceStore.GetServiceState(service.ID)
	if !exists {
		t.Fatal("Service state not found")
	}

	if state.Status != store.StatusDown {
		t.Errorf("Expected status %s, got %s", store.StatusDown, state.Status)
	}

	if state.HTTPStatus != 500 {
		t.Errorf("Expected HTTP status 500, got %d", state.HTTPStatus)
	}

	// Stop checker
	checker.Stop()
}

func TestHealthCheckTimeout(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	service := config.Service{
		ID:   "test-service",
		Name: "Test Service",
		URL:  server.URL,
		Health: config.HealthSpec{
			Timeout:        100 * time.Millisecond, // Shorter than server delay
			Interval:       100 * time.Millisecond,
			ExpectedStatus: 200,
		},
	}

	serviceStore := store.NewStore()
	checker := NewChecker(serviceStore, 10)

	// Initialize service in store
	serviceStore.InitializeService(service.ID)

	// Start checker
	go checker.Start([]config.Service{service})

	// Wait for check
	time.Sleep(300 * time.Millisecond)

	// Verify the result
	state, exists := serviceStore.GetServiceState(service.ID)
	if !exists {
		t.Fatal("Service state not found")
	}

	if state.Status != store.StatusDown {
		t.Errorf("Expected status %s due to timeout, got %s", store.StatusDown, state.Status)
	}

	if state.Error == "" {
		t.Error("Expected error message for timeout")
	}

	// Stop checker
	checker.Stop()
}

func TestHealthCheckUnreachable(t *testing.T) {
	service := config.Service{
		ID:   "test-service",
		Name: "Test Service",
		URL:  "http://localhost:99999", // Non-existent port
		Health: config.HealthSpec{
			Timeout:        5 * time.Second,
			Interval:       100 * time.Millisecond,
			ExpectedStatus: 200,
		},
	}

	serviceStore := store.NewStore()
	checker := NewChecker(serviceStore, 10)

	// Initialize service in store
	serviceStore.InitializeService(service.ID)

	// Start checker
	go checker.Start([]config.Service{service})

	// Wait for check
	time.Sleep(200 * time.Millisecond)

	// Verify the result
	state, exists := serviceStore.GetServiceState(service.ID)
	if !exists {
		t.Fatal("Service state not found")
	}

	if state.Status != store.StatusDown {
		t.Errorf("Expected status %s, got %s", store.StatusDown, state.Status)
	}

	if state.Error == "" {
		t.Error("Expected error message for unreachable service")
	}

	// Stop checker
	checker.Stop()
}

func TestCheckerStop(t *testing.T) {
	service := config.Service{
		ID:   "test-service",
		Name: "Test Service",
		URL:  "http://example.com",
		Health: config.HealthSpec{
			Timeout:        5 * time.Second,
			Interval:       100 * time.Millisecond,
			ExpectedStatus: 200,
		},
	}

	serviceStore := store.NewStore()
	checker := NewChecker(serviceStore, 10)

	// Initialize service in store
	serviceStore.InitializeService(service.ID)

	// Start checker
	done := make(chan bool)
	go func() {
		checker.Start([]config.Service{service})
		done <- true
	}()

	// Wait a bit, then stop
	time.Sleep(50 * time.Millisecond)
	checker.Stop()

	// Wait for checker to stop
	select {
	case <-done:
		// Checker stopped successfully
	case <-time.After(1 * time.Second):
		t.Fatal("Checker did not stop within 1 second")
	}
}

func TestConcurrentHealthChecks(t *testing.T) {
	// Create multiple test servers
	servers := make([]*httptest.Server, 5)
	for i := 0; i < 5; i++ {
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer servers[i].Close()
	}

	// Create services
	services := make([]config.Service, 5)
	for i := 0; i < 5; i++ {
		services[i] = config.Service{
			ID:   fmt.Sprintf("service%d", i),
			Name: fmt.Sprintf("Service %d", i),
			URL:  servers[i].URL,
			Health: config.HealthSpec{
				Timeout:        5 * time.Second,
				Interval:       50 * time.Millisecond,
				ExpectedStatus: 200,
			},
		}
	}

	serviceStore := store.NewStore()
	checker := NewChecker(serviceStore, 10)

	// Initialize services in store
	for _, service := range services {
		serviceStore.InitializeService(service.ID)
	}

	// Start checker
	start := time.Now()
	go checker.Start(services)

	// Wait for checks to complete
	time.Sleep(100 * time.Millisecond)

	// Verify all services are checked
	for i := 0; i < 5; i++ {
		serviceID := fmt.Sprintf("service%d", i)
		state, exists := serviceStore.GetServiceState(serviceID)
		if !exists {
			t.Fatalf("Service %s state not found", serviceID)
		}

		if state.Status != store.StatusUp {
			t.Errorf("Expected service %s status %s, got %s", serviceID, store.StatusUp, state.Status)
		}
	}

	elapsed := time.Since(start)
	// With concurrency, all checks should complete faster than sequential execution
	if elapsed > 200*time.Millisecond {
		t.Errorf("Health checks took too long: %v (expected < 200ms)", elapsed)
	}

	// Stop checker
	checker.Stop()
}
