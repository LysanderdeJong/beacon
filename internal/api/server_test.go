package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LysanderdeJong/beacon/internal/config"
	"github.com/LysanderdeJong/beacon/internal/store"
)

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Title: "Test Dashboard",
	}
	serviceStore := store.NewStore()

	server := NewServer(cfg, serviceStore, nil)

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.config != cfg {
		t.Error("Server config not set correctly")
	}

	if server.store != serviceStore {
		t.Error("Server store not set correctly")
	}
}

func TestHandleGetConfig(t *testing.T) {
	cfg := &config.Config{
		Title: "Test Dashboard",
		Theme: config.Theme{
			Default:     "dark",
			AllowToggle: true,
		},
		Groups: []config.Group{
			{ID: "group1", Title: "Group 1"},
		},
	}
	serviceStore := store.NewStore()
	server := NewServer(cfg, serviceStore, nil)

	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	// Use the server's ServeHTTP method to get CORS headers
	server.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	expectedContentType := "application/json"
	if ct := rr.Header().Get("Content-Type"); ct != expectedContentType {
		t.Errorf("Expected content type %s, got %s", expectedContentType, ct)
	}

	var response config.Config
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Could not parse JSON response: %v", err)
	}

	if response.Title != "Test Dashboard" {
		t.Errorf("Expected title 'Test Dashboard', got %s", response.Title)
	}

	if response.Theme.Default != "dark" {
		t.Errorf("Expected theme default 'dark', got %s", response.Theme.Default)
	}
}

func TestHandleGetServices(t *testing.T) {
	cfg := &config.Config{}
	serviceStore := store.NewStore()
	server := NewServer(cfg, serviceStore, nil)

	// Add some test services
	serviceStore.InitializeService("service1")
	serviceStore.InitializeService("service2")

	serviceStore.UpdateServiceState(&store.ServiceState{
		ID:             "service1",
		Status:         store.StatusUp,
		LastChecked:    time.Now().UTC(),
		ResponseTimeMs: 100,
		HTTPStatus:     200,
	})

	serviceStore.UpdateServiceState(&store.ServiceState{
		ID:             "service2",
		Status:         store.StatusDown,
		LastChecked:    time.Now().UTC(),
		ResponseTimeMs: 0,
		HTTPStatus:     500,
		Error:          "Internal Server Error",
	})

	req, err := http.NewRequest("GET", "/api/services", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	// Use the server's ServeHTTP method to get CORS headers
	server.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	expectedContentType := "application/json"
	if ct := rr.Header().Get("Content-Type"); ct != expectedContentType {
		t.Errorf("Expected content type %s, got %s", expectedContentType, ct)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Could not parse JSON response: %v", err)
	}

	services, ok := response["services"]
	if !ok {
		t.Fatal("Expected 'services' key in response")
	}

	servicesSlice, ok := services.([]interface{})
	if !ok {
		t.Fatal("Expected services to be an array")
	}

	if len(servicesSlice) != 2 {
		t.Errorf("Expected 2 services, got %d", len(servicesSlice))
	}
}

func TestHandleSSE(t *testing.T) {
	cfg := &config.Config{}
	serviceStore := store.NewStore()
	server := NewServer(cfg, serviceStore, nil)

	req, err := http.NewRequest("GET", "/sse/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a mock ResponseWriter that implements http.Flusher
	rr := &mockResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		headers:          make(http.Header),
	}

	// Use a channel to track when headers are set
	done := make(chan bool, 1)

	go func() {
		defer func() {
			// Signal completion
			select {
			case done <- true:
			default:
			}
		}()

		// Call the handler but expect it to block
		server.ServeHTTP(rr, req)
	}()

	// Give the handler time to set headers
	time.Sleep(50 * time.Millisecond)

	// Check headers were set correctly
	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Expected content type 'text/event-stream', got '%s'", ct)
	}

	if cc := rr.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Expected cache control 'no-cache', got '%s'", cc)
	}

	if conn := rr.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("Expected connection 'keep-alive', got '%s'", conn)
	}

	// Check CORS headers
	if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin '*', got '%s'", origin)
	}
}

// mockResponseWriter implements http.ResponseWriter and http.Flusher for testing
type mockResponseWriter struct {
	*httptest.ResponseRecorder
	headers http.Header
}

func (m *mockResponseWriter) Header() http.Header {
	if m.headers == nil {
		m.headers = make(http.Header)
	}
	return m.headers
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	return m.ResponseRecorder.Write(data)
}

func (m *mockResponseWriter) WriteHeader(code int) {
	m.ResponseRecorder.WriteHeader(code)
}

func (m *mockResponseWriter) Flush() {
	// Mock flush - do nothing
}

func TestCORSHeaders(t *testing.T) {
	cfg := &config.Config{}
	serviceStore := store.NewStore()
	server := NewServer(cfg, serviceStore, nil)

	testCases := []struct {
		endpoint string
		path     string
	}{
		{"GET /api/config", "/api/config"},
		{"GET /api/services", "/api/services"},
	}

	for _, tc := range testCases {
		t.Run(tc.endpoint, func(t *testing.T) {
			req, err := http.NewRequest("GET", tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			server.ServeHTTP(rr, req)

			// Check CORS headers
			if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
				t.Errorf("Expected Access-Control-Allow-Origin '*', got '%s'", origin)
			}

			if headers := rr.Header().Get("Access-Control-Allow-Headers"); headers != "Content-Type" {
				t.Errorf("Expected Access-Control-Allow-Headers 'Content-Type', got '%s'", headers)
			}

			if methods := rr.Header().Get("Access-Control-Allow-Methods"); methods != "GET, POST, OPTIONS" {
				t.Errorf("Expected Access-Control-Allow-Methods 'GET, POST, OPTIONS', got '%s'", methods)
			}
		})
	}
}

func TestOptionsRequests(t *testing.T) {
	cfg := &config.Config{}
	serviceStore := store.NewStore()
	server := NewServer(cfg, serviceStore, nil)

	req, err := http.NewRequest("OPTIONS", "/api/config", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d for OPTIONS request, got %d", http.StatusOK, status)
	}

	// Check CORS headers are present
	if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin '*', got '%s'", origin)
	}
}

func TestInvalidHTTPMethods(t *testing.T) {
	cfg := &config.Config{}
	serviceStore := store.NewStore()
	server := NewServer(cfg, serviceStore, nil)

	testCases := []struct {
		method   string
		endpoint string
	}{
		{"POST", "/api/config"},
		{"PUT", "/api/config"},
		{"DELETE", "/api/services"},
		{"PATCH", "/api/services"},
	}

	for _, tc := range testCases {
		t.Run(tc.method+" "+tc.endpoint, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, tc.endpoint, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			server.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusMethodNotAllowed {
				t.Errorf("Expected status code %d for %s request, got %d",
					http.StatusMethodNotAllowed, tc.method, status)
			}
		})
	}
}

func TestHandleSelfHealth(t *testing.T) {
	cfg := &config.Config{}
	serviceStore := store.NewStore()
	server := NewServer(cfg, serviceStore, nil)

	req, err := http.NewRequest("GET", "/.well-known/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	expectedContentType := "application/json"
	if ct := rr.Header().Get("Content-Type"); ct != expectedContentType {
		t.Errorf("Expected content type %s, got %s", expectedContentType, ct)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Could not parse JSON response: %v", err)
	}

	if status, ok := response["status"]; !ok || status != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", status)
	}

	if _, ok := response["timestamp"]; !ok {
		t.Error("Expected timestamp in response")
	}
}

func TestServerConfiguration(t *testing.T) {
	cfg := &config.Config{
		Title:   "Custom Title",
		Favicon: "/custom-favicon.ico",
		Background: config.Background{
			Type:  "gradient",
			Value: "linear-gradient(45deg, #1e3c72, #2a5298)",
			Blur:  5,
		},
		Theme: config.Theme{
			Default:     "auto",
			AllowToggle: false,
		},
		UI: config.UI{
			ShowDescriptions: true,
		},
	}

	serviceStore := store.NewStore()
	server := NewServer(cfg, serviceStore, nil)

	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	var response config.Config
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Could not parse JSON response: %v", err)
	}

	// Verify all configuration fields are present
	if response.Title != "Custom Title" {
		t.Errorf("Expected title 'Custom Title', got %s", response.Title)
	}

	if response.Favicon != "/custom-favicon.ico" {
		t.Errorf("Expected favicon '/custom-favicon.ico', got %s", response.Favicon)
	}

	if response.Background.Type != "gradient" {
		t.Errorf("Expected background type 'gradient', got %s", response.Background.Type)
	}

	if response.Theme.Default != "auto" {
		t.Errorf("Expected theme default 'auto', got %s", response.Theme.Default)
	}

	if response.Theme.AllowToggle != false {
		t.Errorf("Expected theme allowToggle false, got %v", response.Theme.AllowToggle)
	}

	if response.UI.ShowDescriptions != true {
		t.Errorf("Expected ui showDescriptions true, got %v", response.UI.ShowDescriptions)
	}
}

func TestSSEHeaders(t *testing.T) {
	cfg := &config.Config{
		Services: []config.Service{},
	}
	store := store.NewStore()
	server := NewServer(cfg, store, http.Dir("."))

	// Create a request with a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, "GET", "/sse/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Use a simple ResponseRecorder but cancel the context quickly
	rr := httptest.NewRecorder()

	// Start the SSE handler in a goroutine
	done := make(chan bool, 1)
	go func() {
		server.ServeHTTP(rr, req)
		done <- true
	}()

	// Wait briefly for headers to be set, then cancel the context
	time.Sleep(100 * time.Millisecond)
	cancel() // This should trigger r.Context().Done() in the SSE handler

	// Wait for handler to finish or timeout
	select {
	case <-done:
		// Handler finished
	case <-time.After(2 * time.Second):
		t.Error("SSE handler timed out")
		return
	}

	// Check SSE-specific headers
	if contentType := rr.Header().Get("Content-Type"); contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	if cacheControl := rr.Header().Get("Cache-Control"); cacheControl != "no-cache" {
		t.Errorf("Expected Cache-Control 'no-cache', got '%s'", cacheControl)
	}

	if connection := rr.Header().Get("Connection"); connection != "keep-alive" {
		t.Errorf("Expected Connection 'keep-alive', got '%s'", connection)
	}

	// Check CORS headers
	if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin '*', got '%s'", origin)
	}
}
