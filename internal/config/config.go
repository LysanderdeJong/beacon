// Package config provides YAML-based configuration management for Beacon.
// It supports environment variable expansion, validation, and hot-reload.
// Main types: Config, Background, Theme, UI, Group, Service.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/LysanderdeJong/beacon/internal/constants"
	"github.com/LysanderdeJong/beacon/internal/validation"
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

// GetID implements validation.GroupIDProvider
func (g Group) GetID() string { return g.ID }

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

// GetID implements validation.ServiceIDProvider
func (s Service) GetID() string { return s.ID }

// GetName implements validation.ServiceIDProvider
func (s Service) GetName() string { return s.Name }

// GetGroup implements validation.ServiceIDProvider
func (s Service) GetGroup() string { return s.Group }

// GetHealthInterval implements validation.ServiceIDProvider
func (s Service) GetHealthInterval() time.Duration { return s.Health.Interval }

// GetHealthTimeout implements validation.ServiceIDProvider
func (s Service) GetHealthTimeout() time.Duration { return s.Health.Timeout }

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
		config.Title = constants.DefaultAppTitle
	}

	if config.Theme.Default == "" {
		config.Theme.Default = constants.DefaultTheme
	}

	if config.Background.Type == "" {
		config.Background.Type = constants.DefaultBackgroundType
		config.Background.Value = constants.DefaultBackgroundValue
	}

	for i := range config.Services {
		service := &config.Services[i]
		if service.Health.ExpectedStatus == 0 {
			service.Health.ExpectedStatus = constants.DefaultExpectedHTTPStatus
		}
		if service.Health.Interval == 0 {
			service.Health.Interval = constants.DefaultHealthCheckInterval
		}
		if service.Health.Timeout == 0 {
			service.Health.Timeout = constants.DefaultHealthCheckTimeout
		}
		if service.Health.Retries == 0 {
			service.Health.Retries = constants.DefaultHealthRetries
		}
		if service.Health.Headers == nil {
			service.Health.Headers = make(map[string]string)
		}
	}
}

// validateConfig validates the configuration structure
func validateConfig(config *Config) error {
	// Validate services using the new validation helpers
	if err := validation.ValidateServices(config.Services); err != nil {
		return err
	}

	// Validate groups using the new validation helpers
	if err := validation.ValidateGroups(config.Groups); err != nil {
		return err
	}

	// Validate service group references
	if err := validation.ValidateServiceGroupReferences(config.Services, config.Groups); err != nil {
		return err
	}

	// Validate additional service fields not covered by generic validation
	for _, service := range config.Services {
		if service.Health.Endpoint == "" {
			return fmt.Errorf("health endpoint cannot be empty for service: %s", service.ID)
		}
	}

	// Validate theme default value
	validThemes := map[string]bool{"light": true, "dark": true, "auto": true}
	if err := validation.ValidateFieldInSet(config.Theme.Default, validThemes, "theme default"); err != nil {
		return err
	}

	// Validate background type
	validBackgroundTypes := map[string]bool{"solid": true, "gradient": true, "image": true}
	if err := validation.ValidateFieldInSet(config.Background.Type, validBackgroundTypes, "background type"); err != nil {
		return err
	}

	return nil
}
