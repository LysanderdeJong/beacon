# Beacon Monitoring Dashboard

Beacon is a production-ready, single-binary monitoring dashboard for homelab and service health. It features:
- Real-time health monitoring via Server-Sent Events (SSE)
- Responsive Alpine.js + Tailwind CSS frontend
- YAML configuration with hot-reload
- CLI interface with comprehensive flags
- Embedded frontend assets for easy deployment

## Features
- **Live Health Dashboard**: See service status in real time
- **Configurable via YAML**: All services, groups, and UI are defined in `config.yaml`
- **Hot-Reload**: Changes to config are picked up automatically
- **REST API**: Query config and service state programmatically
- **SSE Streaming**: Pushes health updates to the frontend
- **Single Binary**: No external dependencies, easy deployment

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
- **services**: List of monitored services
- **groups**: Optional grouping for dashboard display
- **theme/background/ui**: Customize dashboard appearance
- **env vars**: Use `${VAR}` for environment variable expansion

## Architecture
- **Go Backend**: Handles config, health checks, SSE, REST API
- **Frontend**: Alpine.js for reactivity, Tailwind CSS for styling
- **go:embed**: Embeds all frontend assets in the binary
- **File Watcher**: Hot-reloads config.yaml on change

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
