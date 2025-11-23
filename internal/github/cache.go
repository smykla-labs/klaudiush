// Package github provides GitHub API client with caching
package github

import (
	"sync"
	"time"
)

const (
	// defaultTTL is the default time-to-live for cache entries (15 minutes)
	defaultTTL = 15 * time.Minute
)

// cacheEntry represents a cached value with expiration time
type cacheEntry struct {
	value      any
	expiration time.Time
}

// Cache provides thread-safe caching with TTL
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

// NewCache creates a new Cache with default TTL
func NewCache() *Cache {
	return &Cache{
		entries: make(map[string]*cacheEntry),
		ttl:     defaultTTL,
	}
}

// Get retrieves a value from cache if it exists and hasn't expired
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiration) {
		return nil, false
	}

	return entry.value, true
}

// Set stores a value in cache with the default TTL
func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &cacheEntry{
		value:      value,
		expiration: time.Now().Add(c.ttl),
	}
}

// Clear removes all entries from cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// CleanExpired removes all expired entries from cache
func (c *Cache) CleanExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiration) {
			delete(c.entries, key)
		}
	}
}
