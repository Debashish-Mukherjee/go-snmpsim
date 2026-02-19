# v1.5 Quick Start Guide

## REST API Overview

v1.5 introduces a minimal REST API service (`snmpsim-api`) for managing SNMP simulators as **Labs** with full lifecycle control and Prometheus metrics exposure.

## Quick Start (5 minutes)

### 1. Build the API

```bash
go build -o snmpsim-api ./cmd/snmpsim-api/main.go
```

### 2. Start the API Server

```bash
./snmpsim-api --api-addr=127.0.0.1:8080 --metrics-addr=127.0.0.1:9090
```

You'll see:
```
2024/01/15 10:30:00 Starting API server on 127.0.0.1:8080
2024/01/15 10:30:00 Starting metrics server on 127.0.0.1:9090
```

### 3. Create an Engine (Simulator Configuration)

```bash
ENGINE=$(curl -s -X POST http://127.0.0.1:8080/engines \
  -H "Content-Type: application/json" \
  -d '{
    "name": "lab-engine",
    "engine_id": "800007e5",
    "listen_addr": "127.0.0.1",
    "port_start": 10000,
    "port_end": 10010,
    "num_devices": 5
  }' | jq -r '.id')

echo "Created engine: $ENGINE"
```

### 4. Create a Lab

```bash
LAB=$(curl -s -X POST http://127.0.0.1:8080/labs \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"my-lab\",
    \"engine_id\": \"$ENGINE\"
  }" | jq -r '.id')

echo "Created lab: $LAB"
```

### 5. Start the Lab (Starts the Simulator)

```bash
curl -s -X POST http://127.0.0.1:8080/labs/$LAB/start | jq
```

Response shows status changed to `"running"`.

### 6. Monitor Metrics

```bash
# Check active labs
curl -s http://127.0.0.1:9090/metrics | grep snmpsim_labs_active

# Check active agents
curl -s http://127.0.0.1:9090/metrics | grep snmpsim_agents_active
```

### 7. Stop the Lab

```bash
curl -s -X POST http://127.0.0.1:8080/labs/$LAB/stop | jq
```

## Docker Compose (10 minutes)

Complete stack with API, Simulator, Prometheus, and Grafana:

```bash
docker-compose up -d
```

Services available:
- **API:** http://localhost:8080
- **Prometheus:** http://localhost:9091
- **Grafana:** http://localhost:3000 (admin/admin)

Create a lab:

```bash
curl -X POST http://localhost:8080/labs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "docker-lab",
    "engine_id": "engine-1"
  }' | jq
```

View Grafana dashboard at http://localhost:3000 after creating and starting a lab.

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/labs` | Create lab |
| GET | `/labs` | List labs |
| GET | `/labs/{id}` | Get lab details |
| POST | `/labs/{id}/start` | Start lab (run simulator) |
| POST | `/labs/{id}/stop` | Stop lab |
| DELETE | `/labs/{id}` | Delete lab |
| POST | `/engines` | Create engine config |
| GET | `/engines` | List engines |
| GET | `/engines/{id}` | Get engine details |
| DELETE | `/engines/{id}` | Delete engine |
| POST | `/endpoints` | Create endpoint (address:port) |
| POST | `/users` | Create user profile |
| POST | `/datasets` | Create dataset reference |
| GET | `/health` | Health check |

## Metrics Available

- `snmpsim_labs_total` - Total labs created
- `snmpsim_labs_active` - Currently running labs
- `snmpsim_packets_total` - SNMP packets processed
- `snmpsim_failures_total` - Operation failures
- `snmpsim_latency_seconds` - Operation latency histogram
- `snmpsim_agents_active` - Virtual agents per lab

## Testing

Run all tests:

```bash
cd cmd/snmpsim-api && go test -v
```

Expected output:
```
=== RUN   TestLabsCRUD
--- PASS: TestLabsCRUD (0.00s)
=== RUN   TestEnginesCRUD
--- PASS: TestEnginesCRUD (0.00s)
... (7 more tests)
PASS
ok      github.com/debashish-mukherjee/go-snmpsim/cmd/snmpsim-api       0.017s
```

## Full Documentation

- **REST API Reference:** [docs/REST_API.md](docs/REST_API.md)
- **Docker Deployment:** [docs/DOCKER_DEPLOYMENT.md](docs/DOCKER_DEPLOYMENT.md)
- **Release Notes:** [docs/RELEASE_NOTES_v1.5.md](docs/RELEASE_NOTES_v1.5.md)

## Troubleshooting

**API not responding?**
```bash
curl -s http://127.0.0.1:8080/health
# Should return: {"status":"ok"}
```

**Metrics not updating?**
```bash
# Check if lab is actually running
curl -s http://127.0.0.1:8080/labs | jq '.[] | select(.status=="running")'
```

**Docker containers failing?**
```bash
docker-compose logs -f api
docker-compose logs -f snmpsim
```

## Next Steps

1. Create multiple engines with different configurations
2. Run concurrent labs and monitor metrics
3. Integrate with existing SNMP monitoring tools (Zabbix, Nagios)
4. View Grafana dashboards for trend analysis
5. Export metrics to InfluxDB/Prometheus for long-term storage

See [docs/DOCKER_DEPLOYMENT.md](docs/DOCKER_DEPLOYMENT.md) for advanced usage and integration examples.
