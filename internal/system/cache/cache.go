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

// Package cache is the cache layer of CDD. A Cache is typed and named, and gives its entries a
// time-to-live. By default each process keeps the entries of its caches in memory. A deployment
// can plug in a shared store instead, for example one that every replica reaches, by
// registering a provider (see RegisterProvider) and naming it in cache.type.
//
// A cache is never the source of truth: when the shared store fails, a read is a miss and the
// caller goes to the database.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// Option configures a cache.
type Option func(*options)

type options struct {
	local bool
}

// Local keeps the entries of the cache in this process even when a shared store is configured.
// Use it for a cache that is read on a hot path, where a network round trip would cost more than
// the read it saves, or whose values must not leave the process.
func Local() Option {
	return func(o *options) { o.local = true }
}

// Cache is a typed cache with one TTL for every entry. It is safe for concurrent use.
//
// With a shared store, values are stored as JSON, so V must survive a JSON round trip: only its
// exported fields are kept.
type Cache[V any] struct {
	// name is the namespace of the cache in a shared store. Two caches with one name share
	// their entries there, so each cache needs a name of its own.
	name   string
	ttl    time.Duration
	local  bool
	memory *memory[V]
}

// New creates a cache whose entries live for ttl. name identifies the cache in a shared store.
func New[V any](name string, ttl time.Duration, opts ...Option) *Cache[V] {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return &Cache[V]{name: name, ttl: ttl, local: o.local, memory: newMemory[V]()}
}

// store returns the shared store the cache uses, or nil when it keeps its entries in memory.
func (c *Cache[V]) store() Store {
	if c.local {
		return nil
	}
	return sharedStore()
}

// Get returns the value of the key, and false when there is none or it has expired.
func (c *Cache[V]) Get(ctx context.Context, key string) (V, bool) {
	var zero V
	s := c.store()
	if s == nil {
		return c.memory.get(key)
	}
	data, found, err := s.Get(ctx, c.name, key)
	if err != nil {
		c.warn("read from", err)
		return zero, false
	}
	if !found {
		return zero, false
	}
	var value V
	if err := json.Unmarshal(data, &value); err != nil {
		c.warn("decode an entry of", err)
		return zero, false
	}
	return value, true
}

// Set stores the value of the key for the TTL of the cache.
func (c *Cache[V]) Set(ctx context.Context, key string, value V) {
	s := c.store()
	if s == nil {
		c.memory.set(key, value, c.ttl)
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		c.warn("encode an entry of", err)
		return
	}
	if err := s.Set(ctx, c.name, key, data, c.ttl); err != nil {
		c.warn("write to", err)
	}
}

// Delete removes the key. It goes ahead even when ctx is cancelled, since an invalidation that is
// dropped leaves a stale entry for the rest of its TTL.
func (c *Cache[V]) Delete(ctx context.Context, key string) {
	s := c.store()
	if s == nil {
		c.memory.delete(key)
		return
	}
	ctx, cancel := invalidationContext(ctx)
	defer cancel()
	if err := s.Delete(ctx, c.name, key); err != nil {
		c.invalidationFailed(err)
	}
}

// Clear removes every entry. Used where a write invalidates a whole cached namespace and
// threading the affected keys through the write path would cost more than re-reading. Like
// Delete, it goes ahead even when ctx is cancelled.
func (c *Cache[V]) Clear(ctx context.Context) {
	s := c.store()
	if s == nil {
		c.memory.clear()
		return
	}
	ctx, cancel := invalidationContext(ctx)
	defer cancel()
	if err := s.Clear(ctx, c.name); err != nil {
		c.invalidationFailed(err)
	}
}

// invalidationContext keeps the values of ctx but not its cancellation, and bounds the call
// instead, so that a store that does not answer cannot hold the write that triggered it.
func invalidationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), constants.CacheInvalidationTimeout)
}

func (c *Cache[V]) warn(action string, err error) {
	log.GetLogger().Warn(fmt.Sprintf("Could not %s the %s cache; using the database instead.", action, c.name),
		log.Error(err))
}

func (c *Cache[V]) invalidationFailed(err error) {
	log.GetLogger().Error(fmt.Sprintf("Could not invalidate an entry of the %s cache; it may be served for up to %s.",
		c.name, c.ttl), log.Error(err))
}
