# Web UI Quick Start Guide

This is a quick reference guide for getting started with the Go SNMPSim Web UI.

## 1. Build the Simulator

```bash
cd ~/trials/go-snmpsim
go build -o snmpsim ./cmd/snmpsim
```

## 2. Start the Simulator with Web UI

```bash
# Basic start (SNMP: 20000-30000, Web UI: 8080)
./snmpsim

# Or with custom settings
./snmpsim -port-start 10000 -port-end 15000 -devices 50 -web-port 8888
```

You should see output like:
```
Starting SNMP Simulator
SNMP Port range: 20000-30000
Number of devices: 100
Web UI port: 8080 (http://localhost:8080)
Starting web UI server on http://localhost:8080
```

## 3. Open Web Dashboard

Open your browser and navigate to:
```
http://localhost:8080
```

## 4. Dashboard Tabs Overview

### Dashboard Tab
- **Status Indicator**: Shows "Running" when simulator is active
- **Device Count**: Number of virtual SNMP agents
- **Port Range**: SNMP listen ports
- **Uptime**: How long simulator has been running
- **Metrics**: Poll counts and average response time

### Test SNMP Tab
1. Select test type (GET, BULKWALK, WALK)
2. Enter OIDs (one per line):
   - Single OID: `1.3.6.1.2.1.1.1.0` (sysDescr)
   - Multiple: Add each on a new line
3. Set port range (e.g., 20000-20009 for first 10 devices)
4. Configure community string (default: "public")
5. Set timeout (5-10 seconds recommended)
6. Click "Run Tests"
7. View results in the results table

### Workloads Tab
- **Save**: Name and save current test configuration
- **Load**: Choose from saved workloads
- **Delete**: Remove saved configurations

### Configuration Tab
- Feature showcase and system information

## 5. Quick Test Examples

### Test Basic OIDs
1. Go to Test SNMP tab
2. Enter OIDs:
   ```
   1.3.6.1.2.1.1.1.0
   1.3.6.1.2.1.1.3.0
   1.3.6.1.2.1.1.5.0
   ```
3. Set ports: 20000-20009
4. Community: public
5. Click "Run Tests"

### Test Interface Table
1. Select Test Type: BULKWALK
2. Enter OID: `1.3.6.1.2.1.2.2`
3. Set ports: 20000-20000 (single device)
4. Max Repeaters: 10
5. Click "Run Tests"

### Save a Test Configuration
1. Configure test as desired
2. Fill in name and description
3. Click "Save Workload"
4. Workload appears in Workloads tab

## 6. Troubleshooting

### Can't Access http://localhost:8080
- Ensure simulator is running
- Check if port 8080 is already in use: `sudo lsof -i :8080`
- Try different port: `./snmpsim -web-port 9090`

### SNMP Tests Return "Failed"
- Verify simulator is running (status shows "Running")
- Check snmpget/snmpwalk is installed: `which snmpget`
- Increase timeout value (network might be slow)
- Try ports that definitely exist (20000-20005)

### Slow Test Execution
- Reduce number of ports being tested
- Reduce number of OIDs
- Increase timeout slightly
- Check system resources (CPU, memory)

## 7. API Endpoints (Advanced)

You can also interact with the API directly:

```bash
# Check status
curl http://localhost:8080/api/status

# Run test
curl -X POST http://localhost:8080/api/test/snmp \
  -H "Content-Type: application/json" \
  -d '{
    "test_type": "get",
    "oids": ["1.3.6.1.2.1.1.1.0"],
    "port_start": 20000,
    "port_end": 20009,
    "community": "public",
    "timeout": 5
  }'

# List workloads
curl http://localhost:8080/api/workloads
```

## 8. Key Features

✅ **Status Monitoring** - Real-time simulator metrics (updates every 2 sec)
✅ **SNMP Testing** - GET, BULKWALK, WALK operations on multiple devices
✅ **Performance Metrics** - Latency, success rate, error tracking
✅ **Test Configuration** - Save/load reusable test workloads
✅ **Default Workloads** - 4 pre-built test templates ready to use
✅ **Responsive Design** - Works on desktop, tablet, and mobile
✅ **No Setup Needed** - Pure HTML/CSS/JavaScript, no npm/build tools

## 9. System Requirements

- Go 1.16+ (to build)
- Linux/Unix (SNMP uses UDP sockets)
- net-snmp tools: `snmpget`, `snmpwalk` (install via apt/brew/yum)

Install net-snmp tools:
```bash
# Ubuntu/Debian
sudo apt-get install snmp

# macOS
brew install net-snmp

# CentOS/RHEL
sudo yum install net-snmp-utils
```

## 10. Common Tasks

### Test All Devices
Test Type: GET
OID: `1.3.6.1.2.1.1.1.0` (sysDescr)
Port Range: 20000 - 20099 (if 100 devices)

### Find System Uptime
Test Type: GET
OID: `1.3.6.1.2.1.1.3.0` (sysUptime.0)
Port Range: 20000 - 20009

### Get Interface Details
Test Type: BULKWALK
OID: `1.3.6.1.2.1.2.2.1` (interface table)
Max Repeaters: 20
Port Range: 20000 - 20000 (single device)

### Full Device Walk
Test Type: WALK
OID: `1.3.6.1.2.1` (all devices)
Port Range: 20001 - 20001 (single device)

## 11. File Locations

```
~/trials/go-snmpsim/
├── snmpsim                    # Compiled binary
├── web/ui/index.html          # Dashboard
├── web/assets/
│   ├── style.css             # Styling
│   └── app.js                # JavaScript logic
├── config/workloads/         # Saved workload JSON files
└── docs/WEB_UI.md            # Full documentation
```

## 12. Next Steps

1. **Learn SNMP**: Read SNMP documentation to understand OID structure
2. **Explore OIDs**: Use test tab to discover available OIDs in simulator
3. **Create Workloads**: Save custom test configurations for reuse
4. **Integrate**: Use API for custom monitoring/testing applications
5. **Production**: See WEB_UI.md for security recommendations

## 13. Getting Help

For detailed information, see:
- [docs/WEB_UI.md](docs/WEB_UI.md) - Full documentation
- [docs/README.md](docs/README.md) - General simulator docs
- GitHub Issues: https://github.com/debashish-mukherjee/go-snmpsim/issues

## 14. Default Test Workloads

The UI comes with 4 pre-configured test workloads:

1. **Basic System OIDs** - System info (sysDescr, sysUptime, sysServices)
2. **Interface Metrics** - Interface stats (ifDescr, ifInOctets, ifOutOctets)
3. **Full System Walk** - Complete system subtree (1.3.6.1.2.1.1)
4. **48-Port Switch Test** - Interface table testing (1.3.6.1.2.1.2.2.1)

Click the workload name in Test SNMP tab to load it immediately.

---

**Happy Testing!** 🎉

For more information, see [docs/WEB_UI.md](../docs/WEB_UI.md)
