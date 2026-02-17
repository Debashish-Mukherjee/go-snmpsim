# Docker Deployment Guide

This guide explains how to deploy the Go SNMP Simulator using Docker and Docker Compose.

## Prerequisites

- Docker 20.10+ installed
- Docker Compose 1.29+ (optional, for compose deployment)
- Approximately 500MB disk space for the image

## Quick Start

### Using Makefile (Recommended)

```bash
# Build and start the simulator with Alpine Linux base
make docker-start

# View logs
make docker-logs

# Stop the simulator and clean up
make docker-stop
```

**Access points:**

- Web Dashboard: `http://localhost:8080`
- SNMP Ports: `localhost:20000-30000` (UDP)
- Container name: `snmpsim-alpine`

### Using Docker CLI Directly

```bash
# Build the image
docker build -t go-snmpsim:latest .

# Run the container
docker run -d \
  --name snmpsim-alpine \
  -p 8080:8080 \
  -p 20000-30000:20000-30000/udp \
  -v $(pwd)/config:/app/config \
  go-snmpsim:latest

# View logs
docker logs -f snmpsim-alpine

# Stop the container
docker stop snmpsim-alpine
docker rm snmpsim-alpine
```

## Usage Examples

### 1. Basic Deployment (100 devices)

```bash
docker-compose up -d
```

This starts:

- 100 virtual SNMP devices on ports 20000-20099
- Web UI on port 8080
- Net-SNMP tools pre-installed for testing

### 2. Custom Configuration

Edit `docker-compose.yml` and modify the `command` section:

```yaml
command: [
  "-port-start=10000",
  "-port-end=15000",
  "-devices=50",
  "-web-port=8080",
  "-listen=0.0.0.0"
]
```

Then restart:

```bash
docker-compose up -d --force-recreate
```

### 3. Using Docker CLI with Custom Settings

```bash
docker run -d \
  --name snmpsim \
  -p 8080:8080 \
  -p 10000-15000:10000-15000/udp \
  -e GOMAXPROCS=4 \
  go-snmpsim:latest \
  -port-start=10000 \
  -port-end=15000 \
  -devices=50 \
  -web-port=8080
```

## Available Flags

```
SNMP Configuration:
  -port-start     Starting SNMP port (default: 20000)
  -port-end       Ending SNMP port (default: 30000)
  -devices        Number of virtual devices (default: 100)
  -listen         Bind address (default: 0.0.0.0)
  -snmprec        Path to .snmprec file (optional)

Web UI Configuration:
  -web-port       Web UI port (default: 8080)
```

## Working with Web UI

### Accessing the Dashboard

1. Build and start with docker-compose:

   ```bash
   docker-compose up -d
   ```

2. Open browser: `http://localhost:8080`

3. Dashboard shows:
   - Simulator status
   - Device count
   - SNMP port range
   - Uptime and metrics

### Testing from Web UI

1. Go to **Test SNMP** tab
2. Configure test:
   - Test Type: GET
   - OIDs: Enter OIDs (one per line)
   - Port Range: 20000-20099 (for all 100 devices)
   - Community: public
   - Timeout: 5 seconds
3. Click **Run Tests**
4. View results with latency metrics

## Testing with SNMP Tools

The Docker image includes `snmpget` and `snmpwalk` commands.

### From Host Machine

If you have net-snmp installed:

```bash
# Test a single device
snmpget -v 2c -c public -t 5 localhost:20000 1.3.6.1.2.1.1.1.0

# Walk device tree
snmpwalk -v 2c -c public -t 5 localhost:20000 1.3.6.1.2.1.1
```

### From Within Container

```bash
# Execute into container
docker exec -it snmpsim /bin/sh

# Inside container, test another device
snmpget -v 2c -c public snmpsim:20001 1.3.6.1.2.1.1.1.0

# Or use docker-compose
docker-compose exec snmpsim /bin/sh
```

### Using the Client Container (Optional)

```bash
# Start client container (already has SNMP tools)
docker-compose run snmpsim-client /bin/sh

# Inside client, test:
snmpget -v 2c -c public snmpsim:20000 1.3.6.1.2.1.1.1.0
```

## Volume Management

### Persistent Workload Storage

Workload configurations are stored in `./config/workloads/`:

```bash
# View saved workloads
ls -la ./config/workloads/

# Save workloads from the web UI
# They are automatically persisted to the host volume
```

### Configuration Directory

```
./config/
└── workloads/
    ├── Basic System OIDs.json
    ├── Interface Metrics.json
    ├── Full System Walk.json
    └── 48-Port Switch Test.json
```

## Health Checks

The container includes a health check that:

- Checks HTTP endpoint every 30 seconds
- Waits 5 seconds before first check
- Fails after 3 consecutive failures

**View health status:**

```bash
docker ps  # Shows health status in output
docker inspect snmpsim | grep -A 5 Health
```

## Debugging

### View Logs

```bash
# Follow logs in real-time
docker-compose logs -f snmpsim

# View last 100 lines
docker-compose logs --tail=100 snmpsim

# View logs with timestamps
docker-compose logs -f --timestamps snmpsim
```

### Execute Commands in Container

```bash
# Run shell
docker-compose exec snmpsim /bin/sh

# Run single command
docker-compose exec snmpsim snmpget -v 2c -c public localhost:20000 1.3.6.1.2.1.1.1.0

# Run with docker
docker exec -it snmpsim /bin/sh
```

### Check Container Status

```bash
# Detailed status
docker-compose ps

# Full container inspection
docker inspect snmpsim
```

## Performance Tuning

### CPU and Memory Limits

Edit `docker-compose.yml`:

```yaml
deploy:
  resources:
    limits:
      cpus: '4'
      memory: 2G
    reservations:
      cpus: '2'
      memory: 1G
```

### GOMAXPROCS

Control Go runtime threads:

```yaml
environment:
  - GOMAXPROCS=8  # Adjust based on CPU cores
```

Or via CLI:

```bash
docker run -e GOMAXPROCS=8 go-snmpsim:latest
```

### Network Optimization

For best performance with many devices:

- Increase system file descriptor limit: `ulimit -n 65536`
- Use host network mode: `--network host` (Linux only)

## Networking

### Port Mapping

```bash
# Standard (routed through Docker)
docker run -p 20000-30000:20000-30000/udp go-snmpsim:latest

# Host mode (direct access, Linux only)
docker run --network host go-snmpsim:latest
```

### Container Communication

Containers can communicate via service name in docker-compose:

```bash
# From inside client container
snmpget -v 2c -c public snmpsim:20000 1.3.6.1.2.1.1.1.0
```

## Logging

### Log Rotation

Configure in `docker-compose.yml`:

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

### View Logs

```bash
# Real-time
docker-compose logs -f

# Historical
docker logs snmpsim | tail -50
```

## Cleanup

### Remove Container

```bash
docker-compose down
```

### Remove Image

```bash
docker rmi go-snmpsim:latest
```

### Full Cleanup

```bash
# Stop and remove containers
docker-compose down

# Remove volumes (careful!)
docker-compose down -v

# Prune unused images and volumes
docker image prune -a
docker volume prune
```

## Troubleshooting

### Web UI Not Accessible

```bash
# Check if port is mapped
docker-compose ps

# Verify port is open
netstat -an | grep 8080

# Test endpoint
curl http://localhost:8080/api/status

# Check logs
docker-compose logs snmpsim
```

### SNMP Tests Failing

**In Web UI:**

1. Check status shows "Running"
2. Verify net-snmp tools are available:

   ```bash
   docker-compose exec snmpsim which snmpget
   ```

3. Test manually:

   ```bash
   docker-compose exec snmpsim snmpget -v 2c -c public localhost:20000 1.3.6.1.2.1.1.1.0
   ```

### Container Won't Start

```bash
# Check logs
docker-compose logs snmpsim

# Rebuild image
docker-compose build --no-cache

# Try running with verbose output
docker-compose up snmpsim  # (without -d)
```

### Out of File Descriptors

```bash
# Check current limit
ulimit -n

# Increase (Linux)
ulimit -n 65536

# Permanent setting in docker-compose.yml
ulimit:
  nofile:
    soft: 65536
    hard: 65536
```

## Production Deployment

### Using Docker Swarm

```bash
docker stack deploy -c docker-compose.yml snmpsim
```

### Using Kubernetes

Convert docker-compose to Kubernetes manifests:

```bash
kompose convert -f docker-compose.yml -o k8s/
```

### Environment Variables

Create `.env` file:

```
PORT_START=20000
PORT_END=30000
DEVICES=100
WEB_PORT=8080
GOMAXPROCS=8
```

Reference in docker-compose.yml:

```yaml
command: [
  "-port-start=${PORT_START}",
  "-port-end=${PORT_END}",
  "-devices=${DEVICES}",
  "-web-port=${WEB_PORT}"
]
```

## Security Considerations

### Network Isolation

- Use bridge networks (default in compose)
- Restrict port access with firewall rules
- Don't expose to untrusted networks without authentication

### Authentication

- Add authentication layer with reverse proxy (nginx)
- Use environment-specific credentials
- Implement SNMP v3 with authentication

### Example Nginx Configuration

```nginx
server {
    listen 8080;
    server_name localhost;
    
    auth_basic "SNMP Simulator";
    auth_basic_user_file /etc/nginx/.htpasswd;
    
    location / {
        proxy_pass http://snmpsim:8080;
    }
}
```

## References

- [Docker Documentation](https://docs.docker.com/)
- [Docker Compose Reference](https://docs.docker.com/compose/compose-file/)
- [Net-SNMP Documentation](http://www.net-snmp.org/)

## Support

For issues or questions:

- Check Docker logs: `docker-compose logs`
- Review this guide's troubleshooting section
- Open an issue on GitHub: <https://github.com/debashish-mukherjee/go-snmpsim>
