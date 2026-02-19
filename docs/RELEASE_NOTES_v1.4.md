# Go-SNMPSIM v1.4

Dual-stack networking and scale-focused storage/benchmarking release.

## Highlights

- Added dual-stack UDP listener support with `--listen6` (IPv6) alongside existing IPv4 listener.
- Added IPv6 integration test for SNMP walk over `[::1]:port`.
- Reworked OID store to a sharded in-memory design for higher concurrent lookup throughput.
- Added benchmark harness in `tests/bench` for concurrency and latency profiling.
- Updated documentation with benchmark tables (p50/p95 for 1k/5k/10k agent scales) and dual-stack usage examples.

## What’s New

### 1) Dual-stack listeners

Run simulator with IPv4 + IPv6:

```bash
./snmpsim \
  -port-start=20000 -port-end=20010 -devices=10 \
  --listen 0.0.0.0 \
  --listen6 ::
```

Validate IPv6 walk:

```bash
snmpwalk -v2c -c public udp6:[::1]:20000 1.3.6.1.2.1.1
```

### 2) Sharded store for scale

OID storage now uses sharded in-memory maps with sorted OID index traversal, reducing lock contention under concurrent read-heavy load.

### 3) Benchmark harness

New benchmark/profile file:

- `tests/bench/latency_bench_test.go`

Commands:

```bash
SNMPSIM_RUN_BENCHMARKS=1 SNMPSIM_BENCH_SAMPLES=4000 SNMPSIM_BENCH_WORKERS=16 \
go test ./tests/bench -run TestStoreLatencyProfile -v -count=1

go test ./tests/bench -bench BenchmarkStoreConcurrentGet -benchmem -run '^$' -count=1
```

## Benchmark Snapshot

Latency profile (`SNMPSIM_RUN_BENCHMARKS=1`, 4000 samples, 16 workers):

| Agents | p50 | p95 |
|--------|-----|-----|
| 1,000  | 226ns | 447ns |
| 5,000  | 377ns | 740ns |
| 10,000 | 398ns | 605ns |

Concurrency benchmark:

| Agents | ns/op | allocs/op |
|--------|-------|-----------|
| 1,000  | 22.50 | 0 |
| 5,000  | 28.59 | 0 |
| 10,000 | 44.49 | 0 |

## Included Files (major)

- `cmd/snmpsim/main.go`
- `internal/engine/simulator.go`
- `internal/engine/ipv6_integration_test.go`
- `internal/store/database.go`
- `tests/bench/latency_bench_test.go`
- `README.md`
- `docs/DOCUMENTATION_INDEX.md`

## Notes

- Existing SNMPv2c/v3 functionality, routing, variation plugins, recorder/diff CLIs, and trap/inform features remain available.
- If your environment has IPv6 disabled, IPv6 integration tests skip automatically.
