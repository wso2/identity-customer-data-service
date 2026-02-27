# Performance Benchmarks

This directory contains performance benchmarks for the Identity Customer Data Service. These benchmarks are designed to measure the performance of core operations and help ensure that new changes do not negatively impact performance.

## Overview

The benchmark suite tests the following areas:
- **Profile Operations**: Create, Read, Update, Patch, Delete, List profiles
- **Profile Schema Operations**: Get schema, Add/Update/Delete schema attributes
- **Unification Rules**: Add, Get, Update, Delete unification rules

## Running Benchmarks

### Run All Benchmarks

To run all benchmarks with default settings (10 iterations per benchmark):

```bash
make benchmark
```

### Run Specific Benchmark

To run a specific benchmark by name:

```bash
make benchmark bench=BenchmarkName
```

Examples:
```bash
# Run only profile creation benchmark
make benchmark bench=Benchmark_CreateProfile

# Run all profile-related benchmarks
make benchmark bench=Benchmark_.*Profile

# Run schema benchmarks
make benchmark bench=Benchmark_.*Schema
```

### Run Benchmarks Directly with Go

You can also run benchmarks directly using the Go testing tool for more control:

```bash
# Run with custom iteration count
TESTCONTAINERS_RYUK_DISABLED=true go test -v ./test/benchmark -bench=. -benchmem -benchtime=100x

# Run with custom duration
TESTCONTAINERS_RYUK_DISABLED=true go test -v ./test/benchmark -bench=. -benchmem -benchtime=10s

# Save results to file for comparison
TESTCONTAINERS_RYUK_DISABLED=true go test -v ./test/benchmark -bench=. -benchmem -benchtime=10x > benchmark_results.txt

# Compare benchmark results
TESTCONTAINERS_RYUK_DISABLED=true go test -v ./test/benchmark -bench=. -benchmem -benchtime=10x > new_results.txt
go install golang.org/x/perf/cmd/benchstat@latest
benchstat benchmark_results.txt new_results.txt
```

## Understanding Benchmark Results

Benchmark output includes several metrics:

```
Benchmark_CreateProfile-8    10    5234567 ns/op    12345 B/op    123 allocs/op
```

- `Benchmark_CreateProfile-8`: Benchmark name with GOMAXPROCS value (8)
- `10`: Number of iterations run
- `5234567 ns/op`: Average time per operation in nanoseconds
- `12345 B/op`: Average bytes allocated per operation
- `123 allocs/op`: Average number of allocations per operation

### Performance Metrics to Watch

1. **ns/op (Time per operation)**: Lower is better. Watch for significant increases.
2. **B/op (Bytes per operation)**: Lower is better. High values may indicate memory inefficiency.
3. **allocs/op (Allocations per operation)**: Lower is better. Many allocations can cause GC pressure.

## Baseline Performance Metrics

Below are the baseline performance metrics established on initial implementation (AMD EPYC 7763 64-Core Processor, 4 cores, 10 iterations). Use these as a reference point for comparison:

### Profile Operations

| Operation | Time (ns/op) | Memory (B/op) | Allocations |
|-----------|-------------|---------------|-------------|
| CreateProfile | ~5,609,000 | ~57,200 | ~922 |
| GetProfile | ~1,601,000 | ~10,900 | ~202 |
| UpdateProfile | ~4,891,000 | ~56,900 | ~903 |
| GetAllProfiles | ~5,891,000 | ~81,200 | ~1,397 |
| GetAllProfilesWithFilter | ~1,000,000 | ~5,600 | ~81 |
| DeleteProfile | ~2,854,000 | ~11,900 | ~233 |

**Note**: PatchProfile benchmark is currently disabled due to known issues with application_data marshaling.

### Profile Schema Operations

| Operation | Time (ns/op) | Memory (B/op) | Allocations |
|-----------|-------------|---------------|-------------|
| GetProfileSchema | ~468,000 | ~6,100 | ~78 |
| AddSchemaAttribute | ~1,098,000 | ~4,400 | ~75 |
| GetSchemaAttributesByScope | ~481,500 | ~6,000 | ~96 |
| DeleteSchemaAttribute | ~1,083,000 | ~4,500 | ~80 |

**Note**: PatchSchemaAttribute benchmark is currently disabled due to validation issues in the implementation.

### Unification Rules Operations

| Operation | Time (ns/op) | Memory (B/op) | Allocations |
|-----------|-------------|---------------|-------------|
| GetUnificationRules | ~427,700 | ~2,600 | ~49 |
| GetUnificationRuleById | ~478,400 | ~2,600 | ~48 |
| PatchUnificationRule | ~1,058,000 | ~3,900 | ~73 |
| DeleteUnificationRule | ~476,600 | ~900 | ~17 |

**Note**: AddUnificationRule benchmark is currently disabled because the system only allows one rule per property per organization.

## Best Practices for Benchmarking

1. **Consistent Environment**: Always run benchmarks on the same hardware with similar system load.
2. **Multiple Runs**: Run benchmarks multiple times to account for variability.
3. **Baseline Comparison**: Compare results against baseline to detect regressions.
4. **Isolated Changes**: When testing performance of a change, isolate the change to understand its impact.
5. **Document Changes**: If baseline metrics change significantly due to intentional optimization or feature addition, update this document.

## Adding New Benchmarks

When adding new benchmarks, follow these guidelines:

1. Use the `Benchmark_` prefix for benchmark function names
2. Accept `*testing.B` as the parameter
3. Call `b.ResetTimer()` before the benchmarked code
4. Use `b.StopTimer()` and `b.StartTimer()` for setup/teardown within the loop
5. Use `b.Helper()` for helper functions
6. Clean up resources using `b.Cleanup()`

Example:
```go
func Benchmark_NewOperation(b *testing.B) {
    // Setup
    orgHandle := fmt.Sprintf("bench-org-%d", time.Now().UnixNano())
    service := mypackage.GetService()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Benchmarked code
        _, err := service.DoOperation(orgHandle)
        if err != nil {
            b.Fatalf("Operation failed: %v", err)
        }
    }
}
```

## Troubleshooting

### Docker Container Issues

If you encounter Docker container issues:
```bash
# Clean up Docker containers
docker rm -f cds-test-postgres

# List running containers
docker ps -a | grep cds-test
```

### Test Database Issues

If benchmarks fail due to database issues:
1. Ensure PostgreSQL test container is running
2. Check that schema files are properly loaded
3. Verify TESTCONTAINERS_RYUK_DISABLED is set

### Performance Degradation

If you notice performance degradation:
1. Profile the code using Go's built-in profiler:
   ```bash
   go test -v ./test/benchmark -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof
   go tool pprof cpu.prof
   ```
2. Look for increased allocations or time spent in specific functions
3. Compare with previous benchmark results using `benchstat`

## Continuous Integration

These benchmarks should be run periodically in CI to track performance trends over time. Consider:
- Running benchmarks on a schedule (e.g., nightly)
- Storing benchmark results for historical comparison
- Setting up alerts for significant performance regressions

## References

- [Go Benchmark Documentation](https://golang.org/pkg/testing/#hdr-Benchmarks)
- [Go Performance Testing Best Practices](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- [benchstat Tool](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)
