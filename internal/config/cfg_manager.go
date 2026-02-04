package config

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

var searchConfigPaths = []string{
	"",
	"config",
	"internal/config",
	"internal/config",
	"../internal/config",
	"../../internal/config",
}

// configManager manages multiple configurations with type safety using generics
type configManager[T any] struct {
	config     T
	configPath string
	mu         sync.RWMutex
	watcher    *fsnotify.Watcher
	validator  *validator.Validate
	strictMode bool // Enable strict YAML validation
}

// newConfigManager creates a new generic config manager
func newConfigManager[T any]() *configManager[T] {
	return &configManager[T]{
		validator:  validator.New(),
		strictMode: false, // Default: backward compatible (no strict mode)
	}
}

// WithStrictMode enables strict YAML validation (fails on unknown fields)
func (m *configManager[T]) WithStrictMode(enabled bool) *configManager[T] {
	m.strictMode = enabled
	return m
}

// Load loads configuration from YAML file with optional strict validation
func (m *configManager[T]) Load(filePath string) error {
	// Resolve absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path %s: %w", filePath, err)
	}

	// Check file exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("config file not found: %s", absPath)
	}

	// Read file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// YAML decode (strict mode optional)
	var newConfig T
	decoder := yaml.NewDecoder(bytes.NewReader(data))

	// Only enable strict mode if explicitly requested
	if m.strictMode {
		decoder.KnownFields(true) // Fail on unknown fields (typo detection)
	}

	if err := decoder.Decode(&newConfig); err != nil {
		if m.strictMode {
			return fmt.Errorf("failed to parse YAML (check for typos): %w", err)
		}
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate struct
	if err := m.validator.Struct(&newConfig); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Atomically update config
	m.mu.Lock()
	m.config = newConfig
	m.configPath = absPath
	m.mu.Unlock()

	return nil
}

// LoadWithEnv loads configuration based on environment (dev/prod)
// Searches in multiple common locations
func (m *configManager[T]) LoadWithEnv(baseName string, env string) error {
	configFileName := fmt.Sprintf("%s.%s.yaml", baseName, env)

	var searchPaths []string
	for _, prefix := range searchConfigPaths {
		searchPaths = append(searchPaths, filepath.Join(prefix, configFileName))
	}

	cwd, _ := os.Getwd()

	for _, relPath := range searchPaths {
		fullPath := filepath.Join(cwd, relPath)
		if _, err := os.Stat(fullPath); err == nil {
			return m.Load(fullPath)
		}
	}
	// Optional: turn on if you need to check executable directory
	// exePath, _ := os.Executable()
	// cad := filepath.Dir(exePath) // current app directory
	// for _, relPath := range searchPaths {
	// 	fullPath := filepath.Join(cad, relPath)
	// 	if _, err := os.Stat(fullPath); err == nil {
	// 		return m.Load(fullPath)
	// 	}
	// }

	return fmt.Errorf("no config file found for %s.%s.yaml in standard locations", baseName, env)
}

// Get returns immutable copy of config (thread-safe)
func (m *configManager[T]) Get() T {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config // Returns copy, not pointer
}

// GetPath returns the current config file path
func (m *configManager[T]) GetPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configPath
}

// Watch starts watching config file for changes and auto-reloads
func (m *configManager[T]) Watch(onReload func(T)) error {
	m.mu.RLock()
	path := m.configPath
	m.mu.RUnlock()

	if path == "" {
		return fmt.Errorf("no config loaded, cannot watch")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to watch file: %w", err)
	}

	m.watcher = watcher

	go func() {
		log.Printf("👀 Watching config: %s", path)

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op == fsnotify.Write {
					log.Println("📝 Config file changed, reloading...")
					time.Sleep(100 * time.Millisecond) // Debounce editor multi-writes

					if err := m.Load(path); err != nil {
						log.Printf("⚠️  Failed to reload config: %v", err)
					} else {
						log.Println("✅ Config reloaded successfully")
						if onReload != nil {
							onReload(m.Get())
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("⚠️  Config watcher error: %v", err)
			}
		}
	}()

	return nil
}

// Close stops the file watcher
func (m *configManager[T]) Close() error {
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}

// ===== HELPER FUNCTIONS =====

// loadConfigStrict loads any config type with strict validation
func loadConfigStrict[T any](filePath string) (*configManager[T], error) {
	mgr := newConfigManager[T]()
	if err := mgr.Load(filePath); err != nil {
		return nil, err
	}
	return mgr, nil
}

// loadConfigWithEnv loads config based on environment variable
func loadConfigWithEnv[T any](baseName string) (*configManager[T], error) {
	godotenv.Load()
	env := os.Getenv("ENV")
	if env == "" {
		env = os.Getenv("GO_ENV")
		switch env {
		case "dev":
			env = "dev"
		case "prod":
			env = "prod"
		}
	}
	if env == "" {
		cwd, _ := os.Getwd()
		for _, prefix := range searchConfigPaths {
			fullPath := filepath.Join(cwd, prefix, "conf.yaml")
			if _, err := os.Stat(fullPath); err != nil {
				continue
			}
			if prod, err := fileContainsProd(fullPath); err == nil && prod {
				env = "prod"
				break
			}
		}
	}
	if env == "" {
		env = "dev" // default
	}

	// fmt.Println("===============================================")
	// fmt.Println("ENV")
	// fmt.Println("ENV")
	// fmt.Println("ENV")
	// fmt.Println(env)
	// fmt.Println("ENV")
	// fmt.Println("ENV")
	// fmt.Println("ENV")
	// fmt.Println("ENV")
	// fmt.Println("===============================================")

	mgr := newConfigManager[T]()
	if err := mgr.LoadWithEnv(baseName, env); err != nil {
		return nil, err
	}
	return mgr, nil
}

func fileContainsProd(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), `"prod"`) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// ===== ERROR TYPES =====

// newError creates a new error with message
func newError(msg string) error {
	return &configError{Message: msg}
}

// configError represents a config-related error
type configError struct {
	Message string
}

func (e *configError) Error() string {
	return e.Message
}
