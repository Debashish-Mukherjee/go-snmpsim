# REST API Documentation - v1.5

This document describes the REST API endpoints for managing SNMP simulators, virtual agents, and Prometheus metrics.

## Quick Start

### Start the API server

```bash
go run ./cmd/snmpsim-api/main.go --api-addr=127.0.0.1:8080 --metrics-addr=127.0.0.1:9090
```

### Health Check

```bash
curl -s http://127.0.0.1:8080/health | jq
```

## Resource Management

### Labs

A **Lab** is a container for a running simulator instance with configuration and lifecycle management.

#### Create a Lab

```bash
curl -X POST http://127.0.0.1:8080/labs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "lab-prod",
    "engine_id": "engine-1"
  }' | jq
```

Response:
```json
{
  "id": "lab-0",
  "name": "lab-prod",
  "engine_id": "engine-1",
  "status": "stopped",
  "created_at": "2024-01-15T10:30:00Z"
}
```

#### List Labs

```bash
curl -s http://127.0.0.1:8080/labs | jq
```

#### Get Lab Details

```bash
curl -s http://127.0.0.1:8080/labs/lab-0 | jq
```

#### Start a Lab

```bash
curl -X POST http://127.0.0.1:8080/labs/lab-0/start | jq
```

Response:
```json
{
  "id": "lab-0",
  "name": "lab-prod",
  "engine_id": "engine-1",
  "status": "running",
  "created_at": "2024-01-15T10:30:00Z"
}
```

#### Stop a Lab

```bash
curl -X POST http://127.0.0.1:8080/labs/lab-0/stop | jq
```

#### Delete a Lab

```bash
curl -X DELETE http://127.0.0.1:8080/labs/lab-0
```

---

### Engines

An **Engine** defines the SNMP simulator configuration (listen addresses, port ranges, device count, etc.).

#### Create an Engine

```bash
curl -X POST http://127.0.0.1:8080/engines \
  -H "Content-Type: application/json" \
  -d '{
    "name": "engine-1",
    "engine_id": "800007e5",
    "listen_addr": "127.0.0.1",
    "listen_addr6": "::1",
    "port_start": 10000,
    "port_end": 10100,
    "num_devices": 50
  }' | jq
```

Response:
```json
{
  "id": "engine-0",
  "name": "engine-1",
  "engine_id": "800007e5",
  "listen_addr": "127.0.0.1",
  "listen_addr6": "::1",
  "port_start": 10000,
  "port_end": 10100,
  "num_devices": 50,
  "created_at": "2024-01-15T10:30:00Z"
}
```

#### List Engines

```bash
curl -s http://127.0.0.1:8080/engines | jq
```

#### Get Engine Details

```bash
curl -s http://127.0.0.1:8080/engines/engine-0 | jq
```

#### Delete an Engine

```bash
curl -X DELETE http://127.0.0.1:8080/engines/engine-0
```

Note: Cannot delete an engine that is in use by a running lab.

---

### Endpoints (Network Addresses)

An **Endpoint** represents a target address and port combination for SNMP polling.

#### Create an Endpoint

```bash
curl -X POST http://127.0.0.1:8080/endpoints \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prod-agent-1",
    "address": "127.0.0.1",
    "port": 10000
  }' | jq
```

Response:
```json
{
  "id": "endpoint-0",
  "name": "prod-agent-1",
  "address": "127.0.0.1",
  "port": 10000,
  "created_at": "2024-01-15T10:30:00Z"
}
```

#### List Endpoints

```bash
curl -s http://127.0.0.1:8080/endpoints | jq
```

#### Get Endpoint Details

```bash
curl -s http://127.0.0.1:8080/endpoints/endpoint-0 | jq
```

#### Delete an Endpoint

```bash
curl -X DELETE http://127.0.0.1:8080/endpoints/endpoint-0
```

---

### Users

A **User** represents an SNMP monitoring user with authentication credentials (for future authentication).

#### Create a User

```bash
curl -X POST http://127.0.0.1:8080/users \
  -H "Content-Type: application/json" \
  -d '{
    "name": "john-doe",
    "email": "john@example.com"
  }' | jq
```

Response:
```json
{
  "id": "user-0",
  "name": "john-doe",
  "email": "john@example.com",
  "created_at": "2024-01-15T10:30:00Z"
}
```

#### List Users

```bash
curl -s http://127.0.0.1:8080/users | jq
```

#### Get User Details

```bash
curl -s http://127.0.0.1:8080/users/user-0 | jq
```

#### Delete a User

```bash
curl -X DELETE http://127.0.0.1:8080/users/user-0
```

---

### Datasets

A **Dataset** represents a reference to SNMP record files (`.snmprec`) that define OID values and responses.

#### Create a Dataset

```bash
curl -X POST http://127.0.0.1:8080/datasets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prod-switches",
    "engine_id": "engine-1",
    "file_path": "/data/prod-switches.snmprec"
  }' | jq
```

Response:
```json
{
  "id": "dataset-0",
  "name": "prod-switches",
  "engine_id": "engine-1",
  "file_path": "/data/prod-switches.snmprec",
  "created_at": "2024-01-15T10:30:00Z"
}
```

#### List Datasets

```bash
curl -s http://127.0.0.1:8080/datasets | jq
```

#### Get Dataset Details

```bash
curl -s http://127.0.0.1:8080/datasets/dataset-0 | jq
```

#### Delete a Dataset

```bash
curl -X DELETE http://127.0.0.1:8080/datasets/dataset-0
```

---

## Prometheus Metrics

The API exposes Prometheus metrics at the `/metrics` endpoint on the metrics port (default: `:9090`).

### Available Metrics

- **`snmpsim_labs_total{status}`** - Total number of labs created
- **`snmpsim_labs_active`** - Number of currently active (running) labs
- **`snmpsim_packets_total{method,lab_id}`** - Total SNMP packets processed
- **`snmpsim_failures_total{reason,lab_id}`** - Total SNMP operation failures
- **`snmpsim_latency_seconds{method,lab_id}`** - SNMP operation latency histogram
- **`snmpsim_agents_active{lab_id}`** - Number of active virtual agents per lab

### Scraping Metrics with cURL

```bash
curl -s http://127.0.0.1:9090/metrics | grep snmpsim_
```

Example output:
```
# HELP snmpsim_labs_active Number of active (running) labs
# TYPE snmpsim_labs_active gauge
snmpsim_labs_active 1

# HELP snmpsim_packets_total Total SNMP packets processed
# TYPE snmpsim_packets_total counter
snmpsim_packets_total{lab_id="lab-0",method="GET"} 1520
snmpsim_packets_total{lab_id="lab-0",method="WALK"} 450
```

---

## Workflow Example

### 1. Create an Engine

```bash
ENGINE=$(curl -s -X POST http://127.0.0.1:8080/engines \
  -H "Content-Type: application/json" \
  -d '{
    "name": "sim-1",
    "engine_id": "800007e5",
    "listen_addr": "127.0.0.1",
    "port_start": 10000,
    "port_end": 10010,
    "num_devices": 5
  }' | jq -r '.id')

echo "Created engine: $ENGINE"
```

### 2. Create a Lab for the Engine

```bash
LAB=$(curl -s -X POST http://127.0.0.1:8080/labs \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"test-lab\",
    \"engine_id\": \"$ENGINE\"
  }" | jq -r '.id')

echo "Created lab: $LAB"
```

### 3. Start the Lab

```bash
curl -s -X POST http://127.0.0.1:8080/labs/$LAB/start | jq
```

### 4. Monitor Metrics

```bash
# Query active agents
curl -s http://127.0.0.1:9090/metrics | grep snmpsim_agents_active

# Query packet throughput
curl -s http://127.0.0.1:9090/metrics | grep snmpsim_packets_total
```

### 5. Stop the Lab

```bash
curl -s -X POST http://127.0.0.1:8080/labs/$LAB/stop | jq
```

### 6. Clean Up

```bash
curl -s -X DELETE http://127.0.0.1:8080/labs/$LAB
curl -s -X DELETE http://127.0.0.1:8080/engines/$ENGINE
```

---

## Error Handling

All endpoints return appropriate HTTP status codes:

- **200 OK** - Successful GET request
- **201 Created** - Resource created successfully
- **204 No Content** - Successful DELETE request
- **400 Bad Request** - Invalid request body or parameters
- **404 Not Found** - Resource does not exist
- **405 Method Not Allowed** - HTTP method not supported for endpoint
- **409 Conflict** - Operation conflict (e.g., deleting a running lab)
- **500 Internal Server Error** - Server error (simulator start failure, etc.)

Example error response:

```bash
curl -X DELETE http://127.0.0.1:8080/labs/nonexistent
# Response: HTTP 404 Not Found
# Body: "not found"
```

---

## Docker Compose Deployment

See [DOCKER_DEPLOYMENT.md](../DOCKER_DEPLOYMENT.md) for full Docker setup instructions with Prometheus and Grafana.
