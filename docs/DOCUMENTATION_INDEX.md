# Repository Documentation Index

> **Release**: `v1.2` (2026-02-19) — includes SNMPv3 end-to-end support, dataset routing, variation plugins, and 2000-device stress suite.

## Quick Start

- **[README.md](../README.md)** - Project overview and basic setup
- **[QUICKSTART.md](QUICKSTART.md)** - Get running in 5 minutes
- **[QUICK_COMMANDS.sh](../scripts/QUICK_COMMANDS.sh)** - Essential commands reference

## Deployment & Infrastructure

- **[DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md)** - Docker setup and configuration
- **[SETUP_COMPLETE.md](SETUP_COMPLETE.md)** - Deployment validation and testing

## Performance & Optimization

- **[PERFORMANCE_REVIEW.md](PERFORMANCE_REVIEW.md)** - Initial performance analysis (5 optimizations)
- **[OPTIMIZATIONS_README.md](OPTIMIZATIONS_README.md)** - Optimization implementation guide
- **[OPTIMIZATION_SUMMARY.md](OPTIMIZATION_SUMMARY.md)** - Summary of all optimizations
- **[OPTIMIZATION_QUICKREF.md](OPTIMIZATION_QUICKREF.md)** - Quick reference for optimizations

## Scaling & Testing

- **[SCALING_GUIDE.md](SCALING_GUIDE.md)** - ⭐ **PRIMARY** - How to scale from 100 to 1000 hosts
- **[TEST_REPORT.md](TEST_REPORT.md)** - ⭐ **PRIMARY** - Complete test results (1000 host deployment)

## Integration & Implementation

- **[INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md)** - Zabbix API integration details
- **[IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md)** - Project implementation details
- **[COMPLETION_SUMMARY.txt](COMPLETION_SUMMARY.txt)** - Phase completion record

## Web UI & Advanced Features

- **[WEB_UI_IMPLEMENTATION.md](WEB_UI_IMPLEMENTATION.md)** - Web UI setup and features
- **[WEB_UI_QUICKSTART.md](WEB_UI_QUICKSTART.md)** - Web UI quick start guide

## Development & Contributing

- **[CONTRIBUTING.md](../CONTRIBUTING.md)** - Contribution guidelines
- **[GITHUB_CHECKLIST.md](GITHUB_CHECKLIST.md)** - Pre-commit checklist

## Architecture & Detailed Docs

See [docs/](docs/) folder for comprehensive architecture documentation:
- `ARCHITECTURE.md` - System architecture overview
- `TESTING.md` - Testing framework
- `GRACEFUL_SHUTDOWN.md` - Shutdown procedures
- `ZABBIX_INTEGRATION.md` - Zabbix-specific details
- And more...

---

## Key Files by Use Case

### "I want to understand the project"
1. Start with [README.md](../README.md)
2. Read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
3. Check [PERFORMANCE_REVIEW.md](PERFORMANCE_REVIEW.md) for optimizations

### "I want to set up monitoring for 1000 hosts"
1. Follow [SCALING_GUIDE.md](SCALING_GUIDE.md) - complete step-by-step
2. Reference [QUICK_COMMANDS.sh](../scripts/QUICK_COMMANDS.sh) for commands
3. Check [TEST_REPORT.md](TEST_REPORT.md) for expected results

### "I want to run it locally"
1. Start with [QUICKSTART.md](QUICKSTART.md)
2. Use [QUICK_COMMANDS.sh](../scripts/QUICK_COMMANDS.sh) for commands
3. See [DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md) for details

### "I want to contribute"
1. Read [CONTRIBUTING.md](../CONTRIBUTING.md)
2. Check [GITHUB_CHECKLIST.md](GITHUB_CHECKLIST.md)
3. Review [INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md) for API details

### "I want to understand performance"
1. Review [PERFORMANCE_REVIEW.md](PERFORMANCE_REVIEW.md)
2. Read [OPTIMIZATIONS_README.md](OPTIMIZATIONS_README.md)
3. Check [TEST_REPORT.md](TEST_REPORT.md) for metrics

---

## Scripts Reference

All automation scripts are in the `scripts/` folder:

| Script | Purpose | Scaling |
|--------|---------|---------|
| `generate_rich_snmprec.py` | Generate SNMP OIDs | 1,876 OIDs |
| `add_remaining_hosts.py` | Add 900+ hosts to Zabbix | 100 → 1000 |
| `add_bulk_items.py` | Create ~1,500 items per host | 1,354 per host |
| `add_bulk_items_1000.py` | Alternative for 1000 hosts | With fixes |
| `link_templates.py` | Link Cisco template to hosts | All 1000 |
| `update_host_ips.py` | Update host IP addresses | Network fixing |
| `create_custom_template.py` | Build custom Zabbix template | Custom items |
| `check_host_config.py` | Verify host configuration | Diagnostics |
| `test_zabbix_api.py` | Test API connectivity | Validation |

**Usage**: All scripts located in `scripts/` folder
```bash
cd /path/to/go-snmpsim
python3 scripts/script_name.py
```

---

## Document Purpose Summary

### Primary Documents (READ FIRST!)

| Document | When to Read | Key Info |
|----------|--------------|----------|
| **[SCALING_GUIDE.md](SCALING_GUIDE.md)** | Setting up 1000 hosts | Complete how-to with phases |
| **[TEST_REPORT.md](TEST_REPORT.md)** | Validating deployment | Test results and metrics |
| **[README.md](../README.md)** | First time viewing project | Overview and quick start |

### Reference Documents

| Document | Reference Info |
|----------|----------------|
| [INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md) | Zabbix API parameters and examples |
| [DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md) | Docker configuration details |
| [PERFORMANCE_REVIEW.md](PERFORMANCE_REVIEW.md) | 5 key performance optimizations |
| [OPTIMIZATIONS_README.md](OPTIMIZATIONS_README.md) | How to implement optimizations |

### Supporting Documents

- [QUICKSTART.md](QUICKSTART.md) - 5-min setup
- [SETUP_COMPLETE.md](SETUP_COMPLETE.md) - Validation
- [WEB_UI_IMPLEMENTATION.md](WEB_UI_IMPLEMENTATION.md) - Web UI features
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Dev guidelines

---

## Latest Updates (February 18, 2026)

✅ **NEW**: Scaling to 1000 hosts completed
- ✅ SNMP Simulator: 1000 devices, 1876 OIDs
- ✅ Zabbix: 1000 hosts created
- ✅ Items: ~1,354,000 metrics deployed
- ✅ Polling: 5-minute intervals on all items
- ✅ Data: Actively collecting ~27,000 values/minute

✅ **NEW**: SNMPv3 (`noAuthNoPriv`) completed
- ✅ End-to-end SNMPv3 query path validated
- ✅ Zabbix SNMPv3 host provisioning validated
- ✅ 50 active hosts migrated to SNMPv3 (`cisco-iosxr-001` to `cisco-iosxr-050`)

**Documentation**:
- ✅ [SCALING_GUIDE.md](SCALING_GUIDE.md) - Complete scaling how-to
- ✅ [TEST_REPORT.md](TEST_REPORT.md) - Detailed test results
- ✅ Scripts moved to `scripts/` folder
- ✅ Repository reorganized for clarity

---

## File Organization

```
go-snmpsim/
├── README.md                    # Project overview
├── docs/                        # Primary and detailed docs
│   ├── SCALING_GUIDE.md         # ⭐ How to scale to 1000 hosts
│   ├── TEST_REPORT.md           # ⭐ Complete test results
│   ├── QUICKSTART.md            # Quick setup (5 min)
│   ├── ARCHITECTURE.md
│   ├── TESTING.md
│   ├── ZABBIX_INTEGRATION.md
│   └── ...
│
├── scripts/                     # Automation + command scripts
│   ├── QUICK_COMMANDS.sh
│   ├── add_remaining_hosts.py
│   ├── add_bulk_items.py
│   ├── generate_rich_snmprec.py
│   └── ...
│
├── cmd/                         # Go source code
│   └── snmpsim/main.go
│
├── internal/                    # Go packages
│   ├── agent/
│   ├── api/
│   ├── engine/
│   └── store/
│
└── zabbix/                      # Zabbix integration
    ├── docker-compose.zabbix.yml
    ├── zabbix_api_client.py
    ├── zabbix_config.yaml
    └── requirements.txt
```

---

## Version Information

| Component | Version | Notes |
|-----------|---------|-------|
| Zabbix | 7.4.7 | PostgreSQL backend |
| SNMP Simulator | Custom | Go implementation |
| Polling Scale | 1000 hosts | All at 5-minute interval |
| Metrics | 1,354,000 | ~1,354 per host |

---

## Getting Help

1. **For setup issues**: See [QUICKSTART.md](QUICKSTART.md)
2. **For 1000-host deployment**: See [SCALING_GUIDE.md](SCALING_GUIDE.md)
3. **For test validation**: See [TEST_REPORT.md](TEST_REPORT.md)
4. **For API questions**: See [INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md)
5. **For performance**: See [PERFORMANCE_REVIEW.md](PERFORMANCE_REVIEW.md)

---

Last Updated: February 19, 2026  
Status: ✅ Release v1.2 - SNMPv2c + SNMPv3 + routing + variation + stress suite ready
