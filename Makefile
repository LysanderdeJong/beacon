# Beacon Makefile

# Variables
BINARY_NAME=beacon
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%I:%M:%S%p')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)"

# Default target
.PHONY: all
all: build

# Build the application
.PHONY: build
build:
	@echo "Building Beacon v$(VERSION) ($(COMMIT))"
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/beacon

# Build for multiple platforms
.PHONY: build-all
build-all: build-linux build-darwin build-windows

.PHONY: build-linux
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/beacon
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/beacon

.PHONY: build-darwin
build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/beacon
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/beacon

.PHONY: build-windows
build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/beacon

# Run the application
.PHONY: run
run: build
	./bin/$(BINARY_NAME) -config config.example.yaml

# Run with auto-open browser
.PHONY: dev
dev: build
	./bin/$(BINARY_NAME) -config config.example.yaml -auto-open

# Test the application
.PHONY: test
test:
	go test -v ./...

# Test with coverage
.PHONY: test-coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run

# Create example config
.PHONY: config
config:
	@if [ ! -f config.yaml ]; then \
		cp config.example.yaml config.yaml; \
		echo "Created config.yaml from example"; \
	else \
		echo "config.yaml already exists"; \
	fi

# Install the binary to GOPATH/bin
.PHONY: install
install:
	go install $(LDFLAGS) ./cmd/beacon

# Create release archives
.PHONY: release
release: build-all
	@echo "Creating release archives..."
	mkdir -p releases
	tar -czf releases/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C bin $(BINARY_NAME)-linux-amd64 -C .. config.example.yaml README.md
	tar -czf releases/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz -C bin $(BINARY_NAME)-linux-arm64 -C .. config.example.yaml README.md
	tar -czf releases/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz -C bin $(BINARY_NAME)-darwin-amd64 -C .. config.example.yaml README.md
	tar -czf releases/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz -C bin $(BINARY_NAME)-darwin-arm64 -C .. config.example.yaml README.md
	zip -j releases/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip bin/$(BINARY_NAME)-windows-amd64.exe config.example.yaml README.md

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         Build the application"
	@echo "  build-all     Build for all platforms"
	@echo "  run           Build and run with example config"
	@echo "  dev           Build and run with auto-open browser"
	@echo "  test          Run tests"
	@echo "  test-coverage Run tests with coverage"
	@echo "  clean         Clean build artifacts"
	@echo "  deps          Install dependencies"
	@echo "  fmt           Format code"
	@echo "  lint          Lint code"
	@echo "  config        Create config.yaml from example"
	@echo "  install       Install binary to GOPATH/bin"
	@echo "  release       Create release archives"
	@echo "  help          Show this help"
