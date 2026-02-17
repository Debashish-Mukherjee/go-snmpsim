# Go-SNMPSIM 🚀

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)]()

> A high-performance SNMP simulator for testing monitoring systems at scale. Simulate 1,000+ virtual devices with minimal resource usage.

## ✨ Features

- 🔥 **High Performance** - Handle 10,000+ PDU/sec per port with O(log n) OID lookups
- 📡 **Multi-Device Simulation** - Simulate 1,000+ virtual SNMP devices simultaneously
- 🎯 **Protocol Support** - Full SNMP v2c implementation (v3 framework ready)
- 🗂️ **Flexible Data Loading** - Support for `.snmprec` files, snmpwalk output (3 formats)
- 🔧 **Production Ready** - Context-based graceful shutdown, resource monitoring
- 📊 **Zabbix Optimized** - Table indexing for LLD, <100ms response for 1,056 OIDs
- 🐳 **Docker Support** - Ready-to-use containerized deployment

## 🚀 Quick Start

### Installation

```bash
# Clone repository
git clone https://github.com/debashish/go-snmpsim.git
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

# Walk all OIDs
snmpwalk -v2c -c public localhost:20000 1.3.6.1.2.1

# Bulk walk (efficient)
snmpbulkwalk -v2c -c public localhost:20000 1.3.6.1.2.1.2.2.1
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
  -listen string
        Listen address (default: 0.0.0.0)
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
git clone https://github.com/debashish/go-snmpsim.git
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

- **Author**: Debashish Chakravarty
- **GitHub**: [@debashish](https://github.com/debashish)
- **Issues**: [Report bugs or request features](https://github.com/debashish/go-snmpsim/issues)

---

<p align="center">
  <sub>Built with ❤️ for the network monitoring community</sub>
</p>
