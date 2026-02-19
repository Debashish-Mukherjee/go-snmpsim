# Go-SNMPSIM 🚀

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)]()

> A high-performance SNMP simulator for testing monitoring systems at scale. Simulate 1,000+ virtual devices with minimal resource usage.

## 📦 Release

- **Current Release**: `v1.3`
- **Release Date**: 2026-02-19
- **Highlights**:
  - SNMPv2c + full SNMPv3 (noAuthNoPriv / authNoPriv / authPriv) support in simulator
  - HMAC authentication and AES/DES privacy fully implemented with RFC 3414 compliance
  - Correct USM security error Reports (unknownEngineID, notInTimeWindow, wrongDigest)
  - Zabbix host provisioning updated for SNMPv3 interfaces
  - End-to-end SNMPv3 polling verified with history collection
  - First 50 active hosts migrated to SNMPv3 (`cisco-iosxr-001` to `cisco-iosxr-050`)

### Release Notes (v1.3)

- Added `gosnmpsim-record` to learn live SNMP devices and export `.snmprec` using `OID|TYPE|VALUE`
- Added `gosnmpsim-diff` to compare two recorded walks and report missing/value/type mismatches
- Recorder supports default enterprise roots, exclusions, max OID cap, and rate-limited collection
- Recorder supports both v2c (`--community`) and v3 (`--v3-*`) authentication modes
- Added integration test flow: record mock agent → replay with go-snmpsim → diff identical
- Added trap/inform emission with multi-target support (`--trap-target`, repeatable)
- Added trap builders for SNMPv2c and SNMPv3 (`--trap-version v2c|v3` + existing `--v3-*` auth/priv flags)
- Added trigger support for cron specs, variation events, and SET attempts on selected OIDs
- Added integration tests with `snmptrapd` validating trap varbinds and v3 auth-user acceptance

## ✨ Features

- 🔥 **High Performance** - Handle 10,000+ PDU/sec per port with O(log n) OID lookups
- 📡 **Multi-Device Simulation** - Simulate 1,000+ virtual SNMP devices simultaneously
- 🎯 **Protocol Support** - SNMP v2c and full SNMPv3 (noAuthNoPriv / authNoPriv / authPriv)
- 🗂️ **Flexible Data Loading** - Support for `.snmprec` files, snmpwalk output (3 formats)
- 🔧 **Production Ready** - Context-based graceful shutdown, resource monitoring
- 📊 **Zabbix Optimized** - Table indexing for LLD, <100ms response for 1,056 OIDs
- 🐳 **Docker Support** - Ready-to-use containerized deployment

## 🚀 Quick Start

### Installation

```bash
# Clone repository
git clone https://github.com/debashish-mukherjee/go-snmpsim.git
cd go-snmpsim

# Build
make build

# Run
./snmpsim -port-start=20000 -port-end=20010 -devices=10
```

### Docker

```bash
# Using Docker Compose
docker-compose up -d

# Using Docker directly
docker build -t go-snmpsim .
docker run -p 20000-20100:20000-20100/udp go-snmpsim \
  -port-start=20000 -port-end=20100 -devices=100
```

### Test It

```bash
# Query a simulated device
snmpget -v2c -c public localhost:20000 1.3.6.1.2.1.1.1.0

# Query the same device using SNMPv3
snmpget -v3 -l noAuthNoPriv -u simuser localhost:20000 1.3.6.1.2.1.1.1.0

# Walk all OIDs
snmpwalk -v2c -c public localhost:20000 1.3.6.1.2.1

# Bulk walk (efficient)
snmpbulkwalk -v2c -c public localhost:20000 1.3.6.1.2.1.2.2.1
```

### SNMPv3 Quick Start

Start the simulator with SNMPv3 authentication and privacy enabled:

```bash
./snmpsim \
  -port-start=20000 -port-end=20000 -devices=1 \
  -snmpv3-enabled \
  -snmpv3-user simuser \
  -snmpv3-auth SHA \
  -snmpv3-auth-key authpass123 \
  -snmpv3-priv AES128 \
  -snmpv3-priv-key privpass123
```

Then query using any security level:

```bash
# Discovery / no security
snmpget -v3 -l noAuthNoPriv -u simuser localhost:20000 1.3.6.1.2.1.1.5.0

# Authentication only (authNoPriv)
snmpwalk -v3 -l authNoPriv -u simuser \
  -a SHA -A authpass123 \
  localhost:20000 1.3.6.1.2.1.1

# Authentication + privacy (authPriv)
snmpwalk -v3 -l authPriv -u simuser \
  -a SHA -A authpass123 \
  -x AES -X privpass123 \
  localhost:20000 1.3.6.1.2.1
```

**Supported protocols:**

| Role | Options |
|------|---------|
| Auth | `MD5`, `SHA` (SHA-1), `SHA224`, `SHA256`, `SHA384`, `SHA512` |
| Priv | `DES`, `AES128`, `AES192`, `AES256` |

### SNMP Interaction Test Matrix

Comprehensive SNMP interaction coverage is provided by `TestSNMPInteractionsComprehensive` in `internal/engine/snmpv3_integration_test.go`.

Coverage includes:
- SNMPv1: GET (`snmpget`)
- SNMPv2c: GET, GETNEXT, GETBULK, missing OID behavior, SET rejection (read-only)
- SNMPv3: noAuthNoPriv GET, authNoPriv GETNEXT, authPriv BULKGET

Run only the comprehensive matrix test:

```bash
go test ./internal/engine/... -run TestSNMPInteractionsComprehensive -v -count=1 -timeout 120s
```

Run all engine integration tests (matrix + SNMPv3 report/walk tests):

```bash
go test ./internal/engine/... -v -count=1 -timeout 240s
```

Note: these integration tests require Docker and a running Docker daemon because test commands execute `net-snmp-tools` in a container.

### Dataset Routing with routes.yaml

Use `--route-file` to serve different datasets from the same listening port based on request metadata.

Supported matchers per route:
- `community` (SNMPv1/v2c)
- `context` (SNMPv3 contextName)
- `engineID` (SNMPv3 authoritative engine ID)
- `srcIP` (sender IP)
- `dstPort` (listener UDP port)

Priority order is deterministic:
1. `engineID + context`
2. `context`
3. `community`
4. `endpoint` (`srcIP` / `dstPort`)
5. `default`

Example route file: [examples/routes.yaml](examples/routes.yaml)

Run with routing enabled:

```bash
./snmpsim \
      -port-start=20000 -port-end=20000 -devices=1 \
      -snmprec sample.snmprec \
      -route-file examples/routes.yaml
```

### OID Variation Plugins

Use `--variation-file` to apply variation chains to returned OIDs before response encoding.

Built-ins:
- `counterMonotonic`
- `randomJitter`
- `step`
- `periodicReset`
- `dropOID`
- `timeout`

Example file (`variations.yaml`):

```yaml
bindings:
      - prefix: "1.3.6.1.2.1.2.2.1.10"
            variations:
                  - type: counterMonotonic
                        delta: 7
```

Ready-to-use sample: [examples/variations.yaml](examples/variations.yaml)

Run with variations:

```bash
./snmpsim \
      -port-start=20000 -port-end=20000 -devices=1 \
      -snmprec sample.snmprec \
      -variation-file variations.yaml
```

### Record Live Devices to .snmprec

Record from SNMPv2c target (uses default walk roots):

```bash
go run ./cmd/gosnmpsim-record \
      --target 192.0.2.10 --port 161 --community public \
      --out device.snmprec
```

Record from SNMPv3 target with exclusions, limits, and rate control:

```bash
go run ./cmd/gosnmpsim-record \
      --target 192.0.2.10 --port 161 \
      --v3-user simuser --v3-auth SHA256 --v3-auth-key authpass \
      --v3-priv AES128 --v3-priv-key privpass \
      --exclude 1.3.6.1.2.1.1.3 --exclude 1.3.6.1.2.1.31.1.1.1.15 \
      --max-oids 5000 --rate-limit 200 \
      --out device-v3.snmprec
```

Default walk roots are:

```text
1.3.6.1.2.1.1
1.3.6.1.2.1.2.2
1.3.6.1.2.1.31.1.1
1.3.6.1.2.1.25
1.3.6.1.2.1.99
1.3.6.1.4.1
```

### Compare Two Walks

```bash
go run ./cmd/gosnmpsim-diff --left before.snmprec --right after.snmprec
```

### Trap/Inform Emission

Enable SNMPv2c traps to one or more targets:

```bash
./snmpsim \
      -port-start=20000 -port-end=20010 -devices=10 \
      --trap-target 127.0.0.1:9162 \
      --trap-target 127.0.0.1:9163 \
      --trap-version v2c --trap-community public
```

Enable SNMPv3 informs with event triggers:

```bash
./snmpsim \
      -port-start=20000 -port-end=20010 -devices=10 \
      --trap-target 127.0.0.1:9162 \
      --trap-version v3 \
      --v3-user simuser --v3-auth SHA --v3-auth-key authpass123 \
      --v3-priv AES128 --v3-priv-key privpass123 \
      --trap-inform \
      --trap-cron "*/5 * * * *" \
      --trap-on-variation \
      --trap-on-set-oid 1.3.6.1.2.1.1.5.0
```

Trigger behavior:
- `--trap-cron`: emits periodic notification events based on cron spec
- `--trap-on-variation`: emits when variation engine changes or drops/times out an OID
- `--trap-on-set-oid`: emits on SET attempts to matching OIDs

### 2000-Device Stress Suite (Cisco IOS-style)

Stress suite validates startup/listeners and concurrent SNMP GET/BULK operations across 2000 devices.

Included variants:
- SNMPv2c stress sweep (`TestStress2000CiscoIOSDevices`)
- SNMPv3 noAuthNoPriv stress sweep (`TestStress2000CiscoIOSDevicesV3NoAuthNoPriv`)
- 10-minute soak mode (`TestStressSoak10Minutes`, opt-in)

Run:

```bash
./scripts/stress_test_2000.sh
```

Default run executes v2 stress only for stable baseline validation.

Run v3 variant only:

```bash
./scripts/stress_test_2000.sh --v3-only
```

If the environment cannot establish successful SNMPv3 noAuthNoPriv responses at this scale, the v3 test reports diagnostics and is marked as skipped.

Run default v2 plus optional v3 variant:

```bash
./scripts/stress_test_2000.sh --with-v3
```

Run soak mode (default 10m):

```bash
./scripts/stress_test_2000.sh --soak
```

Run soak smoke (example 30s):

```bash
./scripts/stress_test_2000.sh --soak --duration 30s
```

Direct command:

```bash
go test -tags stress ./internal/engine -run 'TestStress2000CiscoIOSDevices|TestStress2000CiscoIOSDevicesV3NoAuthNoPriv' -v -count=1 -timeout 20m

SNMPSIM_STRESS_SOAK=1 SNMPSIM_STRESS_SOAK_DURATION=10m \
SNMPSIM_STRESS_SOAK_MAX_FAILURE=0.90 \
go test -tags stress ./internal/engine -run TestStressSoak10Minutes -v -count=1 -timeout 30m
```

## 🏆 Scale to 1,000+ Hosts with Zabbix

See **[docs/SCALING_GUIDE.md](docs/SCALING_GUIDE.md)** for complete instructions to deploy:
- **1,000 virtual SNMP devices** (ports 20000-20999)
- **1,354,000 metrics** total (~1,354 per host)
- **5-minute polling** on all items
- **Production-ready** Zabbix integration

### Latest Test Results

See **[docs/TEST_REPORT.md](docs/TEST_REPORT.md)** for details:
- ✅ **1000 hosts** created and deployed
- ✅ **1.35M+ metrics** configured
- ✅ **5-minute polling** verified
- ✅ **27k+ values/minute** data collection rate
- ✅ **All 1876 OIDs** available in SNMP data

**Quick Deploy**:
```bash
# See docs/SCALING_GUIDE.md for all steps
python3 scripts/add_remaining_hosts.py       # Add 900 hosts to Zabbix
python3 scripts/add_bulk_items.py            # Deploy ~1,500 items per host
```

## 📋 Command-Line Options

```bash
Usage: snmpsim [options]

Options:
  -port-start int
        Starting port for UDP listeners (default: 20000)
  -port-end int
        Ending port for UDP listeners (default: 30000)
  -devices int
        Number of virtual devices to simulate (default: 100)
  -snmprec string
        Path to .snmprec file for OID templates
  -route-file string
        Path to routes.yaml for dataset routing
  -variation-file string
        Path to variations.yaml for OID variation chains
  -listen string
        Listen address (default: 0.0.0.0)
  -snmpv3-enabled
        Enable SNMPv3 support (default: false)
  -snmpv3-user string
        SNMPv3 username (default: simuser)
  -snmpv3-auth string
        SNMPv3 auth protocol: MD5, SHA, SHA224, SHA256, SHA384, SHA512 (default: SHA)
  -snmpv3-auth-key string
        SNMPv3 authentication passphrase
  -snmpv3-priv string
        SNMPv3 privacy protocol: DES, AES128, AES192, AES256 (default: AES128)
  -snmpv3-priv-key string
        SNMPv3 privacy passphrase
```

## 🏗️ Architecture

### Project Structure

```
go-snmpsim/
├── cmd/snmpsim/        # Main entry point & CLI
├── internal/           # Internal packages
│   ├── engine/        # UDP listener management, packet dispatching
│   ├── agent/         # Virtual device logic, PDU processing
│   └── store/         # OID storage, indexing, data loading
├── docs/              # Comprehensive documentation
├── examples/          # Sample configurations & test data
├── scripts/           # Deployment & testing scripts
└── build/             # Build artifacts
```

### Component Overview

```
┌─────────────────────────────────────────────┐
│         SNMP Simulator Engine               │
├─────────────────────────────────────────────┤
│  UDP Listeners (Port Range)                 │
│    ↓                                         │
│  Packet Dispatcher (Buffer Pool)            │
│    ↓                                         │
│  Virtual Agents (1000+)                     │
│    ↓                                         │
│  OID Database (Radix Tree + Index)          │
└─────────────────────────────────────────────┘
```

## 📚 Documentation

- **[Architecture](docs/ARCHITECTURE.md)** - System design and components
- **[Refactoring Guide](docs/REFACTORING.md)** - Standard Go project layout
- **[Zabbix Integration](docs/ZABBIX_INTEGRATION.md)** - LLD and table indexing
- **[Implementation Details](docs/IMPLEMENTATION.md)** - Technical deep-dive
- **[Testing Guide](docs/TESTING.md)** - Testing strategies
- **[Graceful Shutdown](docs/GRACEFUL_SHUTDOWN.md)** - Context-based shutdown

### Phase Documentation

- [Phase 1: snmpwalk Parser](docs/PHASE_1_COMPLETION.md) - Multi-format snmpwalk support
- [Phase 3: Device Mappings](docs/PHASE_3_COMPLETION.md) - Port/device-specific OIDs
- [Phase 4: Table Indexing](docs/PHASE_4_COMPLETION.md) - Zabbix LLD optimization

## 🎯 Use Cases

### Monitoring System Testing

Test SNMP monitoring tools (Zabbix, Nagios, PRTG, etc.) with realistic load:

```bash
# Simulate 500 network devices
./snmpsim -port-start=20000 -port-end=20499 -devices=500

# Load custom OID data
./snmpsim -snmprec=examples/testdata/zabbix-48port-switch.snmprec -devices=10
```

### Load Testing

Stress test monitoring systems with high device counts:

```bash
# 1,000 devices across ports 20000-20999
./snmpsim -port-start=20000 -port-end=20999 -devices=1000

# Configure Zabbix/Nagios to poll localhost:20000-20999
```

### Development & CI/CD

Mock SNMP devices in development and testing pipelines:

```yaml
# docker-compose.yml
services:
  snmpsim:
    image: go-snmpsim:latest
    ports:
      - "20000-20010:20000-20010/udp"
    command: ["-devices=10"]
```

## 🔧 Configuration

### SNMPREC File Format

Create custom OID data:

```bash
# Format: OID|TYPE|VALUE
1.3.6.1.2.1.1.1.0|string|Linux Router
1.3.6.1.2.1.1.3.0|timeticks|123456
1.3.6.1.2.1.2.1.0|integer|48

# Template expansion (Phase 2)
1.3.6.1.2.1.2.2.1.2|string|Interface-|#1-48
1.3.6.1.2.1.2.2.1.5|integer|1000000000|#1-48
```

### Device-Specific Mappings

Override OIDs for specific ports/devices:

```bash
# Format: @port:OID|TYPE|VALUE
@20001:1.3.6.1.2.1.1.5.0|string|Switch-01
@20002:1.3.6.1.2.1.1.5.0|string|Router-Core
```

## 📊 Performance

| Metric | Value |
|--------|-------|
| **Throughput** | 10,000+ PDU/sec per port |
| **Latency** | <1ms typical, <100ms Zabbix LLD |
| **Memory** | ~5-7 MB for 1,000 devices |
| **OID Lookup** | O(log n) via radix tree |
| **Scalability** | Tested up to 1,000 devices |

### Benchmarks

```bash
# Single device bulk walk
snmpbulkwalk -v2c -c public localhost:20000 1.3.6.1.2.1.2.2.1
# Result: 1,056 OIDs in 50-105ms (Phase 4 optimization)

# 100 devices concurrent polling
for i in {0..99}; do
  snmpget -v2c -c public localhost:$((20000+i)) 1.3.6.1.2.1.1.1.0 &
done
wait
# Result: All devices respond in <2 seconds
```

## 🛠️ Development

### Prerequisites

- Go 1.21+
- Make
- Docker (optional)

### Build from Source

```bash
# Clone and build
git clone https://github.com/debashish-mukherjee/go-snmpsim.git
cd go-snmpsim
make build

# Run tests
make test

# Build release binary (optimized)
make build-release
```

### Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Areas for contribution:**
- SNMP v3 USM authentication
- TRAP/Notification support
- Additional OID templates
- Performance optimizations
- Documentation improvements

## 🐛 Troubleshooting

### Port Already in Use

```bash
# Find process using port
lsof -i :20000

# Kill process
kill -9 <PID>
```

### File Descriptor Limit

```bash
# Check limit
ulimit -n

# Increase limit (temporary)
ulimit -n 65536

# Permanent: Add to /etc/security/limits.conf
* soft nofile 65536
* hard nofile 65536
```

### High Memory Usage

```bash
# Monitor memory
ps aux | grep snmpsim

# Reduce device count
./snmpsim -devices=100  # Start smaller

# Check for goroutine leaks
# Build with debug flags and use pprof
```

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [gosnmp](https://github.com/gosnmp/gosnmp) - SNMP library for Go
- [go-radix](https://github.com/armon/go-radix) - Radix tree implementation
- SNMP RFCs: [1905](https://tools.ietf.org/html/rfc1905), [1906](https://tools.ietf.org/html/rfc1906), [1907](https://tools.ietf.org/html/rfc1907)

## 📮 Contact

- **Author**: Debashish Mukherjee
- **GitHub**: [@debashish-mukherjee](https://github.com/debashish-mukherjee)
- **Issues**: [Report bugs or request features](https://github.com/debashish-mukherjee/go-snmpsim/issues)

---

<p align="center">
  <sub>Built with ❤️ for the network monitoring community</sub>
</p>
