package config

import (
	"log"
	"sync"
)

// ===== GLOBAL CONFIG WRAPPER =====
// Config wraps configManager to provide simple method-based access.
// Config wraps configManager with convenient methods
type configs[T any] struct {
	mgr  *configManager[T]
	data T
	mu   sync.RWMutex
	once sync.Once
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

// MustInit initializes or panics (use in main only)
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

// Get returns thread-safe copy of configs
func (c *configs[T]) Get() T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data
}

// Watch starts watching for configs changes (run in goroutine)
func (c *configs[T]) Watch() error {
	if c.mgr == nil {
		return ErrNotInitialized
	}

	return c.mgr.Watch(func(newData T) {
		c.update(newData)
		log.Println("🔄 Config reloaded")
	})
}

// Close stops the configs watcher
func (c *configs[T]) Close() error {
	if c.mgr == nil {
		return nil
	}
	return c.mgr.Close()
}

// Path returns the configs file path
func (c *configs[T]) Path() string {
	if c.mgr == nil {
		return ""
	}
	return c.mgr.GetPath()
}

// IsLoaded checks if configs has been initialized
func (c *configs[T]) IsLoaded() bool {
	return c.mgr != nil
}

// update atomically updates configs data
func (c *configs[T]) update(newData T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = newData
}

// ErrNotInitialized is returned when configs is accessed before initialization
var ErrNotInitialized = newError("configs not initialized")
