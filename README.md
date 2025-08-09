# Beacon Monitoring Dashboard

Beacon is a production-ready, single-binary monitoring dashboard for homelab and service health. It features:
- Real-time health monitoring via Server-Sent Events (SSE)
- Responsive Alpine.js + Tailwind CSS frontend (CDN only, no build step)
- YAML configuration with hot-reload and environment variable expansion
- CLI interface with comprehensive flags
- Embedded frontend assets for easy deployment
- Robust config validation and error handling

## Features
- **Live Health Dashboard**: See service status in real time
- **Configurable via YAML**: All services, groups, and UI are defined in `config.yaml`
- **Hot-Reload**: Changes to config are picked up automatically (fsnotify watcher, debounce logic)
- **REST API**: Query config and service state programmatically
- **SSE Streaming**: Pushes health updates to the frontend
- **Single Binary**: No external dependencies, easy deployment
- **Comprehensive Testing**: Unit tests for all major packages

## Quick Start
1. **Build the binary:**
   ```sh
   go build -o beacon.exe ./cmd/beacon
   ```
2. **Create your config:**
   Copy `config.example.yaml` to `config.yaml` and edit as needed.
3. **Run the dashboard:**
   ```sh
   ./beacon.exe --config config.yaml
   ```
4. **Open the dashboard:**
   Visit [http://localhost:8080](http://localhost:8080)

## Configuration
All settings are defined in a single YAML file. Example:
```yaml
# config.yaml
services:
  - id: web
    name: Web Server
    url: http://localhost:80
    health:
      endpoint: /health
      expected_status: 200
      interval: 10s
```
- **services**: List of monitored services (ID, name, group, URL, icon, description, health spec)
- **groups**: Optional grouping for dashboard display
- **theme/background/ui**: Customize dashboard appearance
- **env vars**: Use `${VAR}` for environment variable expansion

## Architecture
- **Go Backend**: Handles config, health checks, SSE, REST API
- **Frontend**: Alpine.js for reactivity, Tailwind CSS for styling (CDN only)
- **go:embed**: Embeds all frontend assets in the binary
- **File Watcher**: Hot-reloads config.yaml on change (fsnotify, debounce)
- **Config Validation**: Ensures unique IDs, valid references, and required fields
- **Service State**: Immutable deep copy for concurrency safety
- **Error Handling**: Consistent JSON error responses via middleware

## API Endpoints
- `GET /api/config` - Returns current config
- `GET /api/services` - Returns all service states
- `GET /api/service/{id}` - Returns state for a single service
- `GET /sse/health` - SSE stream of health updates
- `GET /.well-known/health` - Health of Beacon itself

## Development
- **Run all tests:**
  ```sh
  go test -v ./...
  ```
- **Build binary:**
  ```sh
  go build -o beacon.exe ./cmd/beacon
  ```
- **Format code:**
  ```sh
  go fmt ./...
  ```
- **Lint code:**
  ```sh
  golangci-lint run
  ```
- **Install dependencies:**
  ```sh
  go mod tidy
  ```

## Frontend Best Practices
- Tailwind CDN: Use utility classes directly in HTML (no @apply)
- Alpine.js: Use x-data, x-init, x-bind:class for dynamic UI
- Status indicators: Use dynamic class binding for service status
- Stale service detection: UI sets status to 'unknown' if no SSE update by nextCheck + 5s
