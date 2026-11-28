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
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func TestMain(m *testing.M) {
	_ = log.Init("error")
	m.Run()
}

// fakeStore is a shared store in a map, which can be made to fail.
type fakeStore struct {
	mu      sync.Mutex
	entries map[string][]byte
	ttls    map[string]time.Duration
	err     error
	closed  int
	// lastCtxErr is the error of the context of the last call, to tell whether it was cancelled.
	lastCtxErr error
	// lastDeadline is the deadline of the context of the last call, and hasDeadline whether it had one.
	lastDeadline time.Time
	hasDeadline  bool
}

// record notes the context of a call. The caller holds the lock.
func (f *fakeStore) record(ctx context.Context) {
	f.lastCtxErr = ctx.Err()
	f.lastDeadline, f.hasDeadline = ctx.Deadline()
}

func newFakeStore() *fakeStore {
	return &fakeStore{entries: map[string][]byte{}, ttls: map[string]time.Duration{}}
}

func (f *fakeStore) Get(ctx context.Context, namespace, key string) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(ctx)
	if f.err != nil {
		return nil, false, f.err
	}
	value, ok := f.entries[namespace+"/"+key]
	return value, ok, nil
}

func (f *fakeStore) Set(ctx context.Context, namespace, key string, value []byte, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(ctx)
	if f.err != nil {
		return f.err
	}
	f.entries[namespace+"/"+key] = value
	f.ttls[namespace+"/"+key] = ttl
	return nil
}

func (f *fakeStore) Delete(ctx context.Context, namespace, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(ctx)
	if f.err != nil {
		return f.err
	}
	delete(f.entries, namespace+"/"+key)
	return nil
}

func (f *fakeStore) Clear(ctx context.Context, namespace string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(ctx)
	if f.err != nil {
		return f.err
	}
	for key := range f.entries {
		if strings.HasPrefix(key, namespace+"/") {
			delete(f.entries, key)
		}
	}
	return nil
}

func (f *fakeStore) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
	return nil
}

// useFakeStore registers a provider that returns a new fake store, selects it, and puts the
// in-memory default back when the test ends.
func useFakeStore(t *testing.T) *fakeStore {
	t.Helper()
	store := newFakeStore()
	RegisterProvider("fake", func(config.CacheConfig) (Store, error) { return store, nil })
	if err := Init(config.CacheConfig{Type: "fake"}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
	return store
}

type ref struct {
	ID     string
	Handle string
}

func TestMemoryIsTheDefault(t *testing.T) {
	for _, typ := range []string{"", TypeMemory} {
		if err := Init(config.CacheConfig{Type: typ}); err != nil {
			t.Fatalf("Init(%q): %v", typ, err)
		}
		if sharedStore() != nil {
			t.Errorf("Init(%q) selected a shared store", typ)
		}
	}

	ctx := context.Background()
	c := New[ref]("test.refs", time.Hour)
	c.Set(ctx, "acme", ref{ID: "1", Handle: "acme"})
	if got, ok := c.Get(ctx, "acme"); !ok || got.ID != "1" {
		t.Errorf("Get = %v, %v; want the stored ref", got, ok)
	}
	c.Delete(ctx, "acme")
	if _, ok := c.Get(ctx, "acme"); ok {
		t.Error("a deleted key is still served")
	}
}

func TestInitRefusesAnUnknownProvider(t *testing.T) {
	err := Init(config.CacheConfig{Type: "no-such-provider"})
	if err == nil || !strings.Contains(err.Error(), "no-such-provider") {
		t.Fatalf("Init = %v; want an error naming the provider", err)
	}
}

func TestInitReportsAProviderThatFails(t *testing.T) {
	RegisterProvider("broken", func(config.CacheConfig) (Store, error) { return nil, errors.New("no route") })
	if err := Init(config.CacheConfig{Type: "broken"}); err == nil || !strings.Contains(err.Error(), "no route") {
		t.Fatalf("Init = %v; want the provider's error", err)
	}
	RegisterProvider("empty", func(config.CacheConfig) (Store, error) { return nil, nil })
	if err := Init(config.CacheConfig{Type: "empty"}); err == nil {
		t.Fatal("Init accepted a provider that returned no store")
	}
}

func TestInitPassesTheProperties(t *testing.T) {
	var got map[string]string
	RegisterProvider("props", func(cfg config.CacheConfig) (Store, error) {
		got = cfg.Properties
		return newFakeStore(), nil
	})
	if err := Init(config.CacheConfig{Type: "props", Properties: map[string]string{"addr": "cache:6379"}}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
	if got["addr"] != "cache:6379" {
		t.Errorf("the provider got properties %v", got)
	}
}

func TestSharedStoreHoldsTheEntries(t *testing.T) {
	store := useFakeStore(t)
	ctx := context.Background()

	c := New[[]ref]("test.rules", 30*time.Second)
	c.Set(ctx, "org-1", []ref{{ID: "1", Handle: "acme"}})

	if _, ok := store.entries["test.rules/org-1"]; !ok {
		t.Fatalf("the entry is not in the shared store: %v", store.entries)
	}
	if store.ttls["test.rules/org-1"] != 30*time.Second {
		t.Errorf("the store got TTL %s, want the TTL of the cache", store.ttls["test.rules/org-1"])
	}

	// Another cache with the same name, as on another replica, reads the entry.
	other := New[[]ref]("test.rules", 30*time.Second)
	got, ok := other.Get(ctx, "org-1")
	if !ok || len(got) != 1 || got[0].Handle != "acme" {
		t.Fatalf("Get = %v, %v; want the entry written by the other cache", got, ok)
	}

	// An invalidation through one is seen by the other.
	c.Clear(ctx)
	if _, ok := other.Get(ctx, "org-1"); ok {
		t.Error("a cleared entry is still served")
	}
}

func TestLocalCacheStaysInMemory(t *testing.T) {
	store := useFakeStore(t)
	ctx := context.Background()

	c := New[int]("test.rarity", time.Hour, Local())
	c.Set(ctx, "k", 7)
	if len(store.entries) != 0 {
		t.Fatalf("a local cache wrote to the shared store: %v", store.entries)
	}
	if got, ok := c.Get(ctx, "k"); !ok || got != 7 {
		t.Errorf("Get = %v, %v; want 7", got, ok)
	}
}

func TestAFailingStoreIsAMiss(t *testing.T) {
	store := useFakeStore(t)
	ctx := context.Background()

	c := New[string]("test.failing", time.Hour)
	c.Set(ctx, "k", "v")
	store.err = errors.New("connection refused")

	if _, ok := c.Get(ctx, "k"); ok {
		t.Error("a read that failed was served as a hit")
	}
	// Writes and invalidations that fail do not panic or block the caller.
	c.Set(ctx, "k", "w")
	c.Delete(ctx, "k")
	c.Clear(ctx)
}

func TestAnUndecodableEntryIsAMiss(t *testing.T) {
	store := useFakeStore(t)
	ctx := context.Background()

	store.entries["test.decode/k"] = []byte("not json")
	c := New[ref]("test.decode", time.Hour)
	if _, ok := c.Get(ctx, "k"); ok {
		t.Error("an entry that does not decode was served as a hit")
	}
}

func TestInvalidationOutlivesACancelledRequest(t *testing.T) {
	store := useFakeStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := New[string]("test.cancel", time.Hour)
	c.Delete(ctx, "k")
	if store.lastCtxErr != nil {
		t.Errorf("Delete passed on a cancelled context: %v", store.lastCtxErr)
	}
	c.Clear(ctx)
	if store.lastCtxErr != nil {
		t.Errorf("Clear passed on a cancelled context: %v", store.lastCtxErr)
	}
}

// TestInvalidationIsBounded guards against an invalidation that waits on a store that does not
// answer for as long as it likes: dropping the cancellation of the request must not drop a bound.
func TestInvalidationIsBounded(t *testing.T) {
	store := useFakeStore(t)
	c := New[string]("test.bounded", time.Hour)

	for name, invalidate := range map[string]func(){
		"Delete": func() { c.Delete(context.Background(), "k") },
		"Clear":  func() { c.Clear(context.Background()) },
	} {
		t.Run(name, func(t *testing.T) {
			before := time.Now()
			invalidate()
			if !store.hasDeadline {
				t.Fatal("the store got a context with no deadline")
			}
			limit := before.Add(constants.CacheInvalidationTimeout)
			if store.lastDeadline.After(limit.Add(time.Second)) {
				t.Errorf("deadline %s is later than the invalidation timeout allows (%s)", store.lastDeadline, limit)
			}
		})
	}
}

func TestCloseClosesTheStoreAndFallsBackToMemory(t *testing.T) {
	store := useFakeStore(t)
	if err := Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if store.closed != 1 {
		t.Errorf("the store was closed %d times, want once", store.closed)
	}
	if sharedStore() != nil {
		t.Error("a shared store is still selected after Close")
	}
	// Selecting another provider closes the one it replaces.
	next := useFakeStore(t)
	if err := Init(config.CacheConfig{Type: TypeMemory}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if next.closed != 1 {
		t.Errorf("the replaced store was closed %d times, want once", next.closed)
	}
}
