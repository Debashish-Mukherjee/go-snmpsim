#!/bin/bash
# Zabbix + SNMPSIM Integration - Quick Command Reference
# Copy and paste commands below to get started immediately

################################################################################
# SETUP COMMANDS (Run these first)
################################################################################

# 1. Generate SNMPREC data files (20 devices × 1,750 OIDs = 35,000 metrics)
python3 examples/data/generate_cisco_iosxr.py examples/data/

# 2. Start Zabbix stack (PostgreSQL + Server + UI)
docker-compose -f zabbix/docker-compose.zabbix.yml up -d

# 3. Start SNMP Simulator (20 devices on ports 20000-20019)
SNMPSIM_DEVICE_COUNT=20 docker-compose up -d

# 4. Wait for services to be healthy (check status)
docker-compose -f zabbix/docker-compose.zabbix.yml ps
docker-compose ps

################################################################################
# DEVICE MANAGEMENT COMMANDS
################################################################################

# Add 20 devices to Zabbix
cd zabbix
python3 manage_devices.py add 20

# List all monitored devices
python3 manage_devices.py list

# Show server status
python3 manage_devices.py status

# Set polling interval to 5 minutes
python3 manage_devices.py interval 5m

# Delete first 5 devices (if needed)
python3 manage_devices.py delete 5

# Go back to root
cd ..

################################################################################
# TESTING COMMANDS
################################################################################

# Run full integration test (comprehensive 7-phase test)
cd tests
python3 run_zabbix_test.py

# Run test with specific device count (e.g., 10 devices)
python3 run_zabbix_test.py --devices 10

# Run test with custom configuration
python3 run_zabbix_test.py --config custom_config.yaml

# Go back to root
cd ..

################################################################################
# VERIFICATION COMMANDS
################################################################################

# Check if SNMPREC files were generated
ls -lah examples/data/cisco-iosxr-*.snmprec | head -3
echo "..."
ls -lah examples/data/cisco-iosxr-*.snmprec | tail -3

# Count total SNMPREC files
ls -1 examples/data/cisco-iosxr-*.snmprec | wc -l

# Check OID count in a file
grep "^1\." examples/data/cisco-iosxr-001.snmprec | wc -l

# Check if Zabbix services are healthy
docker-compose -f zabbix/docker-compose.zabbix.yml ps

# Check if SNMPSIM is running
docker-compose ps snmpsim

# Test SNMP port accessibility
nc -zv -w 3 127.0.0.1 20000
nc -zv -w 3 127.0.0.1 20019

# Test SNMP manually with snmpget
snmpget -v2c -c public 127.0.0.1:20000 1.3.6.1.2.1.1.1.0

################################################################################
# ZABBIX UI ACCESS
################################################################################

# Access Zabbix Web Interface
# URL: http://localhost:8081
# Default Username: Admin
# Default Password: zabbix

# Zabbix Server API
# URL: http://localhost:10051/api_jsonrpc.php

# PostgreSQL Database (for advanced users)
# Host: localhost
# Port: 5432
# Database: zabbix
# Username: zabbix
# Password: zabbix_password_123!

################################################################################
# TROUBLESHOOTING COMMANDS
################################################################################

# Check Docker logs
docker-compose logs snmpsim
docker-compose -f zabbix/docker-compose.zabbix.yml logs zabbix-server
docker-compose -f zabbix/docker-compose.zabbix.yml logs zabbix-postgres

# View Zabbix server logs (live)
docker logs -f zabbix-server

# Check container resource usage
docker stats zabbix-server zabbix-postgres snmpsim

# Reset SNMPSIM (stop and remove)
docker-compose down -v snmpsim

# Reset Zabbix (WARNING: deletes all data!)
docker-compose -f zabbix/docker-compose.zabbix.yml down -v

# Restart all services
docker-compose restart
docker-compose -f zabbix/docker-compose.zabbix.yml restart

# Check port connectivity
timeout 2 bash -c 'cat < /dev/null > /dev/tcp/127.0.0.1/20000' && echo "Port 20000 open" || echo "Port 20000 closed"
timeout 2 bash -c 'cat < /dev/null > /dev/tcp/127.0.0.1/5432' && echo "Port 5432 open" || echo "Port 5432 closed"
timeout 2 bash -c 'cat < /dev/null > /dev/tcp/127.0.0.1/10051' && echo "Port 10051 open" || echo "Port 10051 closed"

################################################################################
# DATABASE COMMANDS (Direct PostgreSQL Access)
################################################################################

# Connect to PostgreSQL
docker exec -it zabbix-postgres psql -U zabbix -d zabbix

# Common queries (run inside psql):
# List all hosts:
# SELECT hostid, host, name FROM hosts ORDER BY host;

# Count items per host:
# SELECT host, COUNT(*) FROM hosts h JOIN items i ON h.hostid = i.hostid GROUP BY host;

# List all items:
# SELECT hostid, name, key_, value_type FROM items LIMIT 10;

# Check database size:
# SELECT pg_size_pretty(pg_database_size('zabbix'));

# Exit psql:
# \q

################################################################################
# MONITORING COMMANDS (In Zabbix UI)
################################################################################

# Via Web UI (http://localhost:8081):
# 1. Go to Configuration > Hosts
# 2. Click on a Cisco IOS XR device (e.g., cisco-iosxr-001)
# 3. Click on Items tab to see all 1,700+ items
# 4. Click on Latest data to view current metric values
# 5. Check Graphs for historical data visualization

# Via API (using manage_devices.py):
cd zabbix
python3 -c "
from zabbix_api_client import ZabbixAPIClient
client = ZabbixAPIClient('http://localhost:8081', 'Admin', 'zabbix')
client.login()
hosts = client.get_all_hosts()
print(f'Total hosts: {len(hosts)}')
for host in hosts[:3]:
    items = client.get_host_items(host['hostid'])
    print(f'  {host[\"host\"]}: {len(items)} items')
"

################################################################################
# SCALING COMMANDS
################################################################################

# Add more devices (50 total)
cd zabbix
python3 manage_devices.py add 50
python3 manage_devices.py interval 10m  # Longer interval for more devices

# Monitor Zabbix performance
docker stats zabbix-server zabbix-postgres

# Check database size
docker exec zabbix-postgres du -sh /var/lib/postgresql/data

# Scale up Zabbix pollers (edit docker-compose.zabbix.yml and change ZBX_STARTPOLLERS)
# Then restart:
docker-compose -f zabbix/docker-compose.zabbix.yml restart zabbix-server

################################################################################
# CLEANUP COMMANDS (Use with caution!)
################################################################################

# Stop all services (keeps data)
docker-compose stop
docker-compose -f zabbix/docker-compose.zabbix.yml stop

# Remove all containers (keeps data in volumes)
docker-compose down
docker-compose -f zabbix/docker-compose.zabbix.yml down

# Remove everything including data (WARNING: DESTRUCTIVE!)
docker-compose down -v
docker-compose -f zabbix/docker-compose.zabbix.yml down -v

# Remove generated SNMPREC files
rm -f examples/data/cisco-iosxr-*.snmprec

# Full reset (WARNING: DELETES ALL DATA!)
docker-compose down -v
docker-compose -f zabbix/docker-compose.zabbix.yml down -v
rm -f examples/data/cisco-iosxr-*.snmprec
rm -f tests/zabbix_test_report.json

################################################################################
# PERFORMANCE TUNING COMMANDS
################################################################################

# Monitor CPU and memory during polling
watch -n 1 'docker stats zabbix-server zabbix-postgres --no-stream'

# Monitor Zabbix server statistics (in psql):
# SELECT * FROM history LIMIT 10;
# SELECT MAX(clock) FROM history;  -- Last update time
# SELECT COUNT(*) FROM history;     -- Total history records

# Check item collection status
cd zabbix
python3 manage_devices.py status

# View test report
cat ../tests/zabbix_test_report.json | python3 -m json.tool

################################################################################
# QUICK SETUP (Copy & Paste All At Once)
################################################################################

# This one command does everything:
python3 examples/data/generate_cisco_iosxr.py examples/data/ && \
docker-compose -f zabbix/docker-compose.zabbix.yml up -d && \
sleep 10 && \
SNMPSIM_DEVICE_COUNT=20 docker-compose up -d && \
sleep 10 && \
cd zabbix && \
pip3 install -q requests pyyaml && \
python3 manage_devices.py add 20 && \
cd .. && \
echo "✅ Setup complete! Visit http://localhost:8081 (Admin/zabbix)"

################################################################################
# NOTES
################################################################################

# 1. All SNMPREC files are ready in examples/data/ (20 files)
#    Each file contains ~1,702 OIDs for realistic monitoring
#
# 2. Zabbix services start automatically with docker-compose
#    PostgreSQL, Server, and Web UI startup takes ~30-60 seconds
#
# 3. Default credentials:
#    Zabbix Web UI: Admin / zabbix
#    PostgreSQL: zabbix / zabbix_password_123!
#
# 4. SNMP Configuration:
#    Port: 20000-20019 (20 devices)
#    Community: public
#    Version: SNMPv2c
#
# 5. Polling Configuration:
#    Interval: 5 minutes (configurable)
#    Concurrency: 16 pollers
#    Timeout: 5 seconds per device
#
# 6. Expected Results:
#    • 20 devices with 1,700+ items each
#    • 35,000+ total metrics per polling cycle
#    • 95-99% collection success rate
#    • 5-10 seconds per polling cycle
#
# 7. For more information, see:
#    - INTEGRATION_GUIDE.md
#    - IMPLEMENTATION_SUMMARY.md
#    - COMPLETION_SUMMARY.txt

################################################################################
