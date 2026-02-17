#!/bin/bash
# Deploy SNMP Simulator with Docker Compose

set -e

echo "Building SNMP Simulator Docker image..."
docker build -t go-snmpsim:latest .

echo ""
echo "Starting SNMP Simulator with Docker Compose..."
docker-compose up -d

echo ""
echo "Waiting for simulator to be ready..."
sleep 3

echo ""
echo "Simulator Status:"
docker-compose ps

echo ""
echo "=== Deployment Complete ==="
echo ""
echo "Access the simulator:"
echo "  Container: docker-compose exec snmpsim snmpsim"
echo "  Test port 20000: snmpget -v 2c -c public localhost:20000 1.3.6.1.2.1.1.5.0"
echo ""
echo "View logs:"
echo "  docker-compose logs -f snmpsim"
echo ""
echo "Stop simulator:"
echo "  docker-compose down"
echo ""
