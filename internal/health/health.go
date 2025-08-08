// Package health provides concurrent health checking for services in Beacon.
// Main types: Checker, Worker.
package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/LysanderdeJong/beacon/internal/config"
	"github.com/LysanderdeJong/beacon/internal/store"
)

// Checker manages health checks for all services
type Checker struct {
	store      *store.Store
	httpClient *http.Client
	workers    map[string]*Worker
	workersMu  sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	semaphore  chan struct{} // Limits concurrent health checks
}

// Worker represents a health check worker for a single service
type Worker struct {
	service *config.Service
	checker *Checker
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewChecker creates a new health checker
func NewChecker(store *store.Store, maxConcurrent int) *Checker {
	ctx, cancel := context.WithCancel(context.Background())

	return &Checker{
		store:     store,
		workers:   make(map[string]*Worker),
		ctx:       ctx,
		cancel:    cancel,
		semaphore: make(chan struct{}, maxConcurrent),
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Default timeout, will be overridden per request
		},
	}
}

// Start starts health checking for all services
func (c *Checker) Start(services []config.Service) {
	c.workersMu.Lock()
	defer c.workersMu.Unlock()

	// Stop existing workers
	for _, worker := range c.workers {
		worker.cancel()
	}
	c.workers = make(map[string]*Worker)

	// Start new workers for each service
	for _, service := range services {
		// Initialize service state
		c.store.InitializeService(service.ID)

		// Create and start worker
		worker := c.createWorker(service)
		c.workers[service.ID] = worker
		go worker.run()
	}
}

// Stop stops all health checking workers
func (c *Checker) Stop() {
	c.cancel()

	c.workersMu.Lock()
	defer c.workersMu.Unlock()

	for _, worker := range c.workers {
		worker.cancel()
	}
	c.workers = make(map[string]*Worker)
}

// RestartService restarts health checking for a specific service
func (c *Checker) RestartService(service config.Service) {
	c.workersMu.Lock()
	defer c.workersMu.Unlock()

	// Stop existing worker for this service
	if worker, exists := c.workers[service.ID]; exists {
		worker.cancel()
	}

	// Initialize service state
	c.store.InitializeService(service.ID)

	// Create and start new worker
	worker := c.createWorker(service)
	c.workers[service.ID] = worker
	go worker.run()
}

// createWorker creates a new worker for a service
func (c *Checker) createWorker(service config.Service) *Worker {
	ctx, cancel := context.WithCancel(c.ctx)

	return &Worker{
		service: &service,
		checker: c,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// run executes the health check loop for a worker
func (w *Worker) run() {
	ticker := time.NewTicker(w.service.Health.Interval)
	defer ticker.Stop()

	// Perform initial check immediately
	w.performCheck()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.performCheck()
		}
	}
}

// performCheck performs a single health check
func (w *Worker) performCheck() {
	// Acquire semaphore to limit concurrent checks
	select {
	case w.checker.semaphore <- struct{}{}:
		defer func() { <-w.checker.semaphore }()
	case <-w.ctx.Done():
		return
	}

	startTime := time.Now()
	status, httpStatus, err := w.checkService()
	responseTime := time.Since(startTime)

	state := &store.ServiceState{
		ID:             w.service.ID,
		Status:         status,
		LastChecked:    time.Now().UTC(),
		ResponseTimeMs: responseTime.Milliseconds(),
		HTTPStatus:     httpStatus,
	}

	if err != nil {
		state.Error = err.Error()
	}

	w.checker.store.UpdateServiceState(state)
}

// checkService performs the actual HTTP health check with retries
func (w *Worker) checkService() (store.ServiceStatus, int, error) {
	var lastErr error
	var httpStatus int

	for attempt := 0; attempt <= w.service.Health.Retries; attempt++ {
		if attempt > 0 {
			// Add jitter to retry delay
			delay := time.Duration(attempt) * time.Second
			select {
			case <-time.After(delay):
			case <-w.ctx.Done():
				return store.StatusDown, httpStatus, w.ctx.Err()
			}
		}

		status, code, err := w.performSingleCheck()
		httpStatus = code

		if err == nil {
			// Success - determine status based on response time and HTTP status
			return w.determineStatus(status, code), code, nil
		}

		lastErr = err
	}

	return store.StatusDown, httpStatus, lastErr
}

// performSingleCheck performs a single HTTP request
func (w *Worker) performSingleCheck() (store.ServiceStatus, int, error) {
	ctx, cancel := context.WithTimeout(w.ctx, w.service.Health.Timeout)
	defer cancel()

	// Use Health.Endpoint if configured, otherwise fall back to service URL
	url := w.service.Health.Endpoint
	if url == "" {
		url = w.service.URL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return store.StatusDown, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Add custom headers
	for key, value := range w.service.Health.Headers {
		req.Header.Set(key, value)
	}

	// Set user agent
	req.Header.Set("User-Agent", "Beacon/1.0 Health-Checker")

	// Configure redirect policy
	client := w.checker.httpClient
	if !w.service.Health.FollowRedirects {
		client = &http.Client{
			Timeout: w.service.Health.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return store.StatusDown, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return store.StatusUp, resp.StatusCode, nil
}

// determineStatus determines the service status based on HTTP response
func (w *Worker) determineStatus(status store.ServiceStatus, httpStatus int) store.ServiceStatus {
	// Default expected status to 200 if not configured
	expectedStatus := w.service.Health.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	// Check if HTTP status matches expected
	if httpStatus != expectedStatus {
		return store.StatusDown
	}

	// TODO: Add logic for degraded status based on response time patterns
	// For now, we consider it up if status matches
	return store.StatusUp
}
