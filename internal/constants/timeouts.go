// Package constants provides shared constants used throughout the Beacon application.
// This centralizes timeout values, magic numbers, and other configuration constants
// to improve maintainability and consistency.
package constants

import "time"

// HTTP Server Timeouts
const (
	// DefaultReadTimeout protects against slow clients
	DefaultReadTimeout = 30 * time.Second

	// DefaultReadHeaderTimeout prevents slowloris attacks
	DefaultReadHeaderTimeout = 10 * time.Second

	// DefaultWriteTimeout protects non-SSE routes from hanging
	DefaultWriteTimeout = 15 * time.Second

	// DefaultIdleTimeout for keep-alive connections
	DefaultIdleTimeout = 120 * time.Second
)

// Health Check Timeouts
const (
	// DefaultHealthCheckTimeout for individual health checks
	DefaultHealthCheckTimeout = 5 * time.Second

	// DefaultHealthCheckInterval between health checks
	DefaultHealthCheckInterval = 30 * time.Second

	// DefaultHealthClientTimeout for HTTP client
	DefaultHealthClientTimeout = 30 * time.Second
)

// SSE Configuration
const (
	// SSEWriteDeadline for Server-Sent Events connections
	SSEWriteDeadline = 24 * time.Hour

	// SSEKeepaliveInterval for sending keepalive messages
	SSEKeepaliveInterval = 30 * time.Second

	// SSEEventChannelBuffer size for SSE event channels
	SSEEventChannelBuffer = 10
)

// Configuration Watching
const (
	// ConfigDebounceDelay for file change debouncing
	ConfigDebounceDelay = 100 * time.Millisecond

	// ConfigReloadDelay to ensure file write completion
	ConfigReloadDelay = 50 * time.Millisecond
)

// Default Values
const (
	// DefaultMaxConcurrentHealthChecks limits concurrent health checking
	DefaultMaxConcurrentHealthChecks = 50

	// DefaultHTTPPort for the server
	DefaultHTTPPort = 8080

	// DefaultBindAddress for the server
	DefaultBindAddress = "0.0.0.0"

	// DefaultConfigPath for configuration file
	DefaultConfigPath = "./config.yaml"
)

// Application Info
const (
	// DefaultAppTitle when not specified in config
	DefaultAppTitle = "Beacon"

	// DefaultTheme when not specified in config
	DefaultTheme = "auto"

	// DefaultBackgroundType when not specified in config
	DefaultBackgroundType = "solid"

	// DefaultBackgroundValue when not specified in config
	DefaultBackgroundValue = "#f8fafc"
)

// Health Check Defaults
const (
	// DefaultExpectedHTTPStatus for health checks
	DefaultExpectedHTTPStatus = 200

	// DefaultHealthRetries for failed checks
	DefaultHealthRetries = 1

	// MinHealthCheckInterval minimum allowed interval
	MinHealthCheckInterval = 1 * time.Second

	// MinHealthCheckTimeout minimum allowed timeout
	MinHealthCheckTimeout = 100 * time.Millisecond
)
