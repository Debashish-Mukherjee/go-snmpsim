# Grafana Metrics Dashboard - Setup Complete ✓

## Issue Resolution Summary

**Problem:** Grafana dashboard was not showing any metrics data from the SNMP Simulator API.

**Root Causes Identified & Fixed:**

1. **Metrics Handler Endpoint Issue**
   - The metrics server was running on a separate port (9090) but wasn't properly integrated with the main API
   - **Fix:** Added `/metrics` endpoint to the main API mux and explicitly used `prometheus.DefaultGatherer`

2. **API Server Binding Issue**  
   - API was listening on `127.0.0.1:9090` (localhost only) instead of `0.0.0.0:9090`
   - Docker containers couldn't reach the metrics endpoint from Prometheus
   - **Fix:** Updated docker-compose command to use correct `--metrics-addr=0.0.0.0:9090`

3. **Metrics Recording**
   - Metrics were defined and registered but not being recorded during operations
   - **Fix:** Added `RecordLabStart()` call in the `CreateLab()` handler to trigger metric updates

## Verification

✅ **Metrics are now flowing correctly:**
- API exposes metrics on `http://localhost:8080/metrics`
- Prometheus scrapes metrics every 15 seconds (target status: UP)
- Prometheus stores timeline data for all `snmpsim_*` metrics

```bash
# Verify metrics endpoint
curl http://localhost:8080/metrics | grep snmpsim_labs_total

# Sample output:
# HELP snmpsim_labs_total Total number of labs created
# TYPE snmpsim_labs_total counter
snmpsim_labs_total{status="started"} 5
```

✅ **Prometheus confirmed working:**
```bash
curl 'http://localhost:9091/api/v1/query?query=snmpsim_labs_total'
# Returns: {"status":"success","data":{"resultType":"vector","result":[...]}}
```

## Metrics Available in Grafana

The dashboard now has access to:

| Metric | Type | Description |
|--------|------|-------------|
| `snmpsim_labs_total` | Counter | Total labs created (by status) |
| `snmpsim_labs_active` | Gauge | Currently active/running labs |
| `snmpsim_packets_total` | Counter | Total SNMP packets processed |
| `snmpsim_failures_total` | Counter | Total operation failures |
| `snmpsim_latency_seconds` | Histogram | SNMP operation latency |
| `snmpsim_agents_active` | Gauge | Active virtual agents per lab |

## Accessing the Dashboard

1. **Open Grafana:**  
   http://localhost:3000

2. **Login with:**
   - Username: `admin`
   - Password: `admin`

3. **View Metrics:**
   - Navigate to: Dashboards → SNMP Simulator Metrics
   - Or use Explore to query metrics directly

## Generate Sample Data

Create test labs to populate the dashboard:

```bash
./setup-grafana.sh
```

This will:
- Create a test lab
- Start it (triggering RecordLabStart metric)
- Generate 20 API requests
- Wait for Prometheus to scrape the data

## Files Modified

1. **docker-compose-full.yml** - Fixed API server command format
2. **cmd/snmpsim-api/main.go** - Added metrics handler to main mux
3. **cmd/snmpsim-api/metrics.go** - Improved registration error handling

## Next Steps

- ✅ Metrics Infrastructure - Complete
- ✅ Prometheus Integration - Complete  
- ✅ Grafana Dashboard Access - Complete
- ⏭️ Create actual monitoring workload with 100 SNMPv3 hosts
- ⏭️ Configure Zabbix to poll hosts and inject traffic
