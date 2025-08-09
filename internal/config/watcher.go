package config

import (
	"log"
	"path/filepath"
	"time"

	"github.com/LysanderdeJong/beacon/internal/constants"
	"github.com/fsnotify/fsnotify"
)

// Watcher manages configuration file watching and reloading
type Watcher struct {
	configPath string
	fsWatcher  *fsnotify.Watcher
	reloadChan chan *Config
	errorChan  chan error
	done       chan struct{}
}

// NewWatcher creates a new configuration file watcher
func NewWatcher(configPath string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		configPath: configPath,
		fsWatcher:  fsWatcher,
		reloadChan: make(chan *Config, 1),
		errorChan:  make(chan error, 1),
		done:       make(chan struct{}),
	}

	// Watch the config file
	err = fsWatcher.Add(configPath)
	if err != nil {
		fsWatcher.Close()
		return nil, err
	}

	// Also watch the directory in case the file is replaced (common with editors)
	dir := filepath.Dir(configPath)
	err = fsWatcher.Add(dir)
	if err != nil {
		log.Printf("Warning: couldn't watch config directory %s: %v", dir, err)
	}

	return w, nil
}

// Start starts watching for configuration file changes
func (w *Watcher) Start() {
	go w.watchLoop()
}

// Stop stops the configuration file watcher
func (w *Watcher) Stop() {
	close(w.done)
	w.fsWatcher.Close()
}

// ReloadChan returns a channel that receives new configurations when the file changes
func (w *Watcher) ReloadChan() <-chan *Config {
	return w.reloadChan
}

// ErrorChan returns a channel that receives errors from file watching
func (w *Watcher) ErrorChan() <-chan error {
	return w.errorChan
}

// watchLoop is the main file watching loop
func (w *Watcher) watchLoop() {
	// Debounce rapid file changes (editors often write multiple times)
	var debounceTimer *time.Timer
	debounceDelay := constants.ConfigDebounceDelay

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// Only handle events for our config file
			if !w.isConfigFileEvent(event) {
				continue
			}

			// Handle write and create events (not remove or chmod)
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("Config file changed: %s", event.Name)

				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDelay, func() {
					w.reloadConfig()
				})
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}

			log.Printf("Config file watcher error: %v", err)
			select {
			case w.errorChan <- err:
			default:
			}

		case <-w.done:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return
		}
	}
}

// isConfigFileEvent checks if the event is for our config file
func (w *Watcher) isConfigFileEvent(event fsnotify.Event) bool {
	// Get absolute paths for comparison
	configAbs, err := filepath.Abs(w.configPath)
	if err != nil {
		log.Printf("Warning: couldn't get absolute path for config: %v", err)
		return false
	}

	eventAbs, err := filepath.Abs(event.Name)
	if err != nil {
		log.Printf("Warning: couldn't get absolute path for event: %v", err)
		return false
	}

	return configAbs == eventAbs
}

// reloadConfig attempts to reload the configuration file
func (w *Watcher) reloadConfig() {
	// Add a small delay to ensure file write is complete
	time.Sleep(constants.ConfigReloadDelay)

	config, err := LoadConfig(w.configPath)
	if err != nil {
		log.Printf("Failed to reload config: %v", err)
		select {
		case w.errorChan <- err:
		default:
		}
		return
	}

	log.Printf("Successfully reloaded configuration from %s", w.configPath)

	// Send the new config (non-blocking)
	select {
	case w.reloadChan <- config:
	default:
		log.Printf("Warning: reload channel is full, skipping config update")
	}
}
