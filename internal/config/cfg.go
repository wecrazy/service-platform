// Package config provides configuration loading and management for the service-platform application.
package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
)

// searchConfigPaths defines the potential paths where the configuration file might be located.
var searchConfigPaths = []string{
	"",
	"config",
	"internal/config",
	"../internal/config",
	"../../internal/config",
}

// configs is a generic struct that holds the configuration data and manages its loading and hot-reloading.
// It uses a configManager to handle the actual loading and watching of the configuration file, and a mutex to ensure thread-safe access to the configuration data.
// The once field ensures that the configuration is loaded only once, even if multiple goroutines attempt to access it simultaneously.
// T is a generic type parameter that allows this struct to be used with any configuration struct defined by the user.
type configs[T any] struct {
	mgr  *configManager[T] // Manager to handle configuration loading and hot-reloading
	data T                 // The actual configuration data
	mu   sync.RWMutex      // Mutex to protect concurrent access to the configuration data
	once sync.Once         // Ensures the configuration is loaded only once
}

// configManager is responsible for loading the configuration from a file, validating it, and setting up hot-reloading using fsnotify.
type configManager[T any] struct {
	config     T                   // The actual configuration struct
	configPath string              // Path to the configuration file
	mu         sync.RWMutex        // Mutex to protect concurrent access to the configuration
	watcher    *fsnotify.Watcher   // File watcher for hot-reloading
	validator  *validator.Validate // YAML validation
	strictMode bool                // Enable strict YAML validation
}

// newConfigManager creates a new instance of configManager with default settings. It initializes the YAML validator and sets strictMode to false for backward compatibility. The returned configManager can be used to load and manage configuration data of any type specified by the generic parameter T.
func newConfigManager[T any]() *configManager[T] {
	return &configManager[T]{
		validator:  validator.New(),
		strictMode: false, // Default: backward compatible (no strict mode)
	}
}

// Init initializes configs with custom file name (auto-detects environment)
func (c *configs[T]) Init(baseName string) error {
	var err error
	c.once.Do(func() {
		c.mgr, err = loadConfigWithEnv[T](baseName)
		if err == nil {
			c.update(c.mgr.Get())
			log.Printf("✅ Config '%s' initialized", baseName)
		}
	})
	return err
}

// Load loads configs from specific file path
func (c *configs[T]) Load(filePath string) error {
	var err error
	c.once.Do(func() {
		c.mgr, err = loadConfigStrict[T](filePath)
		if err == nil {
			c.update(c.mgr.Get())
			log.Printf("✅ Config loaded from '%s'", filePath)
		}
	})
	return err
}

// Load loads configuration from YAML file with optional strict validation
// It resolves the absolute path, checks if the file exists, reads the file, parses the YAML content into the config struct, validates it, and then updates the config atomically. If any step fails, it returns an appropriate error message.
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

	// YAML parse
	var newConfig T
	decodeOpts := []yaml.DecodeOption{}

	// goccy/go-yaml doesn't support KnownFields like yaml.v3, but we can validate after decode
	if err := yaml.UnmarshalWithOptions(data, &newConfig, decodeOpts...); err != nil {
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

// loadConfigStrict loads the configuration from a specified file path with strict validation enabled. It creates a new configManager instance, calls the Load method to load and validate the configuration, and returns the configManager instance if successful. If any error occurs during loading or validation, it returns the error.
func loadConfigStrict[T any](filePath string) (*configManager[T], error) {
	mgr := newConfigManager[T]()
	if err := mgr.Load(filePath); err != nil {
		return nil, err
	}
	return mgr, nil
}

// LoadWithEnv loads configuration based on environment (dev/prod)
// It constructs the config file name using the base name and environment, then searches for it in predefined paths. If found, it loads the configuration using the Load method.
// If no config file is found for the specified environment, it returns an error indicating that the config file could not be found in the standard locations.
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

	return fmt.Errorf("no config file found for %s.%s.yaml in standard locations", baseName, env)
}

// loadConfigWithEnv loads the configuration based on environment variables. It first attempts to load environment variables from a .env file, then checks for ENV or GO_ENV to determine the environment (dev/prod). If no environment variable is set, it tries to detect the environment by checking if any config file contains "prod". Finally, it loads the configuration using the determined environment and returns a configManager instance.
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

	mgr := newConfigManager[T]()
	if err := mgr.LoadWithEnv(baseName, env); err != nil {
		return nil, err
	}
	return mgr, nil
}

// fileContainsProd checks if the given file contains the string "prod". It reads the file line by line and returns true if it finds a line containing "prod". If it encounters any error while opening or reading the file, it returns false along with the error.
// This function is used to help determine the environment (dev/prod) based on the content of the config file when environment variables are not set.
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

// update safely updates the configuration data in the configs struct. It acquires a write lock on the mutex to ensure that no other goroutine can read or write the configuration data while it is being updated. Once the new configuration data is set, it releases the lock, allowing other goroutines to access the updated configuration.
func (c *configs[T]) update(newData T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = newData
}

// Get returns the current configuration data. It acquires a read lock on the mutex to ensure that the configuration data is not being modified by another goroutine while it is being read. Once it retrieves the configuration data, it releases the lock and returns the data. This method provides thread-safe access to the configuration data.
func (m *configManager[T]) Get() T {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config // Returns copy, not pointer
}

// GetPath returns the path of the currently loaded configuration file. It acquires a read lock on the mutex to ensure that the configPath is not being modified by another goroutine while it is being read. Once it retrieves the configPath, it releases the lock and returns the path as a string. This method provides thread-safe access to the configuration file path.
func (m *configManager[T]) GetPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configPath
}

// Watch sets up a file watcher on the currently loaded configuration file to enable hot-reloading. It first checks if a configuration file is loaded by acquiring a read lock and checking the configPath. If no config is loaded, it returns an error. If a config file is found, it creates a new fsnotify watcher and adds the config file to the watch list. It then starts a goroutine that listens for file change events. When a write event is detected, it attempts to reload the configuration by calling the Load method. If the reload is successful, it calls the onReload callback with the new configuration data. If any errors occur during watching or reloading, they are logged appropriately.
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
		log.Printf("👀 Watching config: %s\n", path)

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op == fsnotify.Write {
					log.Printf("📝 Detected change in config file: %s\n", event.Name)
					time.Sleep(100 * time.Millisecond) // Debounce editor multi-writes

					if err := m.Load(path); err != nil {
						log.Printf("⚠️  Failed to reload config: %v\n", err)
					} else {
						log.Printf("✅ Config reloaded successfully: %s\n", path)
						if onReload != nil {
							onReload(m.Get())
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("⚠️  Config watcher error: %v\n", err)
			}
		}
	}()

	return nil
}

// Close stops the file watcher. It acquires a write lock on the mutex to ensure that no other goroutine is accessing the watcher while it is being closed. Once the watcher is closed, it releases the lock. This method provides thread-safe access to stop the file watcher.
func (m *configManager[T]) Close() error {
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}

// newError creates a new configError with the provided message. It returns an error interface that can be used to represent configuration-related errors in a consistent manner throughout the application.
func newError(msg string) error {
	return &configError{Message: msg}
}

// configError is a custom error type that implements the error interface. It contains a Message field that holds the error message. The Error method returns the error message when the error is printed or logged. This custom error type can be used to provide more specific and descriptive error messages related to configuration issues in the application.
type configError struct {
	Message string
}

// Error returns the error message contained in the configError struct. It satisfies the error interface, allowing instances of configError to be used as standard errors in Go. When this method is called, it simply returns the Message field of the configError, which contains a descriptive error message about the configuration issue that occurred.
func (e *configError) Error() string {
	return e.Message
}

// MustInit initializes configs with custom file name and panics on error (use in main only)
func (c *configs[T]) MustInit(baseName string) {
	if err := c.Init(baseName); err != nil {
		log.Fatalf("Failed to init configs '%s': %v", baseName, err)
	}
}

// MustLoad loads or panics (use in main only)
func (c *configs[T]) MustLoad(filePath string) {
	if err := c.Load(filePath); err != nil {
		log.Fatalf("Failed to load configs '%s': %v", filePath, err)
	}
}

// Get returns the current configuration data. It acquires a read lock on the mutex to ensure that the configuration data is not being modified by another goroutine while it is being read. Once it retrieves the configuration data, it releases the lock and returns the data. This method provides thread-safe access to the configuration data.
func (c *configs[T]) Get() T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data
}

// Watch starts watching for config changes and calls the provided callback with new config data on each change. It returns an error if the configs are not initialized (i.e., if no config file has been loaded). This method allows the application to react to configuration changes in real-time by providing a callback function that will be called whenever the configuration file is modified.
func (c *configs[T]) Watch() error {
	if c.mgr == nil {
		return ErrNotInitialized
	}

	return c.mgr.Watch(func(newData T) {
		c.update(newData)
		log.Printf("🔄 Config reloaded: %s\n", c.mgr.GetPath())
	})
}

// Close stops watching for config changes and releases resources
func (c *configs[T]) Close() error {
	if c.mgr == nil {
		return nil
	}
	return c.mgr.Close()
}

// Path returns the path of the currently loaded config file. If no config is loaded, it returns an empty string. This method provides a way to retrieve the file path of the configuration being used, which can be useful for logging or debugging purposes.
func (c *configs[T]) Path() string {
	if c.mgr == nil {
		return ""
	}
	return c.mgr.GetPath()
}

// IsLoaded checks if the configs have been loaded by verifying if the config manager is initialized. It returns true if the config manager is not nil, indicating that the configuration has been successfully loaded, and false otherwise. This method can be used to check if the configuration is ready before attempting to access it or watch for changes.
func (c *configs[T]) IsLoaded() bool {
	return c.mgr != nil
}

// ErrNotInitialized is returned when trying to watch configs before they are loaded
var ErrNotInitialized = newError("configs not initialized")
