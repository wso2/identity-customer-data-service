# Performance Benchmarks

This directory contains performance benchmarks for the Identity Customer Data Service. These benchmarks are designed to measure the performance of core profile operations and help ensure that new changes do not negatively impact performance.

## Overview

The benchmark suite focuses on critical profile operations:
- **Profile Get**: Retrieve individual profiles by ID
- **Profile Filtering**: Query profiles with filters
- **Profile Unification**: Automatic merging of profiles based on matching attributes

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
# Run only profile get benchmark
make benchmark bench=Benchmark_GetProfile

# Run profile filtering benchmark
make benchmark bench=Benchmark_GetAllProfilesWithFilter

# Run profile unification benchmark
make benchmark bench=Benchmark_ProfileUnification
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
Benchmark_GetProfile-8    10    1234567 ns/op    12345 B/op    123 allocs/op
```

- `Benchmark_GetProfile-8`: Benchmark name with GOMAXPROCS value (8)
- `10`: Number of iterations run
- `1234567 ns/op`: Average time per operation in nanoseconds
- `12345 B/op`: Average bytes allocated per operation
- `123 allocs/op`: Average number of allocations per operation

### Performance Metrics to Watch

1. **ns/op (Time per operation)**: Lower is better. Watch for significant increases.
2. **B/op (Bytes per operation)**: Lower is better. High values may indicate memory inefficiency.
3. **allocs/op (Allocations per operation)**: Lower is better. Many allocations can cause GC pressure.

## Baseline Performance Metrics

Below are the baseline performance metrics established on initial implementation (AMD EPYC 7763 64-Core Processor, 4 cores, 10 iterations). Use these as a reference point for comparison:

### Profile Operations

| Operation | Time (ns/op) | Memory (B/op) | Allocations | Description |
|-----------|-------------|---------------|-------------|-------------|
| GetProfile | ~1,800,000 | ~13,300 | ~244 | Retrieve profile by ID |
| GetAllProfilesWithFilter | ~840,000 | ~6,600 | ~99 | Query profiles with filters |
| ProfileUnification | ~3,900,000 | ~71,700 | ~1,125 | Create profile triggering unification |

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
    
    setupTestSchema(b, orgHandle)
    
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
