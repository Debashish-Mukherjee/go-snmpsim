# Docker Deployment Guide - v1.5

This guide covers deploying the SNMP Simulator API, Prometheus metrics collection, and Grafana visualization using Docker Compose.

## Prerequisites

- Docker 20.10+ and Docker Compose 2.0+
- 2GB+ available memory
- Ports available: 8080 (API), 9090 (Prometheus), 9091 (Prometheus Web), 3000 (Grafana)

## Quick Start

### 1. Clone and Navigate

```bash
cd /path/to/go-snmpsim
```

### 2. Start the Stack

```bash
docker-compose up -d
```

This will start:
- **snmpsim-api** - REST API on http://localhost:8080
- **snmpsim-simulator** - SNMP simulator on ports 10000-10100/udp
- **snmpsim-prometheus** - Metrics storage on http://localhost:9091
- **snmpsim-grafana** - Visualization on http://localhost:3000

### 3. Verify Services

```bash
# Check API health
curl http://localhost:8080/health

# Check Prometheus scrape targets
curl http://localhost:9091/api/v1/targets

# Access Grafana
# Visit http://localhost:3000
# Login: admin / admin
```

### 4. Create and Start a Lab

```bash
# Create an engine
ENGINE=$(curl -s -X POST http://localhost:8080/engines \
  -H "Content-Type: application/json" \
  -d '{
    "name": "docker-engine",
    "engine_id": "800007e5",
    "listen_addr": "snmpsim",
    "port_start": 10000,
    "port_end": 10050,
    "num_devices": 20
  }' | jq -r '.id')

# Create a lab  
LAB=$(curl -s -X POST http://localhost:8080/labs \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"docker-lab\",
    \"engine_id\": \"$ENGINE\"
  }" | jq -r '.id')

# Start the lab
curl -s -X POST http://localhost:8080/labs/$LAB/start | jq

# Monitor metrics
curl -s http://localhost:9091/api/v1/query?query=snmpsim_labs_active
```

### 5. Access Grafana Dashboard

1. Navigate to http://localhost:3000
2. Login with credentials: `admin` / `admin`
3. Select "SNMP Simulator Metrics" dashboard
4. View real-time metrics for packets/sec, failures, latency, and active agents

### 6. Stop Services

```bash
docker-compose down
```

To also remove persistent storage:

```bash
docker-compose down -v
```

---

## Configuration

### Environment Variables

Create a `.env` file to customize settings:

```bash
# API Configuration
API_ADDR=0.0.0.0:8080
METRICS_ADDR=0.0.0.0:9090

# Simulator Configuration
LISTEN_ADDR=0.0.0.0
PORT_START=10000
PORT_END=10100
NUM_DEVICES=50
SNMPREC_FILE=./sample.snmprec

# Prometheus Configuration
SCRAPE_INTERVAL=15s
EVALUATION_INTERVAL=15s
```

Then update `docker-compose.yml` to use these variables:

```yaml
environment:
  - API_ADDR=${API_ADDR}
  - METRICS_ADDR=${METRICS_ADDR}
```

### Volume Mounts

To persist data or inject custom files:

```yaml
snmpsim:
  volumes:
    - ./custom-data:/app/data
    - ./custom.snmprec:/app/custom.snmprec
```

---

## Monitoring and Logging

### View Docker Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f api
docker-compose logs -f snmpsim-prometheus
```

### Check Service Status

```bash
docker-compose ps
```

### Real-time Metrics Scrape

```bash
# Stream metrics from Prometheus
curl -s http://localhost:9091/metrics | head -50

# Query specific metric
curl -s 'http://localhost:9091/api/v1/query?query=snmpsim_packets_total' | jq
```

---

## Troubleshooting

### API Not Responding

```bash
# Check if container is running
docker-compose ps api

# View logs
docker-compose logs api

# Verify port binding
docker-compose port api 8080
```

### Prometheus Not Scraping Metrics

```bash
# Check targets in Prometheus UI
curl http://localhost:9091/api/v1/targets

# Check if API metrics endpoint is responding
curl http://api:9090/metrics
```

### Grafana Dashboard Blank

1. Verify Prometheus data source is configured: Grafana → Configuration → Data Sources
2. Check if metrics are being collected: Run a lab and query Prometheus directly
3. Verify dashboard JSON is valid in `/docker/grafana/dashboards/`

### Memory Issues

If containers are exiting due to memory limits:

```yaml
services:
  api:
    deploy:
      resources:
        limits:
          memory: 1G
  prometheus:
    deploy:
      resources:
        limits:
          memory: 1G
```

---

## Scaling

### Run Multiple Simulators

Create separate services in `docker-compose.yml`:

```yaml
snmpsim-2:
  image: golang:1.22-alpine
  working_dir: /app
  volumes:
    - .:/app
  command: >
    /tmp/snmpsim
    --listen-addr=0.0.0.0
    --port-start=11000
    --port-end=11100
    --num-devices=50
  ports:
    - "11000-11100:11000-11100/udp"
  networks:
    - snmpsim-net
```

### Load Test Labs

```bash
# Create and start 10 labs concurrently
for i in {1..10}; do
  ENGINE=$(curl -s -X POST http://localhost:8080/engines \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"engine-$i\",
      \"engine_id\": \"800007e$(printf '%x' $i)\",
      \"listen_addr\": \"snmpsim\",
      \"port_start\": $((10000 + i * 100)),
      \"port_end\": $((10099 + i * 100)),
      \"num_devices\": 20
    }" | jq -r '.id')
  
  LAB=$(curl -s -X POST http://localhost:8080/labs \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"lab-$i\", \"engine_id\": \"$ENGINE\"}" | jq -r '.id')
  
  curl -s -X POST http://localhost:8080/labs/$LAB/start > /dev/null
  echo "Started lab-$i"
done
```

---

## Performance Tuning

### Optimize Prometheus Storage

```yaml
prometheus:
  command:
    - "--config.file=/etc/prometheus/prometheus.yml"
    - "--storage.tsdb.path=/prometheus"
    - "--storage.tsdb.retention.time=7d"  # Retention period
    - "--storage.tsdb.retention.size=1GB"  # Max storage
```

### Increase Scrape Timeout

```yaml
# In docker/prometheus.yml
global:
  scrape_interval: 10s         # More frequent scraping
  evaluation_interval: 10s
  scrape_timeout: 5s          # Timeout for scrapes
```

### Enable Compression

```yaml
api:
  environment:
    - GOMAXPROCS=4  # CPU cores
```

---

## Cleanup and Maintenance

### Remove All Data

```bash
docker-compose down -v --remove-orphans
```

### Update Images

```bash
docker-compose pull
docker-compose up -d --build
```

### Backup Prometheus Data

```bash
docker cp snmpsim-prometheus:/prometheus ./prometheus-backup-$(date +%s)
```

### Restore Prometheus Data

```bash
docker cp ./prometheus-backup-<timestamp> snmpsim-prometheus:/prometheus
docker-compose restart prometheus
```

---

## Security

### Change Default Credentials

Update Grafana admin password via environment:

```yaml
grafana:
  environment:
    - GF_SECURITY_ADMIN_PASSWORD=your-secure-password
```

### Enable HTTPS

Use a reverse proxy (nginx/traefik) in front of the stack:

```yaml
reverse-proxy:
  image: traefik:latest
  ports:
    - "443:443"
  volumes:
    - ./traefik.yml:/traefik.yml
    - ./ssl:/ssl
```

### Network Isolation

Use a custom network and restrict container communication:

```yaml
networks:
  snmpsim-net:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/16
```

---

## Integration Examples

### Alert on High Failure Rate

In Prometheus, create an alert rule (`docker/prometheus.yml`):

```yaml
alert_rules:
  - alert: HighSNMPFailureRate
    expr: rate(snmpsim_failures_total[5m]) > 0.1
    for: 5m
    annotations:
      summary: "High SNMP failure rate detected"
```

### Export Metrics to InfluxDB

Add an InfluxDB service and configure Prometheus remote write:

```yaml
influxdb:
  image: influxdb:latest
  environment:
    - INFLUXDB_DB=snmpsim
```

Update `docker/prometheus.yml`:

```yaml
remote_write:
  - url: "http://influxdb:8086/api/v1/prom/write?db=snmpsim"
```

---

For more API documentation, see [REST_API.md](REST_API.md).

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
