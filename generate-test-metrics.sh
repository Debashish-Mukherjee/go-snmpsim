#!/bin/bash

# Script to generate comprehensive test metrics data

echo "Generating comprehensive metrics for Grafana..."
echo ""

# Create multiple labs
echo "Creating labs..."
for i in {1..10}; do
  curl -s -X POST "http://localhost:8080/labs" \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"Lab $i\", \"description\": \"Test lab\"}" > /dev/null
  echo "  Lab $i created"
done

echo ""
echo "Generating API traffic to record metrics..."
sleep 2

# Make many API requests to generate packet metrics
for i in {1..50}; do
  curl -s "http://localhost:8080/labs" > /dev/null &
  curl -s "http://localhost:8080/labs/lab-0" > /dev/null 2>&1 &
  curl -s "http://localhost:8080/labs/lab-5" > /dev/null 2>&1 &
  
  if [ $((i % 10)) -eq 0 ]; then
    echo "  ...request batch $i/50"
  fi
done

wait  # Wait for all background requests to complete

echo ""
echo "Waiting for Prometheus to scrape metrics (15-20 seconds)..."
sleep 20

echo ""
echo "✓ Metrics data generated!"
echo ""
echo "Refresh Grafana to see:"
echo "  • Total Labs Created"
echo "  • Active Labs" 
echo "  • SNMP Failures"
echo "  • SNMP Operation Latency"
echo ""
