package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `
title: "Test Beacon"
theme:
  default: "dark"
  allowToggle: true
ui:
  showDescriptions: false
groups:
  - id: "test-group"
    title: "Test Group"
services:
  - id: "test-service"
    name: "Test Service"
    group: "test-group"
    url: "https://example.com"
    icon: "🧪"
    description: "Test service"
    health:
      endpoint: "https://example.com/health"
      expected_status: 200
      interval: 30s
      timeout: 5s
      retries: 2
`

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "beacon-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	// Load and test configuration
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test basic fields
	if config.Title != "Test Beacon" {
		t.Errorf("Expected title 'Test Beacon', got '%s'", config.Title)
	}

	if config.Theme.Default != "dark" {
		t.Errorf("Expected theme 'dark', got '%s'", config.Theme.Default)
	}

	if config.Theme.AllowToggle != true {
		t.Errorf("Expected theme.allowToggle true, got %v", config.Theme.AllowToggle)
	}

	if config.UI.ShowDescriptions != false {
		t.Errorf("Expected ui.showDescriptions false, got %v", config.UI.ShowDescriptions)
	}

	// Test groups
	if len(config.Groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(config.Groups))
	}

	group := config.Groups[0]
	if group.ID != "test-group" {
		t.Errorf("Expected group ID 'test-group', got '%s'", group.ID)
	}

	if group.Title != "Test Group" {
		t.Errorf("Expected group title 'Test Group', got '%s'", group.Title)
	}

	// Test services
	if len(config.Services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(config.Services))
	}

	service := config.Services[0]
	if service.ID != "test-service" {
		t.Errorf("Expected service ID 'test-service', got '%s'", service.ID)
	}

	if service.Name != "Test Service" {
		t.Errorf("Expected service name 'Test Service', got '%s'", service.Name)
	}

	if service.Group != "test-group" {
		t.Errorf("Expected service group 'test-group', got '%s'", service.Group)
	}

	if service.URL != "https://example.com" {
		t.Errorf("Expected service URL 'https://example.com', got '%s'", service.URL)
	}

	if service.Icon != "🧪" {
		t.Errorf("Expected service icon '🧪', got '%s'", service.Icon)
	}

	if service.Description != "Test service" {
		t.Errorf("Expected service description 'Test service', got '%s'", service.Description)
	}

	// Test health spec
	health := service.Health
	if health.Endpoint != "https://example.com/health" {
		t.Errorf("Expected health endpoint 'https://example.com/health', got '%s'", health.Endpoint)
	}

	if health.ExpectedStatus != 200 {
		t.Errorf("Expected health expected_status 200, got %d", health.ExpectedStatus)
	}

	if health.Interval != 30*time.Second {
		t.Errorf("Expected health interval 30s, got %v", health.Interval)
	}

	if health.Timeout != 5*time.Second {
		t.Errorf("Expected health timeout 5s, got %v", health.Timeout)
	}

	if health.Retries != 2 {
		t.Errorf("Expected health retries 2, got %d", health.Retries)
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// Create minimal config
	configContent := `
services:
  - id: "minimal-service"
    name: "Minimal Service"
    health:
      endpoint: "https://example.com"
`

	tmpFile, err := os.CreateTemp("", "beacon-minimal-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test defaults
	if config.Title != "Beacon" {
		t.Errorf("Expected default title 'Beacon', got '%s'", config.Title)
	}

	if config.Theme.Default != "auto" {
		t.Errorf("Expected default theme 'auto', got '%s'", config.Theme.Default)
	}

	if config.Background.Type != "solid" {
		t.Errorf("Expected default background type 'solid', got '%s'", config.Background.Type)
	}

	service := config.Services[0]
	if service.Health.ExpectedStatus != 200 {
		t.Errorf("Expected default expected_status 200, got %d", service.Health.ExpectedStatus)
	}

	if service.Health.Interval != 30*time.Second {
		t.Errorf("Expected default interval 30s, got %v", service.Health.Interval)
	}

	if service.Health.Timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", service.Health.Timeout)
	}

	if service.Health.Retries != 1 {
		t.Errorf("Expected default retries 1, got %d", service.Health.Retries)
	}
}

func TestValidateConfigErrors(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectError   string
	}{
		{
			name: "duplicate service IDs",
			configContent: `
services:
  - id: "duplicate"
    name: "Service 1"
    health:
      endpoint: "https://example.com"
  - id: "duplicate"
    name: "Service 2"
    health:
      endpoint: "https://example.com"
`,
			expectError: "duplicate service ID",
		},
		{
			name: "empty service ID",
			configContent: `
services:
  - id: ""
    name: "Empty ID Service"
    health:
      endpoint: "https://example.com"
`,
			expectError: "service ID cannot be empty",
		},
		{
			name: "empty service name",
			configContent: `
services:
  - id: "test"
    name: ""
    health:
      endpoint: "https://example.com"
`,
			expectError: "service name cannot be empty",
		},
		{
			name: "empty health endpoint",
			configContent: `
services:
  - id: "test"
    name: "Test Service"
    health:
      endpoint: ""
`,
			expectError: "health endpoint cannot be empty",
		},
		{
			name: "invalid theme",
			configContent: `
theme:
  default: "invalid"
services:
  - id: "test"
    name: "Test Service"
    health:
      endpoint: "https://example.com"
`,
			expectError: "invalid theme default",
		},
		{
			name: "non-existent group reference",
			configContent: `
services:
  - id: "test"
    name: "Test Service"
    group: "non-existent"
    health:
      endpoint: "https://example.com"
`,
			expectError: "references non-existent group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "beacon-invalid-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.configContent); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			tmpFile.Close()

			_, err = LoadConfig(tmpFile.Name())
			if err == nil {
				t.Fatalf("Expected error containing '%s', but got no error", tt.expectError)
			}

			if !contains(err.Error(), tt.expectError) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectError, err.Error())
			}
		})
	}
}

func TestEnvironmentVariableExpansion(t *testing.T) {
	// Set environment variable for testing
	os.Setenv("TEST_TOKEN", "secret123")
	defer os.Unsetenv("TEST_TOKEN")

	configContent := `
services:
  - id: "test-service"
    name: "Test Service"
    health:
      endpoint: "https://example.com"
      headers:
        Authorization: "Bearer ${TEST_TOKEN}"
`

	tmpFile, err := os.CreateTemp("", "beacon-env-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	service := config.Services[0]
	expectedAuth := "Bearer secret123"
	if service.Health.Headers["Authorization"] != expectedAuth {
		t.Errorf("Expected Authorization header '%s', got '%s'",
			expectedAuth, service.Health.Headers["Authorization"])
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(substr) > 0 && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && (s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
