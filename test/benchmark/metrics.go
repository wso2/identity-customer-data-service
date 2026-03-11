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

package benchmark

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Metrics collects latency samples and computes percentile statistics.
type Metrics struct {
	mu       sync.Mutex
	name     string
	samples  []time.Duration
	errors   int
	retries  int
}

// NewMetrics creates a new named metrics collector.
func NewMetrics(name string) *Metrics {
	return &Metrics{name: name}
}

// Record adds a latency sample.
func (m *Metrics) Record(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.samples = append(m.samples, d)
}

// RecordError increments the error counter.
func (m *Metrics) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors++
}

// RecordRetry increments the retry counter.
func (m *Metrics) RecordRetry() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retries++
}

// Count returns the number of recorded samples.
func (m *Metrics) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.samples)
}

// Errors returns the number of recorded errors.
func (m *Metrics) Errors() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errors
}

// Retries returns the number of recorded retries.
func (m *Metrics) Retries() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.retries
}

// MetricsReport holds computed statistics for a metric.
type MetricsReport struct {
	Name       string
	Count      int
	Errors     int
	Retries    int
	Avg        time.Duration
	P50        time.Duration
	P95        time.Duration
	P99        time.Duration
	Min        time.Duration
	Max        time.Duration
	OpsPerSec  float64
	TotalTime  time.Duration
}

// Report computes statistics from the collected samples.
func (m *Metrics) Report() MetricsReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := MetricsReport{
		Name:    m.name,
		Count:   len(m.samples),
		Errors:  m.errors,
		Retries: m.retries,
	}
	if len(m.samples) == 0 {
		return r
	}

	sorted := make([]time.Duration, len(m.samples))
	copy(sorted, m.samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var total time.Duration
	for _, d := range sorted {
		total += d
	}

	r.Avg = total / time.Duration(len(sorted))
	r.P50 = percentile(sorted, 50)
	r.P95 = percentile(sorted, 95)
	r.P99 = percentile(sorted, 99)
	r.Min = sorted[0]
	r.Max = sorted[len(sorted)-1]
	r.TotalTime = total
	if total > 0 {
		r.OpsPerSec = float64(len(sorted)) / total.Seconds()
	}
	return r
}

// String formats the report for logging.
func (r MetricsReport) String() string {
	return fmt.Sprintf(
		"[%s] count=%d errors=%d avg=%v p50=%v p95=%v p99=%v min=%v max=%v ops/s=%.1f",
		r.Name, r.Count, r.Errors, r.Avg, r.P50, r.P95, r.P99, r.Min, r.Max, r.OpsPerSec,
	)
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// QueueMonitor tracks queue health metrics.
type QueueMonitor struct {
	mu              sync.Mutex
	depths          []int
	processedCount  int
	startTime       time.Time
}

// NewQueueMonitor creates a new queue monitor.
func NewQueueMonitor() *QueueMonitor {
	return &QueueMonitor{startTime: time.Now()}
}

// RecordDepth records the current queue depth.
func (qm *QueueMonitor) RecordDepth(depth int) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.depths = append(qm.depths, depth)
}

// RecordProcessed increments the processed counter.
func (qm *QueueMonitor) RecordProcessed() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.processedCount++
}

// QueueReport holds queue health statistics.
type QueueReport struct {
	MaxDepth       int
	AvgDepth       float64
	ProcessedCount int
	ProcessingRate float64
}

// Report computes queue health statistics.
func (qm *QueueMonitor) Report() QueueReport {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	r := QueueReport{
		ProcessedCount: qm.processedCount,
	}

	elapsed := time.Since(qm.startTime).Seconds()
	if elapsed > 0 {
		r.ProcessingRate = float64(qm.processedCount) / elapsed
	}

	if len(qm.depths) > 0 {
		total := 0
		for _, d := range qm.depths {
			total += d
			if d > r.MaxDepth {
				r.MaxDepth = d
			}
		}
		r.AvgDepth = float64(total) / float64(len(qm.depths))
	}
	return r
}

// String formats the queue report for logging.
func (r QueueReport) String() string {
	return fmt.Sprintf(
		"[Queue] maxDepth=%d avgDepth=%.1f processed=%d rate=%.1f/s",
		r.MaxDepth, r.AvgDepth, r.ProcessedCount, r.ProcessingRate,
	)
}
