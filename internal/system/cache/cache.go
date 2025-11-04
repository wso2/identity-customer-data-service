/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
		logger.Debug(fmt.Sprint("Cache not found for key: ", key))
		return nil, false
	}
	// Check if expired
	if time.Now().After(item.Expiration) {
		logger.Debug(fmt.Sprint("Cache expired for key: ", key))
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
