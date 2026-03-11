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
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileService "github.com/wso2/identity-customer-data-service/internal/profile/service"
	schemaService "github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	unificationModel "github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	unificationService "github.com/wso2/identity-customer-data-service/internal/unification_rules/service"
)

// setupBenchmarkSchema registers the full schema for a benchmark org.
func setupBenchmarkSchema(b *testing.B, orgHandle string) {
	b.Helper()

	schemaSvc := schemaService.GetProfileSchemaService()
	restore := schemaService.OverrideValidateApplicationIdentifierForTest(
		func(appID, org string) (error, bool) { return nil, true })
	b.Cleanup(restore)

	sc := GenerateSchemaConfig(orgHandle)
	_ = schemaSvc.AddProfileSchemaAttributesForScope(sc.IdentityAttributes, constants.IdentityAttributes, orgHandle)
	_ = schemaSvc.AddProfileSchemaAttributesForScope(sc.Traits, constants.Traits, orgHandle)
	_ = schemaSvc.AddProfileSchemaAttributesForScope(sc.AppData, constants.ApplicationData, orgHandle)
}

// seedProfiles creates numProfiles profiles and returns their IDs.
func seedProfiles(b *testing.B, profileSvc profileService.ProfilesServiceInterface, orgHandle string, numProfiles int) []string {
	b.Helper()

	overlap := DefaultOverlap()
	ids := make([]string, 0, numProfiles)

	for i := 0; i < numProfiles; i++ {
		req := GenerateProfileRequest(i, numProfiles, overlap)
		resp, err := profileSvc.CreateProfile(req, orgHandle)
		if err != nil {
			b.Fatalf("Failed to seed profile %d: %v", i, err)
		}
		ids = append(ids, resp.ProfileId)
	}
	return ids
}

// cleanupProfiles truncates profile-related tables between benchmark runs.
func cleanupProfiles(b *testing.B) {
	b.Helper()
	if testDB == nil {
		return
	}
	tables := []string{
		"profile_consents", "profile_cookies", "application_data",
		"profile_reference", "profiles",
		"unification_rules", "profile_schema",
	}
	for _, t := range tables {
		_, _ = testDB.Exec(fmt.Sprintf("DELETE FROM %s", t))
	}
}

// ---------------------------------------------------------------------------
// 4.1 Profile Write Latency
// ---------------------------------------------------------------------------

// Benchmark_ProfileWriteLatency measures ingestion throughput with latency
// percentiles. A data set of the given size is written and per-operation
// latency is collected.
func Benchmark_ProfileWriteLatency(b *testing.B) {
	datasetSize := benchmarkDatasetSize()

	orgHandle := fmt.Sprintf("bench-write-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()
	setupBenchmarkSchema(b, orgHandle)
	b.Cleanup(func() { cleanupProfiles(b) })

	overlap := DefaultOverlap()
	metrics := NewMetrics("ProfileWrite")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i % datasetSize
		req := GenerateProfileRequest(idx+i*datasetSize, datasetSize, overlap)

		start := time.Now()
		_, err := profileSvc.CreateProfile(req, orgHandle)
		elapsed := time.Since(start)

		if err != nil {
			metrics.RecordError()
		} else {
			metrics.Record(elapsed)
		}
	}
	b.StopTimer()

	report := metrics.Report()
	b.ReportMetric(float64(report.Avg.Nanoseconds()), "avg-ns/op")
	b.ReportMetric(float64(report.P95.Nanoseconds()), "p95-ns/op")
	b.ReportMetric(float64(report.P99.Nanoseconds()), "p99-ns/op")
	b.ReportMetric(float64(report.Errors), "errors")
	b.Logf("Write Latency: %s", report)
}

// ---------------------------------------------------------------------------
// 4.2 Profile Read Latency
// ---------------------------------------------------------------------------

// Benchmark_ProfileReadLatency measures GET /profiles/{id} performance.
func Benchmark_ProfileReadLatency(b *testing.B) {
	datasetSize := benchmarkDatasetSize()

	orgHandle := fmt.Sprintf("bench-read-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()
	setupBenchmarkSchema(b, orgHandle)
	b.Cleanup(func() { cleanupProfiles(b) })

	// Seed dataset
	ids := seedProfiles(b, profileSvc, orgHandle, datasetSize)

	metrics := NewMetrics("ProfileRead")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		profileID := ids[i%len(ids)]

		start := time.Now()
		_, err := profileSvc.GetProfile(profileID)
		elapsed := time.Since(start)

		if err != nil {
			metrics.RecordError()
		} else {
			metrics.Record(elapsed)
		}
	}
	b.StopTimer()

	report := metrics.Report()
	b.ReportMetric(float64(report.Avg.Nanoseconds()), "avg-ns/op")
	b.ReportMetric(float64(report.P95.Nanoseconds()), "p95-ns/op")
	b.ReportMetric(float64(report.P99.Nanoseconds()), "p99-ns/op")
	b.ReportMetric(float64(report.Errors), "errors")
	b.Logf("Read Latency: %s", report)
}

// ---------------------------------------------------------------------------
// 4.3 Profile Filter Latency
// ---------------------------------------------------------------------------

// Benchmark_ProfileFilterLatency measures query performance across multiple
// filter types: identity, traits, and application data.
func Benchmark_ProfileFilterLatency(b *testing.B) {
	datasetSize := benchmarkDatasetSize()

	orgHandle := fmt.Sprintf("bench-filter-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()
	setupBenchmarkSchema(b, orgHandle)
	b.Cleanup(func() { cleanupProfiles(b) })

	seedProfiles(b, profileSvc, orgHandle, datasetSize)

	queries := GenerateFilterQueries()

	for _, q := range queries {
		q := q // capture
		b.Run(q.Name, func(b *testing.B) {
			metrics := NewMetrics("Filter_" + q.Name)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				start := time.Now()
				_, _, err := profileSvc.GetAllProfilesWithFilterCursor(orgHandle, q.Filter, 50, nil)
				elapsed := time.Since(start)

				if err != nil {
					metrics.RecordError()
				} else {
					metrics.Record(elapsed)
				}
			}
			b.StopTimer()

			report := metrics.Report()
			b.ReportMetric(float64(report.Avg.Nanoseconds()), "avg-ns/op")
			b.ReportMetric(float64(report.P95.Nanoseconds()), "p95-ns/op")
			b.ReportMetric(float64(report.P99.Nanoseconds()), "p99-ns/op")
			b.ReportMetric(float64(report.Errors), "errors")
			b.Logf("Filter[%s]: %s", q.Name, report)
		})
	}
}

// ---------------------------------------------------------------------------
// 4.4 Unification Latency
// ---------------------------------------------------------------------------

// Benchmark_ProfileUnificationLatency measures the full unification lifecycle:
// profile write → queue enqueue → worker merge.
func Benchmark_ProfileUnificationLatency(b *testing.B) {
	orgHandle := fmt.Sprintf("bench-unify-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()
	unificationSvc := unificationService.GetUnificationRuleService()
	setupBenchmarkSchema(b, orgHandle)
	b.Cleanup(func() { cleanupProfiles(b) })

	// Setup unification rules
	rules := []string{"identity_attributes.email", "identity_attributes.device_id", "identity_attributes.customer_id"}
	for i, prop := range rules {
		rule := unificationModel.UnificationRule{
			RuleName:     fmt.Sprintf("rule_%d", i),
			RuleId:       uuid.New().String(),
			OrgHandle:    orgHandle,
			PropertyName: prop,
			Priority:     i + 1,
			IsActive:     true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		if err := unificationSvc.AddUnificationRule(rule, orgHandle); err != nil {
			b.Fatalf("Failed to add unification rule: %v", err)
		}
	}

	// Create seed profile that others will unify against
	var seedReq profileModel.ProfileRequest
	seedJSON := []byte(`{
		"user_id": "seed-user",
		"identity_attributes": {
			"email": ["unify-bench@example.com"],
			"phone": ["+1000000000"],
			"username": "seed_user",
			"customer_id": "CUST-SEED",
			"device_id": ["device-seed"]
		},
		"traits": { "loyalty_tier": "gold", "engagement_score": "high" }
	}`)
	_ = json.Unmarshal(seedJSON, &seedReq)
	_, err := profileSvc.CreateProfile(seedReq, orgHandle)
	if err != nil {
		b.Fatalf("Failed to create seed profile: %v", err)
	}

	// Allow queue to settle
	time.Sleep(200 * time.Millisecond)

	mergeMetrics := NewMetrics("Unification")
	queueMon := NewQueueMonitor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Build profile that shares email → triggers unification
		var req profileModel.ProfileRequest
		data := []byte(fmt.Sprintf(`{
			"user_id": "unify-user-%d",
			"identity_attributes": {
				"email": ["unify-bench@example.com"],
				"phone": ["+1%010d"],
				"username": "unify_%d",
				"customer_id": "CUST-%d",
				"device_id": ["device-unify-%d"]
			},
			"traits": { "loyalty_tier": "silver", "engagement_score": "medium" }
		}`, i, i, i, i, i))
		_ = json.Unmarshal(data, &req)

		// Record queue depth before write
		queueMon.RecordDepth(len(workers.UnificationQueue))
		b.StartTimer()

		start := time.Now()
		_, err := profileSvc.CreateProfile(req, orgHandle)
		elapsed := time.Since(start)

		if err != nil {
			mergeMetrics.RecordError()
		} else {
			mergeMetrics.Record(elapsed)
			queueMon.RecordProcessed()
		}
	}
	b.StopTimer()

	// Allow unification queue to drain
	time.Sleep(500 * time.Millisecond)

	report := mergeMetrics.Report()
	qReport := queueMon.Report()
	b.ReportMetric(float64(report.Avg.Nanoseconds()), "avg-ns/op")
	b.ReportMetric(float64(report.P95.Nanoseconds()), "p95-ns/op")
	b.ReportMetric(float64(report.P99.Nanoseconds()), "p99-ns/op")
	b.ReportMetric(float64(report.Max.Nanoseconds()), "max-ns/op")
	b.ReportMetric(report.OpsPerSec, "merges/s")
	b.ReportMetric(float64(report.Errors), "merge-errors")
	b.Logf("Unification: %s", report)
	b.Logf("Queue: %s", qReport)
}

// ---------------------------------------------------------------------------
// 4.5 Queue Stability Under Merge Workload
// ---------------------------------------------------------------------------

// Benchmark_QueueStability floods the write path to observe queue health.
func Benchmark_QueueStability(b *testing.B) {
	orgHandle := fmt.Sprintf("bench-queue-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()
	unificationSvc := unificationService.GetUnificationRuleService()
	setupBenchmarkSchema(b, orgHandle)
	b.Cleanup(func() { cleanupProfiles(b) })

	// Single unification rule on email
	rule := unificationModel.UnificationRule{
		RuleName:     "email_rule",
		RuleId:       uuid.New().String(),
		OrgHandle:    orgHandle,
		PropertyName: "identity_attributes.email",
		Priority:     1,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = unificationSvc.AddUnificationRule(rule, orgHandle)

	queueMon := NewQueueMonitor()
	writeMetrics := NewMetrics("QueueStabilityWrite")
	failedMerges := NewMetrics("FailedMerges")

	overlap := DefaultOverlap()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := GenerateProfileRequest(i, b.N, overlap)

		start := time.Now()
		_, err := profileSvc.CreateProfile(req, orgHandle)
		elapsed := time.Since(start)

		if err != nil {
			writeMetrics.RecordError()
			failedMerges.RecordError()
		} else {
			writeMetrics.Record(elapsed)
		}

		// Sample queue depth
		queueMon.RecordDepth(len(workers.UnificationQueue))
		queueMon.RecordProcessed()
	}
	b.StopTimer()

	// Wait for queue to drain
	time.Sleep(1 * time.Second)
	finalDepth := len(workers.UnificationQueue)

	qReport := queueMon.Report()
	wReport := writeMetrics.Report()
	b.ReportMetric(float64(qReport.MaxDepth), "max-queue-depth")
	b.ReportMetric(qReport.AvgDepth, "avg-queue-depth")
	b.ReportMetric(qReport.ProcessingRate, "processing-rate/s")
	b.ReportMetric(float64(finalDepth), "final-queue-depth")
	b.ReportMetric(float64(failedMerges.Errors()), "merge-errors")
	b.Logf("Write: %s", wReport)
	b.Logf("Queue: %s", qReport)
	b.Logf("Final queue depth: %d (should be 0 if worker keeps up)", finalDepth)
}

// ---------------------------------------------------------------------------
// 4.6 Database Query Efficiency
// ---------------------------------------------------------------------------

// Benchmark_DatabaseQueryEfficiency measures raw database query performance
// for profile retrieval and filtering.
func Benchmark_DatabaseQueryEfficiency(b *testing.B) {
	datasetSize := benchmarkDatasetSize()

	orgHandle := fmt.Sprintf("bench-dbquery-%d", time.Now().UnixNano())
	profileSvc := profileService.GetProfilesService()
	setupBenchmarkSchema(b, orgHandle)
	b.Cleanup(func() { cleanupProfiles(b) })

	ids := seedProfiles(b, profileSvc, orgHandle, datasetSize)

	b.Run("SingleGet", func(b *testing.B) {
		metrics := NewMetrics("DB_SingleGet")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := time.Now()
			_, err := profileSvc.GetProfile(ids[i%len(ids)])
			elapsed := time.Since(start)
			if err != nil {
				metrics.RecordError()
			} else {
				metrics.Record(elapsed)
			}
		}
		b.StopTimer()
		report := metrics.Report()
		b.ReportMetric(float64(report.Avg.Nanoseconds()), "avg-ns/op")
		b.ReportMetric(float64(report.P95.Nanoseconds()), "p95-ns/op")
		b.Logf("DB SingleGet: %s", report)
	})

	b.Run("CursorPagination", func(b *testing.B) {
		metrics := NewMetrics("DB_CursorPagination")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := time.Now()
			_, _, err := profileSvc.GetAllProfilesCursor(orgHandle, 50, nil)
			elapsed := time.Since(start)
			if err != nil {
				metrics.RecordError()
			} else {
				metrics.Record(elapsed)
			}
		}
		b.StopTimer()
		report := metrics.Report()
		b.ReportMetric(float64(report.Avg.Nanoseconds()), "avg-ns/op")
		b.ReportMetric(float64(report.P95.Nanoseconds()), "p95-ns/op")
		b.Logf("DB CursorPagination: %s", report)
	})

	b.Run("FilteredQuery", func(b *testing.B) {
		metrics := NewMetrics("DB_FilteredQuery")
		filter := []string{"identity_attributes.email co 'shared'"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := time.Now()
			_, _, err := profileSvc.GetAllProfilesWithFilterCursor(orgHandle, filter, 50, nil)
			elapsed := time.Since(start)
			if err != nil {
				metrics.RecordError()
			} else {
				metrics.Record(elapsed)
			}
		}
		b.StopTimer()
		report := metrics.Report()
		b.ReportMetric(float64(report.Avg.Nanoseconds()), "avg-ns/op")
		b.ReportMetric(float64(report.P95.Nanoseconds()), "p95-ns/op")
		b.Logf("DB FilteredQuery: %s", report)
	})
}

// benchmarkDatasetSize returns the number of profiles to seed.
// Controlled via BENCH_TIER env var: small=50, medium=200, large=500.
// Defaults to small for CI-friendly runs.
func benchmarkDatasetSize() int {
	tier := os.Getenv("BENCH_TIER")
	switch tier {
	case "medium":
		return 200
	case "large":
		return 500
	default:
		return 50
	}
}
