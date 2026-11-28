/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

// TypeMemory is the built-in provider: each process keeps the entries of its caches in memory.
const TypeMemory = "memory"

// Store is a shared cache store, which a provider supplies. Values arrive encoded, so a store
// can keep them outside the process. Each cache has its own namespace; a store keeps the keys of
// one namespace apart from those of another. A store must be safe for concurrent use.
type Store interface {
	// Get returns the value of the key, and false when there is none or it has expired.
	Get(ctx context.Context, namespace, key string) (value []byte, found bool, err error)
	// Set stores the value of the key until the TTL passes.
	Set(ctx context.Context, namespace, key string, value []byte, ttl time.Duration) error
	// Delete removes the key. Removing a key that is not there is not an error.
	Delete(ctx context.Context, namespace, key string) error
	// Clear removes every key of the namespace.
	Clear(ctx context.Context, namespace string) error
	// Close releases the connections of the store.
	Close() error
}

// Provider builds a Store from the cache configuration.
type Provider func(cfg config.CacheConfig) (Store, error)

// storeRef holds the active store, so that it can sit behind an atomic pointer.
type storeRef struct {
	Store
}

var (
	providersMu sync.RWMutex
	providers   = map[string]Provider{}

	// active is the shared store, or nil when each process keeps its own entries.
	active atomic.Pointer[storeRef]
)

// RegisterProvider registers a provider under the name that cache.type selects it by. Call it
// from an init function of the provider package, and import that package from the server.
func RegisterProvider(name string, p Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[name] = p
}

// Init selects the provider that cache.type names. An empty type, or "memory", keeps the entries
// of every cache in this process. An unknown type is an error, so that a misspelled provider
// stops the server rather than silently running without the shared store.
func Init(cfg config.CacheConfig) error {
	if cfg.Type == "" || cfg.Type == TypeMemory {
		return setStore(nil)
	}
	providersMu.RLock()
	p, ok := providers[cfg.Type]
	providersMu.RUnlock()
	if !ok {
		return fmt.Errorf("cache: unknown provider %q; register it by importing its package "+
			"(see docs/guides/extending-cache-providers.md)", cfg.Type)
	}
	s, err := p(cfg)
	if err != nil {
		return fmt.Errorf("cache: the %q provider failed to start: %w", cfg.Type, err)
	}
	if s == nil {
		return fmt.Errorf("cache: the %q provider returned no store", cfg.Type)
	}
	return setStore(s)
}

// Close closes the shared store, if there is one. Caches keep their entries in this process
// afterwards.
func Close() error {
	return setStore(nil)
}

// setStore makes s the shared store and closes the one it replaces.
func setStore(s Store) error {
	var next *storeRef
	if s != nil {
		next = &storeRef{Store: s}
	}
	if previous := active.Swap(next); previous != nil {
		return previous.Close()
	}
	return nil
}

// sharedStore returns the shared store, or nil when there is none.
func sharedStore() Store {
	if ref := active.Load(); ref != nil {
		return ref.Store
	}
	return nil
}
