# Performance Benchmarks

Enterprise-scale performance benchmark suite for the Identity Customer Data
Service. The benchmarks validate profile ingestion throughput, query
performance, filtering, identity resolution (unification) latency, queue
stability under merge workloads, and database query efficiency.

## Test Objectives

| # | Objective | Benchmark |
|---|-----------|-----------|
| 1 | Profile ingestion throughput | `Benchmark_ProfileWriteLatency` |
| 2 | Profile query performance | `Benchmark_ProfileReadLatency` |
| 3 | Profile filtering performance | `Benchmark_ProfileFilterLatency` |
| 4 | Identity resolution (unification) latency | `Benchmark_ProfileUnificationLatency` |
| 5 | Queue stability under merge workloads | `Benchmark_QueueStability` |
| 6 | Database query efficiency | `Benchmark_DatabaseQueryEfficiency` |

## Data Generation

The data generator (`data_generator.go`) creates realistic profile data with:

### Identity Attributes
- email, phone, username, customer_id, device_id
- 10–20% attribute overlap between profiles to trigger unification

### Traits
- loyalty_tier, engagement_score, preferred_category, spending_score, campaign_affinity
- Diverse values for segmentation testing

### Application Data
- device_type, session_id, login_method, app_preference, last_purchase

### Scale Tiers

| Tier | Seed Profiles | Spec Profiles | Spec Identity Attrs | Spec Traits | Spec App Data | Unification Rules |
|------|--------------|---------------|---------------------|-------------|---------------|-------------------|
| small | 50 | 10,000 | 100 | 100 | 100 | 3 |
| medium | 200 | 100,000 | 150 | 250 | 250 | 5 |
| large | 500 | 1,000,000 | 200 | 500 | 500 | 10 |

> **Note**: Seed profiles are the number actually inserted during benchmarks.
> Full-scale tiers represent the target deployment sizes in the specification.
> Increase `benchtime` and tier for production-representative runs.

## Running Benchmarks

### Run All Benchmarks (default: small tier)

```bash
make benchmark
```

### Run with Specific Tier

```bash
make benchmark tier=small                    # 50 seed profiles
make benchmark tier=medium                   # 200 seed profiles
make benchmark tier=large                    # 500 seed profiles
```

### Run Specific Benchmark

```bash
make benchmark bench=Benchmark_ProfileWriteLatency
make benchmark bench=Benchmark_ProfileFilterLatency tier=medium
make benchmark bench=Benchmark_QueueStability benchtime=50x
```

### Run Directly with Go

```bash
TESTCONTAINERS_RYUK_DISABLED=true BENCH_TIER=medium \
  go test -v ./test/benchmark -bench=. -benchmem -benchtime=20x -timeout 30m
```

## Metrics Captured

Every benchmark reports the following latency percentiles:

| Metric | Description |
|--------|-------------|
| avg-ns/op | Average latency per operation (nanoseconds) |
| p95-ns/op | 95th percentile latency |
| p99-ns/op | 99th percentile latency |
| errors | Number of failed operations |

### Unification-Specific Metrics

| Metric | Description |
|--------|-------------|
| max-ns/op | Maximum merge latency |
| merges/s | Merge throughput |
| merge-errors | Failed merge count |

### Queue Health Metrics

| Metric | Description |
|--------|-------------|
| max-queue-depth | Peak queue depth during test |
| avg-queue-depth | Average queue depth |
| final-queue-depth | Queue depth after test (0 = worker keeps up) |
| processing-rate/s | Worker processing rate |

## Filter Benchmarks

The filter benchmark tests five realistic query patterns:

| Sub-benchmark | Filter Expression | Tests |
|---------------|-------------------|-------|
| email_eq | `identity_attributes.email eq value` | Identity lookup |
| loyalty_eq | `traits.loyalty_tier eq 'gold'` | Trait segmentation |
| device_co | `application_data.device_type co 'mobile'` | App data search |
| username_sw | `identity_attributes.username sw 'user_1'` | Prefix search |
| category_eq | `traits.preferred_category eq 'electronics'` | Category filter |

## Baseline Metrics (Small Tier, 10 iterations)

| Benchmark | Avg | P95 | P99 | Allocs |
|-----------|-----|-----|-----|--------|
| ProfileWriteLatency | ~5.2ms | ~5.8ms | ~5.8ms | ~1,408 |
| ProfileReadLatency | ~1.6ms | ~2.0ms | ~2.0ms | ~238 |
| FilterLatency/email_eq | ~0.76ms | ~0.84ms | ~0.84ms | ~62 |
| FilterLatency/loyalty_eq | ~0.72ms | ~0.78ms | ~0.78ms | ~62 |
| FilterLatency/device_co | ~1.06ms | ~1.16ms | ~1.16ms | ~63 |
| UnificationLatency | ~4.6ms | ~8.2ms | ~8.2ms | ~2,713 |
| QueueStability | ~5.9ms | ~7.8ms | ~7.8ms | ~2,994 |
| DB/SingleGet | ~1.5ms | ~1.7ms | ~1.7ms | ~216 |
| DB/FilteredQuery | ~0.73ms | ~0.97ms | ~0.97ms | ~64 |

## Test Execution Modes

### Mode 1 — In-Memory Queue (Default)

The default mode uses the built-in in-memory channel queue (`workers.UnificationQueue`).
This establishes baseline performance without external broker overhead.

```bash
make benchmark
```

### Mode 2 — ActiveMQ Queue

For production-representative testing, configure CDS to use ActiveMQ:

1. Start ActiveMQ broker
2. Configure CDS queue settings to use ActiveMQ
3. Run the same benchmarks

Queue health metrics (depth, worker lag, merge throughput) are captured
automatically in both modes.

> **Important**: Tests should be run in both modes. In-memory results alone
> are not sufficient to validate production readiness.

## Architecture

```
test/benchmark/
├── main_test.go                  # TestMain: DB setup, worker init
├── profile_benchmark_test.go     # All benchmark functions
├── data_generator.go             # Profile data generation & tiers
├── metrics.go                    # Latency collector & queue monitor
└── README.md                     # This file
```

## Troubleshooting

### Docker Container Conflicts

```bash
docker rm -f cds-test-postgres
```

### Slow Benchmarks

Reduce tier or iteration count:
```bash
make benchmark tier=small benchtime=5x
```

### Performance Profiling

```bash
TESTCONTAINERS_RYUK_DISABLED=true go test -v ./test/benchmark \
  -bench=Benchmark_ProfileReadLatency -cpuprofile=cpu.prof -memprofile=mem.prof
go tool pprof cpu.prof
```
