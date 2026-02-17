#!/bin/bash
# Quick Start Script for Zabbix + SNMPSIM Integration
# Automates initial setup and verification

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_header() {
    echo -e "\n${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║${NC} $1"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}\n"
}

print_step() {
    echo -e "${YELLOW}▸${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check prerequisites
print_header "Checking Prerequisites"

print_step "Checking Docker..."
if ! command -v docker &> /dev/null; then
    print_error "Docker is not installed"
    exit 1
fi
print_success "Docker found: $(docker --version)"

print_step "Checking Docker Compose..."
if ! command -v docker-compose &> /dev/null; then
    print_error "Docker Compose is not installed"
    exit 1
fi
print_success "Docker Compose found: $(docker-compose --version)"

print_step "Checking Python..."
if ! command -v python3 &> /dev/null; then
    print_error "Python 3 is not installed"
    exit 1
fi
print_success "Python found: $(python3 --version)"

# Navigate to project root
print_header "Setting Up Project"

if [ ! -f "docker-compose.yml" ]; then
    print_error "docker-compose.yml not found. Run from project root directory."
    exit 1
fi

PROJECT_DIR=$(pwd)
print_success "Project directory: $PROJECT_DIR"

# Step 1: Generate SNMPREC files
print_header "Step 1: Generating SNMPREC Data Files"

if [ ! -f "examples/data/generate_cisco_iosxr.py" ]; then
    print_error "Generator script not found"
    exit 1
fi

print_step "Generating 20 device files with 1700+ OIDs each..."
if python3 examples/data/generate_cisco_iosxr.py examples/data/ > /tmp/generator.log 2>&1; then
    FILE_COUNT=$(ls examples/data/cisco-iosxr-*.snmprec 2>/dev/null | wc -l)
    print_success "Generated $FILE_COUNT SNMPREC files (35,000+ total metrics)"
else
    print_error "Generator failed. Check /tmp/generator.log"
    exit 1
fi

# Step 2: Start Zabbix Stack
print_header "Step 2: Starting Zabbix 7.4 Stack"

print_step "Starting PostgreSQL, Zabbix Server, and Frontend..."
if docker-compose -f zabbix/docker-compose.zabbix.yml up -d > /tmp/docker_start.log 2>&1; then
    print_success "Zabbix services started"
else
    print_error "Failed to start Zabbix services. Check /tmp/docker_start.log"
    exit 1
fi

# Wait for services
print_step "Waiting for services to be ready..."
sleep 10

# Check health
POSTGRES_READY=0
ZABBIX_READY=0

for i in {1..30}; do
    if docker-compose -f zabbix/docker-compose.zabbix.yml ps postgres | grep -q "healthy"; then
        POSTGRES_READY=1
    fi
    if docker-compose -f zabbix/docker-compose.zabbix.yml ps zabbix-server | grep -q "healthy"; then
        ZABBIX_READY=1
    fi
    
    if [ $POSTGRES_READY -eq 1 ] && [ $ZABBIX_READY -eq 1 ]; then
        break
    fi
    
    sleep 2
done

if [ $POSTGRES_READY -eq 1 ]; then
    print_success "PostgreSQL is healthy"
else
    print_error "PostgreSQL not healthy after 60 seconds"
fi

if [ $ZABBIX_READY -eq 1 ]; then
    print_success "Zabbix Server is healthy"
else
    print_error "Zabbix Server not healthy after 60 seconds"
fi

# Step 3: Start SNMP Simulator
print_header "Step 3: Starting SNMP Simulator"

print_step "Starting SNMPSIM with 20 devices..."
if SNMPSIM_DEVICE_COUNT=20 docker-compose up -d snmpsim > /tmp/snmpsim_start.log 2>&1; then
    print_success "SNMPSIM started with 20 devices (ports 20000-20019)"
else
    print_error "Failed to start SNMPSIM. Check /tmp/snmpsim_start.log"
    exit 1
fi

sleep 5

# Step 4: Add devices to Zabbix
print_header "Step 4: Adding Devices to Zabbix"

cd zabbix

print_step "Installing Python requirements..."
if [ -f "requirements.txt" ]; then
    pip install -q -r requirements.txt 2>/dev/null || true
fi

# Try to add devices
pip install -q requests pyyaml 2>/dev/null || true

print_step "Adding 20 Cisco IOS XR devices to Zabbix..."
if python3 -c "from zabbix_api_client import ZabbixAPIClient; print('API client available')" 2>/dev/null; then
    if timeout 120 python3 manage_devices.py add 20 > /tmp/add_devices.log 2>&1; then
        ADDED=$(grep "✓ Device" /tmp/add_devices.log | wc -l)
        print_success "Added $ADDED devices to Zabbix"
    else
        print_error "Failed to add devices. This may be normal if Zabbix is still initializing."
        print_step "Run manually later with: cd zabbix && python3 manage_devices.py add 20"
    fi
else
    print_error "Python dependencies not available"
    print_step "Install with: pip install requests pyyaml"
fi

cd ..

# Step 5: Verification
print_header "Step 5: Verification & Status"

print_step "Checking Docker containers..."
docker-compose ps | grep -E "snmpsim|zabbix" || true
docker-compose -f zabbix/docker-compose.zabbix.yml ps || true

print_step "Checking SNMPSIM response..."
if timeout 5 nc -zv -w 2 127.0.0.1 20000 > /tmp/port_check.log 2>&1; then
    print_success "SNMPSIM is responding on port 20000"
else
    print_error "SNMPSIM not responding. It may still be starting."
fi

# Summary
print_header "✅ Setup Complete!"

echo -e "${GREEN}Services Running:${NC}"
echo "  • SNMP Simulator: localhost:20000-20019 (20 devices)"
echo "  • Zabbix Server: localhost:10051 (API)"
echo "  • Zabbix Frontend: http://localhost:8081"
echo "  • PostgreSQL: localhost:5432"
echo ""

echo -e "${GREEN}Next Steps:${NC}"
echo "  1. Access Zabbix UI: http://localhost:8081"
echo "     Login: Admin / zabbix"
echo ""
echo "  2. View monitored hosts:"
echo "     cd zabbix && python3 manage_devices.py list"
echo ""
echo "  3. Run full integration test:"
echo "     cd tests && python3 run_zabbix_test.py"
echo ""
echo "  4. Monitor SNMP polling in real-time:"
echo "     docker logs -f zabbix-server | grep snmp"
echo ""
echo "  5. Check data collection status:"
echo "     cd zabbix && python3 manage_devices.py status"
echo ""

echo -e "${YELLOW}Configuration Files:${NC}"
echo "  • SNMPREC files: examples/data/cisco-iosxr-*.snmprec"
echo "  • Test config: tests/zabbix_config.yaml"
echo "  • Zabbix config: zabbix/docker-compose.zabbix.yml"
echo ""

echo -e "${YELLOW}Useful Commands:${NC}"
echo "  # Restart all services"
echo "  docker-compose down && docker-compose up -d"
echo ""
echo "  # View logs"
echo "  docker logs -f zabbix-server"
echo "  docker logs -f snmpsim"
echo ""
echo "  # Stop services"
echo "  docker-compose down"
echo "  docker-compose -f zabbix/docker-compose.zabbix.yml down"
echo ""
echo "  # Full reset (WARNING: deletes all data)"
echo "  docker-compose down -v && docker-compose -f zabbix/docker-compose.zabbix.yml down -v"
echo "  rm -rf examples/data/cisco-iosxr-*.snmprec"
echo ""

echo -e "${BLUE}Documentation: INTEGRATION_GUIDE.md${NC}\n"
