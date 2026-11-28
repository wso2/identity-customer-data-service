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
	"fmt"
	"testing"
	"time"
)

func TestGetDropsAnExpiredEntry(t *testing.T) {
	c, ttl := newMemory[any](), 10*time.Millisecond
	c.set("k", "v", ttl)

	if _, found := c.get("k"); !found {
		t.Fatal("entry should be readable before it expires")
	}

	time.Sleep(20 * time.Millisecond)

	if _, found := c.get("k"); found {
		t.Error("expired entry should not be readable")
	}
	if c.len() != 0 {
		t.Errorf("reading an expired entry should remove it; %d left", c.len())
	}
}

// TestExpiredEntriesDoNotAccumulate is the bug this guards: an expired entry was invisible
// to Get but still held its slot, and nothing ever removed it. Keyed by something
// high-cardinality — an email address — the map grew for the life of the process.
func TestExpiredEntriesDoNotAccumulate(t *testing.T) {
	c, ttl := newMemory[any](), time.Millisecond

	for i := 0; i < initialSweepThreshold*3; i++ {
		c.set(fmt.Sprintf("value-%d@example.com", i), i, ttl)
		if i%64 == 0 {
			time.Sleep(2 * time.Millisecond)
		}
	}

	// Entries written long enough ago to have expired must not still be resident. The live
	// set here is tiny, so the cache should be nowhere near the number of keys written.
	if c.len() >= initialSweepThreshold*2 {
		t.Errorf("cache held %d entries after writing %d short-lived keys; it is not being swept",
			c.len(), initialSweepThreshold*3)
	}
}

// TestLiveEntriesSurviveASweep checks the sweep only removes what has actually expired.
func TestLiveEntriesSurviveASweep(t *testing.T) {
	c, ttl := newMemory[any](), time.Hour

	for i := 0; i < initialSweepThreshold+50; i++ {
		c.set(fmt.Sprintf("k%d", i), i, ttl)
	}

	if c.len() != initialSweepThreshold+50 {
		t.Errorf("unexpired entries were dropped: have %d, wrote %d",
			c.len(), initialSweepThreshold+50)
	}
	if _, found := c.get("k0"); !found {
		t.Error("the first entry should still be present")
	}
}

func TestSetRefreshesExpiry(t *testing.T) {
	c, ttl := newMemory[any](), 40*time.Millisecond
	c.set("k", "first", ttl)

	time.Sleep(25 * time.Millisecond)
	c.set("k", "second", ttl)
	time.Sleep(25 * time.Millisecond)

	value, found := c.get("k")
	if !found {
		t.Fatal("re-setting a key should extend its life")
	}
	if value != "second" {
		t.Errorf("value = %v, want the refreshed one", value)
	}
}

func TestConcurrentReadsAndWrites(t *testing.T) {
	c, ttl := newMemory[any](), 5*time.Millisecond
	done := make(chan struct{})

	for w := 0; w < 4; w++ {
		go func(w int) {
			for i := 0; i < 500; i++ {
				c.set(fmt.Sprintf("w%d-k%d", w, i), i, ttl)
				c.get(fmt.Sprintf("w%d-k%d", w, i/2))
			}
			done <- struct{}{}
		}(w)
	}
	for w := 0; w < 4; w++ {
		<-done
	}
}
