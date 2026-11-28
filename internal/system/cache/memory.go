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
	"sync"
	"time"
)

// memoryEntry is a cached value and the time it expires.
type memoryEntry[V any] struct {
	value      V
	expiration time.Time
}

// memory keeps the entries of one cache in this process. It is safe for concurrent use.
type memory[V any] struct {
	items map[string]memoryEntry[V]
	mutex sync.RWMutex
	// sweepAt is the entry count that triggers a sweep of expired items on the next set.
	// It rises with the cache so a genuinely large working set is not swept on every write.
	sweepAt int
}

// initialSweepThreshold is the entry count at which a cache first sweeps expired items.
const initialSweepThreshold = 1024

func newMemory[V any]() *memory[V] {
	return &memory[V]{
		items:   make(map[string]memoryEntry[V]),
		sweepAt: initialSweepThreshold,
	}
}

func (m *memory[V]) set(key string, value V, ttl time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.items[key] = memoryEntry[V]{value: value, expiration: time.Now().Add(ttl)}

	// An expired entry is invisible to get but still occupies the map, so a cache keyed by
	// something high-cardinality — an email address, say — would otherwise grow for the
	// life of the process and never shrink. Sweeping on a size trigger keeps that bounded
	// without a background goroutine that would outlive the cache.
	if len(m.items) >= m.sweepAt {
		m.sweepExpiredLocked()
	}
}

// sweepExpiredLocked drops every expired entry. The caller must hold the write lock.
func (m *memory[V]) sweepExpiredLocked() {
	now := time.Now()
	for key, item := range m.items {
		if now.After(item.expiration) {
			delete(m.items, key)
		}
	}

	// If sweeping freed little, the live set really is this large; raise the bar so the
	// next sweep is not attempted on every subsequent write.
	if next := len(m.items) * 2; next > m.sweepAt {
		m.sweepAt = next
	} else if initialSweepThreshold > m.sweepAt {
		m.sweepAt = initialSweepThreshold
	}
}

func (m *memory[V]) get(key string) (V, bool) {
	m.mutex.RLock()
	item, found := m.items[key]
	m.mutex.RUnlock()

	var zero V
	if !found {
		return zero, false
	}
	if time.Now().After(item.expiration) {
		m.mutex.Lock()
		// Re-check: another writer may have refreshed the entry while the lock was released.
		if current, stillThere := m.items[key]; stillThere && time.Now().After(current.expiration) {
			delete(m.items, key)
		}
		m.mutex.Unlock()
		return zero, false
	}
	return item.value, true
}

// len reports how many entries the cache currently holds, expired ones included.
func (m *memory[V]) len() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.items)
}

func (m *memory[V]) delete(key string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.items, key)
}

func (m *memory[V]) clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.items = make(map[string]memoryEntry[V])
	m.sweepAt = initialSweepThreshold
}
