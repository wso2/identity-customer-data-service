package cache

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"sync"
	"time"
)

type CacheItem struct {
	Value      interface{}
	Expiration time.Time
}

type Cache struct {
	items map[string]CacheItem
	mutex sync.RWMutex
	ttl   time.Duration
}

// NewCache creates a new cache with a TTL (time-to-live)
func NewCache(defaultTTL time.Duration) *Cache {
	return &Cache{
		items: make(map[string]CacheItem),
		ttl:   defaultTTL,
	}
}

// Set adds an item to the cache
func (c *Cache) Set(key string, value interface{}) {

	logger := log.GetLogger()
	logger.Debug(fmt.Sprint("Setting cache for key: ", key))
	c.mutex.Lock()
	defer c.mutex.Unlock()

	expiration := time.Now().Add(c.ttl)
	c.items[key] = CacheItem{
		Value:      value,
		Expiration: expiration,
	}
}

// Get retrieves an item from the cache
func (c *Cache) Get(key string) (interface{}, bool) {

	logger := log.GetLogger()
	logger.Debug(fmt.Sprint("Getting cache for key: ", key))
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, found := c.items[key]
	if !found {
		return nil, false
	}

	// Check if expired
	if time.Now().After(item.Expiration) {
		return nil, false
	}

	return item.Value, true
}

// Delete removes an item from the cache
func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.items, key)
}
