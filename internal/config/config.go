// Package config provides YAML-based configuration management for Beacon.
// It supports environment variable expansion, validation, and hot-reload.
// Main types: Config, Background, Theme, UI, Group, Service.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Title      string     `yaml:"title" json:"title"`
	Favicon    string     `yaml:"favicon" json:"favicon"`
	Background Background `yaml:"background" json:"background"`
	Theme      Theme      `yaml:"theme" json:"theme"`
	UI         UI         `yaml:"ui" json:"ui"`
	Groups     []Group    `yaml:"groups" json:"groups"`
	Services   []Service  `yaml:"services" json:"services"`
}

// Background configuration for the dashboard
type Background struct {
	Type  string `yaml:"type" json:"type"`   // "solid" | "gradient" | "image"
	Value string `yaml:"value" json:"value"` // CSS value or URL
	Blur  int    `yaml:"blur" json:"blur"`   // px blur for background overlay
}

// Theme configuration
type Theme struct {
	Default     string `yaml:"default" json:"default"` // "light" | "dark" | "auto"
	AllowToggle bool   `yaml:"allowToggle" json:"allowToggle"`
}

// UI configuration
type UI struct {
	ShowDescriptions bool `yaml:"showDescriptions" json:"showDescriptions"`
}

// Group represents a service group
type Group struct {
	ID    string `yaml:"id" json:"id"`
	Title string `yaml:"title" json:"title"`
}

// Service represents a monitored service
type Service struct {
	ID          string     `yaml:"id" json:"id"`
	Name        string     `yaml:"name" json:"name"`
	Group       string     `yaml:"group" json:"group"`
	URL         string     `yaml:"url" json:"url"`
	Icon        string     `yaml:"icon" json:"icon"`
	Description string     `yaml:"description" json:"description"`
	Health      HealthSpec `yaml:"health" json:"health"`
}

// HealthSpec defines health check configuration
type HealthSpec struct {
	Endpoint        string            `yaml:"endpoint" json:"endpoint"`
	ExpectedStatus  int               `yaml:"expected_status" json:"expected_status"`
	Interval        time.Duration     `yaml:"interval" json:"interval"`
	Timeout         time.Duration     `yaml:"timeout" json:"timeout"`
	FollowRedirects bool              `yaml:"follow_redirects" json:"follow_redirects"`
	Retries         int               `yaml:"retries" json:"retries"`
	Headers         map[string]string `yaml:"headers" json:"headers"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the config
	expanded := expandEnvVars(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	setDefaults(&config)

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// expandEnvVars replaces ${VAR} and $VAR with environment variables
func expandEnvVars(s string) string {
	return os.ExpandEnv(s)
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	if config.Title == "" {
		config.Title = "Beacon"
	}

	if config.Theme.Default == "" {
		config.Theme.Default = "auto"
	}

	if config.Background.Type == "" {
		config.Background.Type = "solid"
		config.Background.Value = "#f8fafc"
	}

	for i := range config.Services {
		service := &config.Services[i]
		if service.Health.ExpectedStatus == 0 {
			service.Health.ExpectedStatus = 200
		}
		if service.Health.Interval == 0 {
			service.Health.Interval = 30 * time.Second
		}
		if service.Health.Timeout == 0 {
			service.Health.Timeout = 5 * time.Second
		}
		if service.Health.Retries == 0 {
			service.Health.Retries = 1
		}
		if service.Health.Headers == nil {
			service.Health.Headers = make(map[string]string)
		}
	}
}

// validateConfig validates the configuration structure
func validateConfig(config *Config) error {
	// Validate service IDs are unique
	serviceIDs := make(map[string]bool)
	for _, service := range config.Services {
		if service.ID == "" {
			return fmt.Errorf("service ID cannot be empty")
		}
		if serviceIDs[service.ID] {
			return fmt.Errorf("duplicate service ID: %s", service.ID)
		}
		serviceIDs[service.ID] = true

		if service.Name == "" {
			return fmt.Errorf("service name cannot be empty for service: %s", service.ID)
		}

		if service.Health.Endpoint == "" {
			return fmt.Errorf("health endpoint cannot be empty for service: %s", service.ID)
		}

		if service.Health.Interval < time.Second {
			return fmt.Errorf("health check interval too short for service %s: %v", service.ID, service.Health.Interval)
		}

		if service.Health.Timeout < time.Millisecond*100 {
			return fmt.Errorf("health check timeout too short for service %s: %v", service.ID, service.Health.Timeout)
		}
	}

	// Validate group IDs are unique
	groupIDs := make(map[string]bool)
	for _, group := range config.Groups {
		if group.ID == "" {
			return fmt.Errorf("group ID cannot be empty")
		}
		if groupIDs[group.ID] {
			return fmt.Errorf("duplicate group ID: %s", group.ID)
		}
		groupIDs[group.ID] = true
	}

	// Validate service groups exist
	for _, service := range config.Services {
		if service.Group != "" && !groupIDs[service.Group] {
			return fmt.Errorf("service %s references non-existent group: %s", service.ID, service.Group)
		}
	}

	// Validate theme default value
	validThemes := map[string]bool{"light": true, "dark": true, "auto": true}
	if !validThemes[config.Theme.Default] {
		return fmt.Errorf("invalid theme default: %s", config.Theme.Default)
	}

	// Validate background type
	validBackgroundTypes := map[string]bool{"solid": true, "gradient": true, "image": true}
	if !validBackgroundTypes[config.Background.Type] {
		return fmt.Errorf("invalid background type: %s", config.Background.Type)
	}

	return nil
}
