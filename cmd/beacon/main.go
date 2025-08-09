// Main entry point for Beacon monitoring dashboard.
// Handles CLI flags, config loading, server startup, and graceful shutdown.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/LysanderdeJong/beacon/internal/api"
	"github.com/LysanderdeJong/beacon/internal/config"
	"github.com/LysanderdeJong/beacon/internal/constants"
	"github.com/LysanderdeJong/beacon/internal/health"
	"github.com/LysanderdeJong/beacon/internal/store"
	"github.com/LysanderdeJong/beacon/internal/ui"
)

var (
	version = "1.0.0"
	commit  = "dev"
)

type flags struct {
	configPath    string
	port          int
	bind          string
	logLevel      string
	tlsCert       string
	tlsKey        string
	basePath      string
	autoOpen      bool
	showVersion   bool
	maxConcurrent int
}

func main() {
	var f flags
	parseFlags(&f)

	if f.showVersion {
		fmt.Printf("Beacon v%s (%s)\n", version, commit)
		os.Exit(0)
	}

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Starting Beacon v%s", version)

	// Load configuration
	cfg, err := config.LoadConfig(f.configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Loaded configuration from %s", f.configPath)

	// Create store
	store := store.NewStore()

	// Create health checker
	checker := health.NewChecker(store, f.maxConcurrent)

	// Start health checking
	checker.Start(cfg.Services)
	log.Printf("Started health checking for %d services", len(cfg.Services))

	// Create API server
	staticFS := ui.GetStaticFileSystem()
	server := api.NewServer(cfg, store, staticFS)

	// Setup config file watcher for hot-reload
	watcher, err := config.NewWatcher(f.configPath)
	if err != nil {
		log.Printf("Warning: failed to create config watcher: %v", err)
	} else {
		watcher.Start()
		defer watcher.Stop()

		// Handle config reloads in a separate goroutine
		go func() {
			for {
				select {
				case newConfig := <-watcher.ReloadChan():
					log.Println("Reloading configuration...")

					// Update server config
					server.UpdateConfig(newConfig)

					// Restart health checking with new services
					checker.Start(newConfig.Services)

					log.Printf("Configuration reloaded successfully with %d services", len(newConfig.Services))

				case err := <-watcher.ErrorChan():
					log.Printf("Config watcher error: %v", err)
				}
			}
		}()
	}

	// Create HTTP server with SSE-optimized configuration using constants
	// Based on best practices research: disable write timeout for SSE, but keep other security timeouts
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", f.bind, f.port),
		Handler: server,

		// Sensible global timeouts for all routes using constants:
		ReadTimeout:       constants.DefaultReadTimeout,       // Protect against slow clients
		ReadHeaderTimeout: constants.DefaultReadHeaderTimeout, // Prevent slowloris attacks
		WriteTimeout:      constants.DefaultWriteTimeout,      // Protect non-SSE routes from hanging
		IdleTimeout:       constants.DefaultIdleTimeout,       // Keep-alive for better performance

		// Enhanced error logging
		ErrorLog: log.Default(),
	}

	// Setup graceful shutdown
	cancel := make(chan struct{})
	defer close(cancel)

	// Start HTTP server
	go func() {
		var err error
		if f.tlsCert != "" && f.tlsKey != "" {
			log.Printf("Starting HTTPS server on %s", httpServer.Addr)
			err = httpServer.ListenAndServeTLS(f.tlsCert, f.tlsKey)
		} else {
			log.Printf("Starting HTTP server on %s", httpServer.Addr)
			err = httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Auto-open browser if requested
	if f.autoOpen {
		go func() {
			time.Sleep(1 * time.Second) // Give server time to start
			url := fmt.Sprintf("http://localhost:%d", f.port)
			if f.tlsCert != "" && f.tlsKey != "" {
				url = fmt.Sprintf("https://localhost:%d", f.port)
			}
			openBrowser(url)
		}()
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Received shutdown signal, stopping...")

	// Stop health checker
	checker.Stop()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

func parseFlags(f *flags) {
	flag.StringVar(&f.configPath, "config", "./config.yaml", "Path to configuration file")
	flag.StringVar(&f.configPath, "c", "./config.yaml", "Path to configuration file (short)")
	flag.IntVar(&f.port, "port", 8080, "Port to listen on")
	flag.IntVar(&f.port, "p", 8080, "Port to listen on (short)")
	flag.StringVar(&f.bind, "bind", "0.0.0.0", "Interface to bind to")
	flag.StringVar(&f.logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&f.tlsCert, "tls-cert", "", "Path to TLS certificate file")
	flag.StringVar(&f.tlsKey, "tls-key", "", "Path to TLS key file")
	flag.StringVar(&f.basePath, "base-path", "", "Base path for reverse proxy")
	flag.BoolVar(&f.autoOpen, "auto-open", false, "Automatically open browser on start")
	flag.BoolVar(&f.showVersion, "version", false, "Show version and exit")
	flag.IntVar(&f.maxConcurrent, "max-concurrent", 50, "Maximum concurrent health checks")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Beacon - A beautiful service monitoring dashboard\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -config ./config.yaml -port 8080\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -config ./config.yaml -tls-cert cert.pem -tls-key key.pem\n", os.Args[0])
	}

	flag.Parse()
}

func openBrowser(url string) {
	var cmd string
	var args []string

	switch {
	case isWindows():
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case isMacOS():
		cmd = "open"
		args = []string{url}
	default: // Linux and others
		cmd = "xdg-open"
		args = []string{url}
	}

	// Don't worry if this fails
	if err := exec.Command(cmd, args...).Start(); err != nil {
		log.Printf("Failed to open browser: %v", err)
	}
}

func isWindows() bool {
	return os.Getenv("OS") == "Windows_NT"
}

func isMacOS() bool {
	return os.Getenv("GOOS") == "darwin"
}
