#!/bin/bash

# Complete Grafana metrics setup and verification script

set -e

echo "========================================="
echo "Setting up Metrics and Grafana Dashboard" 
echo "========================================="
echo ""

# 1. Create test lab and start it to trigger metrics
echo "[1/5] Creating test lab and starting it..."
LAB_RESPONSE=$(curl -s -X POST "http://localhost:8080/labs" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Performance Test Lab",
    "description": "Lab for testing metrics collection"
  }')

LAB_ID=$(echo "$LAB_RESPONSE" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
echo "   Lab created: $LAB_ID"

if [ -z "$LAB_ID" ]; then
  echo "   ERROR: Failed to create lab"
  exit 1
fi

# 2. Create an engine
echo "[2/5] Creating engine..."
ENGINE_RESPONSE=$(curl -s -X POST "http://localhost:8080/engines" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Test Engine\",
    \"lab_id\": \"$LAB_ID\",
    \"agent_count\": 50
  }")

ENGINE_ID=$(echo "$ENGINE_RESPONSE" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
echo "   Engine created: $ENGINE_ID"

# 3. Start the lab (triggers RecordLabStart metric)
echo "[3/5] Starting lab (triggers metric recording)..."
curl -s -X POST "http://localhost:8080/labs/$LAB_ID/start" \
  -H "Content-Type: application/json" \
  -d "{\"engine_id\": \"$ENGINE_ID\"}" > /dev/null
echo "   Lab started"

# 4. Generate some activity to trigger additional metrics
echo "[4/5] Generating metrics activity..."
for i in {1..20}; do
  curl -s -X GET "http://localhost:8080/labs/$LAB_ID" > /dev/null 2>&1
  curl -s -X GET "http://localhost:8080/engines" > /dev/null 2>&1
  if [ $((i % 5)) -eq 0 ]; then
    echo "   ...request $i/20"
  fi
  sleep 0.1
done

# 5. Wait for Prometheus to scrape the metrics
echo "[5/5] Waiting for metrics collection..."
sleep 5

# Verify metrics are available
METRICS=$(curl -s "http://localhost:8080/metrics" 2>&1 | grep "snmpsim_labs_total" | wc -l)

if [ "$METRICS" -gt 0 ]; then
  echo ""
  echo "✓ SUCCESS: Metrics are being recorded!"
  echo ""
  echo "───────────────────────────────────────"
  echo "GRAFANA DASHBOARD SETUP COMPLETE"
  echo "───────────────────────────────────────"
  echo ""
  echo "Access Grafana at: http://localhost:3000"
  echo "Credentials:       admin / admin"
  echo ""
  echo "Steps to view metrics:"
  echo "1. Click 'Dashboards' in the left sidebar"
  echo "2. Select 'SNMP Simulator Metrics'"
  echo "3. View live metrics from your test lab"
  echo ""
else
  echo ""
  echo "⚠ WARNING: Metrics may not be fully collected yet"
  echo "Check again in 30 seconds as Prometheus scrapes every 15 seconds"
  echo ""
fi
