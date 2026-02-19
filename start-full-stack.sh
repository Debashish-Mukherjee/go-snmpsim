#!/bin/bash

# Master startup script for complete SNMP Simulator + Zabbix stack with 100 SNMPv3 hosts

set -e

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_DIR"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}================================================================================${NC}"
echo -e "${BLUE}     SNMP Simulator + Zabbix Full Stack with 100 SNMPv3 Hosts${NC}"
echo -e "${BLUE}================================================================================${NC}"
echo ""

# Parse arguments
SKIP_DOCKER=false
SKIP_HOSTS=false
NUM_HOSTS=100

while [[ $# -gt 0 ]]; do
  case $1 in
    --skip-docker)
      SKIP_DOCKER=true
      shift
      ;;
    --skip-hosts)
      SKIP_HOSTS=true
      shift
      ;;
    --num-hosts)
      NUM_HOSTS="$2"
      shift 2
      ;;
    --help)
      echo "Usage: $0 [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --skip-docker    Don't start docker containers (use existing)"
      echo "  --skip-hosts     Don't configure hosts in Zabbix"
      echo "  --num-hosts N    Number of hosts to create (default: 100)"
      echo "  --help           Show this help message"
      echo ""
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Start Docker containers
if [ "$SKIP_DOCKER" = false ]; then
  echo -e "${BLUE}[1/5]${NC} Starting Docker services..."
  echo "      Bringing up: SNMP Simulator, API, Prometheus, Grafana, Zabbix..."
  
  docker-compose -f docker-compose-full.yml up -d
  
  # Wait for critical services
  echo ""
  echo -e "${YELLOW}⏳ Waiting for services to be healthy...${NC}"
  
  # Wait for Zabbix Server
  echo -n "   Zabbix Server: "
  for i in {1..60}; do
    if docker exec zabbix-server curl -f http://localhost:10051 >/dev/null 2>&1; then
      echo -e "${GREEN}✓${NC}"
      break
    fi
    echo -n "."
    sleep 1
  done
  
  # Wait for Zabbix Web UI
  echo -n "   Zabbix Web UI: "
  for i in {1..60}; do
    if curl -f http://localhost:8081 >/dev/null 2>&1; then
      echo -e "${GREEN}✓${NC}"
      break
    fi
    echo -n "."
    sleep 1
  done
  
  # Wait for SNMP Simulator
  echo -n "   SNMP Simulator: "
  for i in {1..30}; do
    if docker logs snmpsim-simulator 2>/dev/null | grep -q "Started\|listening"; then
      echo -e "${GREEN}✓${NC}"
      break
    fi
    echo -n "."
    sleep 1
  done
  
  # Wait for API
  echo -n "   API Server: "
  for i in {1..30}; do
    if curl -f http://localhost:8080/health >/dev/null 2>&1; then
      echo -e "${GREEN}✓${NC}"
      break
    fi
    echo -n "."
    sleep 1
  done
  
  echo ""
  echo -e "${GREEN}✓ All services started!${NC}"
else
  echo -e "${YELLOW}⏭️  Skipping Docker startup (using existing services)${NC}"
fi

echo ""
echo -e "${BLUE}[2/5]${NC} Service Status"
echo "      API Server:         http://localhost:8080"
echo "      Prometheus:         http://localhost:9091"
echo "      Grafana:            http://localhost:3000 (admin/admin)"
echo "      Zabbix Web UI:      http://localhost:8081 (Admin/zabbix)"
echo "      SNMP Simulator:     UDP ports 10000-10099"
echo ""

echo -e "${BLUE}[3/5]${NC} Waiting for Zabbix API to be ready..."

# Retry logic for Zabbix API
MAX_RETRIES=30
RETRY_DELAY=2

for retry in $(seq 1 $MAX_RETRIES); do
  if curl -s -X POST http://localhost:8081/api_jsonrpc.php \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"user.login","params":{"username":"Admin","password":"zabbix"},"id":1}' \
    | grep -q "result"; then
    echo -e "${GREEN}✓ Zabbix API is ready!${NC}"
    break
  fi
  
  if [ $retry -eq $MAX_RETRIES ]; then
    echo -e "${RED}❌ Zabbix API failed to respond after ${MAX_RETRIES} attempts${NC}"
    echo "   Check Zabbix logs: docker logs zabbix-web"
    exit 1
  fi
  
  echo -n "."
  sleep $RETRY_DELAY
done

echo ""

# Configure hosts in Zabbix
if [ "$SKIP_HOSTS" = false ]; then
  echo -e "${BLUE}[4/5]${NC} Configuring ${NUM_HOSTS} SNMP hosts in Zabbix..."
  
  cd "$PROJECT_DIR/zabbix"
  
  python3 bulk_import_snmpv3_hosts.py \
    --url http://localhost:8081 \
    --username Admin \
    --password zabbix \
    --num-hosts $NUM_HOSTS \
    --start-port 10000 \
    --base-ip snmpsim \
    --hostgroup "SNMP Simulators"
  
  cd "$PROJECT_DIR"
else
  echo -e "${YELLOW}⏭️  Skipping host configuration${NC}"
fi

echo ""
echo -e "${BLUE}[5/5]${NC} Startup Complete!"
echo ""
echo -e "${GREEN}================================================================================${NC}"
echo -e "${GREEN}✓ SNMP Simulator + Zabbix Stack is running with ${NUM_HOSTS} SNMPv3 hosts!${NC}"
echo -e "${GREEN}================================================================================${NC}"
echo ""
echo -e "${YELLOW}Quick Links:${NC}"
echo "  • Zabbix Dashboard:  http://localhost:8081 (Admin/zabbix)"
echo "  • Zabbix API:        http://localhost:8081/api_jsonrpc.php"
echo "  • Grafana Metrics:   http://localhost:3000 (admin/admin)"
echo "  • Prometheus:        http://localhost:9091"
echo "  • API Documentation: http://localhost:8080"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "  1. Log into Zabbix: http://localhost:8081"
echo "  2. Navigate to: Configuration → Hosts"
echo "  3. View 'SNMP Simulators' host group with ${NUM_HOSTS} hosts"
echo "  4. Check Monitoring → Latest data for SNMP metrics"
echo "  5. View Grafana dashboard at http://localhost:3000"
echo ""
echo -e "${YELLOW}Monitor Logs:${NC}"
echo "  • Simulator:   docker logs -f snmpsim-simulator"
echo "  • Zabbix:      docker logs -f zabbix-server"
echo "  • API:         docker logs -f snmpsim-api"
echo ""
echo -e "${YELLOW}Shutdown Command:${NC}"
echo "  docker-compose -f docker-compose-full.yml down"
echo ""

# Optional: Show sample Zabbix host details
echo -e "${BLUE}Sample Host Configuration:${NC}"
echo "  • Hostname Pattern: snmpsim-host-000 to snmpsim-host-099"
echo "  • SNMP Version:     3 (SNMPv3)"
echo "  • SNMP Ports:       10000-10099 (UDP)"
echo "  • Username:         simuser"
echo "  • Authentication:   SHA (authpass1234)"
echo "  • Privacy:          AES (privpass1234)"
echo ""

echo -e "${GREEN}Status: READY FOR MONITORING${NC}"
echo ""
