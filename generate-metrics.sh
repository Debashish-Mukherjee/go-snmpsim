#!/bin/bash

# Script to generate test metrics data for Grafana

API="http://localhost:8080"
HEADERS="Content-Type: application/json"

echo "Generating test metrics data..."

# Create a test lab
echo "Creating test lab..."
LAB_ID=$(curl -s -X POST "$API/labs" \
  -H "$HEADERS" \
  -d '{
    "name": "Test Lab 1",
    "description": "Test lab for metrics"
  }' | grep -o '"id":"[^"]*' | cut -d'"' -f4)

echo "Lab created: $LAB_ID"

if [ -z "$LAB_ID" ]; then
  echo "Failed to create lab"
  exit 1
fi

# Create an engine for the lab
echo "Creating engine..."
ENGINE_ID=$(curl -s -X POST "$API/engines" \
  -H "$HEADERS" \
  -d "{
    \"name\": \"Engine 1\",
    \"lab_id\": \"$LAB_ID\",
    \"agent_count\": 10
  }" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

echo "Engine created: $ENGINE_ID"

# Create an endpoint for the engine
echo "Creating endpoint..."
ENDPOINT_ID=$(curl -s -X POST "$API/endpoints" \
  -H "$HEADERS" \
  -d "{
    \"engine_id\": \"$ENGINE_ID\",
    \"name\": \"Test Endpoint\",
    \"address\": \"192.168.1.1\",
    \"port\": 161,
    \"protocol\": \"snmp\"
  }" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

echo "Endpoint created: $ENDPOINT_ID"

# Now start the lab (this will trigger RecordLabStart)
echo "Starting lab..."
curl -s -X POST "$API/labs/$LAB_ID/start" \
  -H "$HEADERS" \
  -d "{
    \"engine_id\": \"$ENGINE_ID\"
  }" > /dev/null

sleep 1

# Generate some traffic
echo "Generating traffic..."
for i in {1..10}; do
  curl -s -X GET "$API/labs/$LAB_ID" > /dev/null
  curl -s -X GET "$API/engines" > /dev/null
  echo "Request $i completed"
  sleep 0.3
done

# Check metrics
echo ""
echo "Checking metrics availability..."
curl -s http://localhost:9090/metrics 2>&1 | grep "^snmpsim" | head -5

echo ""
echo "Metrics data generated!"
echo "Access Grafana at: http://localhost:3000"
echo "Credentials: admin / admin"
