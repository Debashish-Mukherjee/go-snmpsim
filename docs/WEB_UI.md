# Web UI Dashboard

Go SNMPSim includes a modern web-based dashboard for managing and testing the SNMP simulator.

## Overview

The Web UI provides:

- **Simulator Status Monitoring** - Real-time status of running simulator instance
- **SNMP Testing** - Execute SNMP GET, BULKWALK, and WALK operations on simulated devices
- **Workload Management** - Save, load, and manage test configurations
- **Dashboard Metrics** - View simulator uptime, poll statistics, and performance metrics

## Starting the Simulator with Web UI

```bash
# Run with default settings (SNMP: 20000-30000, Web UI: :8080)
./snmpsim

# Run with custom ports
./snmpsim -port-start 10000 -port-end 15000 -devices 50 -web-port 8888

# Run with OID file
./snmpsim -snmprec path/to/file.snmprec -web-port 8080
```

### Available Flags

- `-port-start` (default: 20000) - First SNMP port to listen on
- `-port-end` (default: 30000) - Last SNMP port to listen on
- `-devices` (default: 100) - Number of virtual devices to simulate
- `-snmprec` - Path to .snmprec file with OID definitions (optional)
- `-listen` (default: 0.0.0.0) - Bind address for SNMP listeners
- `-web-port` (default: 8080) - Port for Web UI API server

## Accessing the Dashboard

Once the simulator is running, open your browser and visit:

```
http://localhost:8080
```

The dashboard is accessible from any machine on your network if the simulator is bound to `0.0.0.0`.

## Web UI Features

### Dashboard Tab

**Status Panel:**

- Current simulator state (Running/Stopped)
- Number of virtual devices
- Listening address and port range
- Uptime
- Total polls and average response latency

**Control Panel:**

- Port range configuration (start/end)
- Device count
- Listener address configuration

**Metrics:**

- Successful SNMP polls
- Failed SNMP polls
- Average latency
- Success rate percentage

### Test SNMP Tab

**Configuration:**

- Test Type: GET, GETNEXT, BULKWALK, or WALK
- OID List: Multi-line OID input (one OID per line)
- Port Range: Start and end ports to test
- Community String: SNMP community (default: "public")
- Timeout: Request timeout in seconds
- Max Repeaters: For BULKWALK operations

**Quick Workloads:**

- Launch pre-configured test workloads
- Includes common test scenarios out-of-the-box

**Results:**

- Summary statistics (total, successful, failed, success rate)
- Latency metrics (average, minimum, maximum)
- Detailed results table showing:
  - Target port
  - OID queried
  - Success/failure status
  - Returned value (if successful)
  - SNMP value type
  - Response latency in milliseconds

### Workloads Tab

**Save Current Configuration:**

- Name the current test configuration
- Add description
- Save to disk for reuse

**Manage Saved Workloads:**

- View all saved workload configurations
- Load workload (populates test configuration)
- Delete workload

**Default Workloads:**

1. **Basic System OIDs** - System description, uptime, services
2. **Interface Metrics** - Interface descriptions and octets counters
3. **Full System Walk** - Complete system subtree walk
4. **48-Port Switch Test** - Interface table testing

### Configuration Tab

- Feature showcase
- System information display
- Planned features and roadmap
- API documentation

## Architecture

### Frontend

Located in `web/ui/` and `web/assets/`:

- **index.html** - Dashboard UI with 4 main tabs
- **style.css** - Professional dark-theme styling with responsive layout
- **app.js** - JavaScript client for API communication and interactivity

**Features:**

- No build dependencies - pure HTML/CSS/JavaScript
- Mobile-responsive design
- Real-time status polling (updates every 2 seconds)
- Tab-based navigation
- Form validation

### Backend API

Located in `internal/api/` and `internal/webui/`:

#### API Server (`internal/api/server.go`)

REST endpoints:

- `GET /api/status` - Current simulator metrics
- `POST /api/start` - Create and start a simulator instance with the provided parameters
- `POST /api/stop` - Stop the active simulator instance and release listeners/resources
- `POST /api/test/snmp` - Execute SNMP tests
- `GET /api/workloads` - List saved workloads
- `POST /api/workloads/save` - Save workload configuration
- `GET /api/workloads/load` - Load workload by name
- `DELETE /api/workloads/delete` - Delete workload
- `GET /api/test/results` - Retrieve last test results

Behavior and error handling:

- Starting while a simulator is already running returns `409 Conflict`
- Invalid port ranges (for example, `port_end <= port_start`) return `400 Bad Request`
- SNMP test endpoints return `503 Service Unavailable` if SNMP tester is not configured
- Workload endpoints return `503 Service Unavailable` if workload manager is not configured

#### SNMP Tester (`internal/webui/snmp_tester.go`)

Executes SNMP operations:

- Spawns `snmpget` and `snmpwalk` commands from net-snmp tools
- Supports GET, BULKWALK, and WALK operations
- Calculates latency statistics
- Thread-safe concurrent testing
- Returns detailed results with timestamps and error summaries

#### Workload Manager (`internal/webui/workload_manager.go`)

Manages test configurations:

- Persists workloads to disk as JSON files (in `config/workloads/`)
- In-memory caching with disk synchronization
- CRUD operations for workload configurations
- Default workload templates
- Atomic file operations

## Requirements

### System Requirements

- Go 1.16 or higher
- Linux/Unix system (uses UDP sockets)
- File descriptor limit: at least (port_range + 200) open files
- net-snmp tools installed (`snmpget`, `snmpwalk`)

### Installing net-snmp Tools

```bash
# Ubuntu/Debian
sudo apt-get install snmp

# CentOS/RHEL
sudo yum install net-snmp-utils

# macOS
brew install net-snmp
```

## Performance Notes

- Status updates poll every 2 seconds
- SNMP test execution runs all ports sequentially to avoid overwhelming the network
- Results are cached and returned immediately on subsequent requests
- Workload configurations stored as JSON files (no database required)

## Security Considerations

**MVP Limitations (v1.0):**

- No authentication/authorization
- No HTTPS/TLS (should be added for production)
- SNMP community string transmitted over HTTP
- No rate limiting

**Recommendations for Production:**

1. Run behind a reverse proxy with HTTPS
2. Implement authentication (API keys, OAuth, etc.)
3. Add authorization levels (read-only, test-only, admin)
4. Use SNMP v3 with authentication/encryption
5. Implement rate limiting and request validation
6. Run in an isolated network or use a VPN

## Troubleshooting

### Web UI Not Accessible

- Verify simulator is running: `./snmpsim -web-port 8080`
- Check if port is in use: `netstat -an | grep 8080`
- Check firewall rules if accessing from another machine
- Verify binding address: `./snmpsim -listen 0.0.0.0`

### SNMP Tests Failing

- Verify simulator is actually running (status shows "Running")
- Check that snmpget/snmpwalk are installed and in PATH
- Verify port range configuration matches running simulator
- Check community string (default: "public")
- Try individual ports first before testing ranges

### Tests Timeout

- Increase timeout value in test configuration
- Check network connectivity to localhost (127.0.0.1)
- Verify simulator is not overloaded (check CPU/memory)
- Try with fewer devices or ports in test range

### Workload Not Saving

- Verify `config/workloads/` directory exists and is writable
- Check file permissions: `ls -la config/`
- Ensure disk has available space
- Check browser console for JavaScript errors

## API Examples

### Using curl

```bash
# Get simulator status
curl http://localhost:8080/api/status

# Run SNMP GET test
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

# List saved workloads
curl http://localhost:8080/api/workloads

# Save workload
curl -X POST http://localhost:8080/api/workloads/save \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Workload",
    "description": "Test configuration",
    "test_type": "get",
    "oids": ["1.3.6.1.2.1.1.1.0"],
    "port_start": 20000,
    "port_end": 20009,
    "community": "public",
    "timeout": 5
  }'
```

### Using JavaScript

```javascript
// Get simulator status
fetch('/api/status')
  .then(r => r.json())
  .then(status => console.log(status));

// Run tests
fetch('/api/test/snmp', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    test_type: 'get',
    oids: ['1.3.6.1.2.1.1.1.0'],
    port_start: 20000,
    port_end: 20009,
    community: 'public',
    timeout: 5
  })
})
.then(r => r.json())
.then(results => console.log(results));
```

## Future Enhancements

Planned improvements:

- [ ] Real-time WebSocket updates (reduce polling)
- [ ] Test result history and charts
- [ ] Multiple simulator instances (load balancing)
- [ ] User authentication and role-based access
- [ ] OID autocomplete/search
- [ ] Bulk test scheduling
- [ ] Performance graphs and analytics
- [ ] Docker containerization
- [ ] OpenAPI/Swagger documentation
- [ ] Custom SNMP value generation

## File Structure

```
go-snmpsim/
├── cmd/snmpsim/
│   └── main.go                    # Entry point with API server setup
├── internal/
│   ├── api/
│   │   └── server.go             # HTTP API server and endpoints
│   ├── webui/
│   │   ├── snmp_tester.go        # SNMP test executor
│   │   └── workload_manager.go   # Workload configuration manager
│   ├── engine/                   # Simulator engine
│   ├── agent/                    # Virtual SNMP agents
│   └── store/                    # OID database
├── web/
│   ├── ui/
│   │   └── index.html            # Dashboard HTML
│   └── assets/
│       ├── style.css             # Stylesheet
│       └── app.js                # JavaScript client
└── config/
    └── workloads/                # Saved workload configurations
```

## Contributing

To extend the Web UI:

1. **Frontend Changes:** Edit `web/ui/index.html`, `web/assets/style.css`, `web/assets/app.js`
2. **Backend API:** Modify `internal/api/server.go` to add new endpoints
3. **Testing Features:** Extend `internal/webui/snmp_tester.go` for new test types
4. **Workload Options:** Update workload managers and defaults

## Support

For issues or feature requests, please open an issue on GitHub:
<https://github.com/debashish-mukherjee/go-snmpsim>
