# SNMP Simulator - Testing & Validation Guide

## Quick Start Testing

### Prerequisites
```bash
# Install SNMP tools (Debian/Ubuntu)
sudo apt-get install snmp-tools

# Install SNMP tools (CentOS/RHEL)
sudo yum install net-snmp-utils

# Or using Docker
docker run -it --rm --network host alpine sh -c "apk add net-snmp-tools && exec sh"
```

### Basic Connectivity Test

```bash
# Test a single port
snmpget -v 2c -c public -t 1 localhost:20000 1.3.6.1.2.1.1.5.0

# Expected output:
# SNMPv2-MIB::sysName.0 = STRING "Device-0"
```

## Regression Test Suite

Run focused regression tests for the recent correctness and stability fixes:

```bash
GOCACHE=/tmp/go-build go test ./internal/store -run 'TestOIDIndexManager' -count=1
GOCACHE=/tmp/go-build go test ./internal/agent -run 'TestHandlePacketUpdatesPollStatsConcurrently' -count=1
GOCACHE=/tmp/go-build go test ./internal/api -run 'TestHandleSNMPTestWithoutTester|TestHandleWorkloadsWithoutManager|TestHandleStartStopLifecycle|TestHandleSNMPTestStartsAsyncJob|TestHandleTestJobWithoutTester|TestAPIMiddlewareAuth|TestAPIMiddlewareRateLimit' -count=1
GOCACHE=/tmp/go-build go test ./internal/webui -run 'TestWorkloadManager' -count=1
GOCACHE=/tmp/go-build go test ./cmd/snmpsim-api -run 'TestShutdownCancelsAndCleansLabState' -count=1
```

Expected result for each command:

- `ok   github.com/debashish-mukherjee/go-snmpsim/<package>`

### Walk Operation Test

```bash
# Walk system OIDs
snmpwalk -v 2c -c public localhost:20000 1.3.6.1.2.1.1

# Walk interfaces
snmpwalk -v 2c -c public localhost:20000 1.3.6.1.2.1.2
```

### Bulk Walk Test (Efficient)

```bash
# Bulk walk for multiple OIDs
snmpbulkwalk -v 2c -c public localhost:20000 1.3.6.1.2.1.1
```

## Stress Testing

### Test Multiple Devices Sequentially

```bash
#!/bin/bash
# Test 100 random devices
for i in {0..99}; do
  port=$((20000 + $i))
  echo "Testing port $port..."
  snmpget -v 2c -c public -t 1 -r 1 localhost:$port 1.3.6.1.2.1.1.5.0 2>/dev/null
done
```

### Test Multiple Devices in Parallel

```bash
#!/bin/bash
# Parallel testing with controlled concurrency
MAX_JOBS=20
for i in {0..999}; do
  while [ $(jobs -r -p | wc -l) -ge $MAX_JOBS ]; do
    sleep 0.1
  done
  
  port=$((20000 + (i % 1000)))
  (
    snmpget -v 2c -c public -t 2 localhost:$port 1.3.6.1.2.1.1.5.0 2>/dev/null && \
    echo "Port $port: OK" || echo "Port $port: FAIL"
  ) &
done
wait
```

### Load Test with nc (netcat)

```bash
#!/bin/bash
# Fast connectivity test
for i in {0..999}; do
  port=$((20000 + $i))
  nc -zv -w 1 localhost $port 2>&1 | grep -q "succeeded" && echo "Port $port: OK" &
done
wait
```

## Performance Metrics

### Measure Latency

```bash
# Single query latency
time snmpget -v 2c -c public localhost:20000 1.3.6.1.2.1.1.5.0 > /dev/null
```

### Measure Throughput

```bash
#!/bin/bash
# Throughput test - queries per second
start=$(date +%s%N | cut -b1-13)

for i in {1..1000}; do
  snmpget -v 2c -c public localhost:20000 1.3.6.1.2.1.1.5.0 > /dev/null 2>&1 &
done
wait

end=$(date +%s%N | cut -b1-13)
elapsed=$((end - start))
qps=$((1000 * 1000 / elapsed))
echo "Queries per second: $qps"
```

### Memory Usage

```bash
# Monitor simulator memory
docker stats snmpsim

# Or for local process
top -p $(pgrep go-snmpsim)
```

## Debugging

### Validate Packet Size

```bash
# Capture packets on port 20000
sudo tcpdump -i lo 'udp port 20000' -s 0 -w capture.pcap

# Analyze
tcpdump -r capture.pcap -A
```

### Check Port Binding

```bash
# List all bound ports
netstat -tulnp | grep LISTEN

# Or using ss (newer)
ss -tulnp | grep LISTEN

# Check specific port range
ss -tulnp | grep ":2[0-9][0-9][0-9][0-9]"
```

### Monitor File Descriptors

```bash
# Get simulator PID
PID=$(pgrep go-snmpsim)

# Check FD usage
lsof -p $PID | wc -l

# List FD types
lsof -p $PID | tail -n +2 | awk '{print $4}' | sort | uniq -c
```

### Enable Verbose Logging

```bash
# Run simulator with explicit logging
./go-snmpsim -port-start=20000 -port-end=20100 -devices=10 2>&1 | tee simulator.log
```

## Integration Testing

### Test with Monitoring Tools

#### Nagios/Icinga
```bash
# Test with check_snmp
/usr/lib64/nagios/plugins/check_snmp \
  -H localhost:20000 \
  -C public \
  -o 1.3.6.1.2.1.1.5.0
```

#### Zabbix
```bash
# Test with zabbix_get
zabbix_get -s localhost:20000 -k "snmp_device_info"

# Or query directly
zabbix_get -s localhost:20000 -p 20000 -I 127.0.0.1 \
  -O 1.3.6.1.2.1.1.5.0
```

#### Prometheus (via SNMP Exporter)
```bash
# Configure in snmp.yml:
# localhost:
#   auth: public
#   module: if_mib

prometheus_sd_consul --config.file=sd.yml
```

## Troubleshooting

### No Response on Port

```bash
# Check if port is listening
netstat -tulnp | grep 20000

# Try numeric port directly
snmpget -v 2c -c public 127.0.0.1:20000 .1.3.6.1.2.1.1.5.0

# Check for firewall
ufw status
sudo ufw allow 20000:30000/udp
```

### Timeout on Queries

```bash
# Increase timeout
snmpget -v 2c -c public -t 5 -r 3 localhost:20000 1.3.6.1.2.1.1.5.0

# Check simulator logs for errors
docker-compose logs snmpsim | tail -20
```

### High CPU Usage

```bash
# Check threads
ps -eLo pid,tid,cmd | grep go-snmpsim

# Monitor in real-time
watch 'ps aux | grep go-snmpsim'
```

### Memory Saturation

```bash
# Check memory usage
free -h

# Monitor process memory
while true; do
  ps aux | grep go-snmpsim | grep -v grep | awk '{print $6}'
  sleep 1
done
```

## Performance Tuning

### Optimize File Descriptors

```bash
# Edit /etc/security/limits.conf
echo "* soft nofile 65536" | sudo tee -a /etc/security/limits.conf
echo "* hard nofile 65536" | sudo tee -a /etc/security/limits.conf

# Apply (requires login)
ulimit -n 65536
```

### Docker Resource Limits

```yaml
# In docker-compose.yml
deploy:
  resources:
    limits:
      cpus: '8'
      memory: 4G
    reservations:
      cpus: '4'
      memory: 2G
```

### Kernel Tuning

```bash
# Increase UDP buffer sizes
sudo sysctl -w net.core.rmem_max=262144
sudo sysctl -w net.core.wmem_max=262144
sudo sysctl -w net.ipv4.udp_mem="65536 131072 262144"

# Make persistent in /etc/sysctl.conf
echo "net.core.rmem_max=262144" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

## Monitoring Setup

### Docker Health Check

```bash
# Check simulator health
docker exec snmpsim nc -zv localhost 20000

# Or with curl (if added to container)
docker exec snmpsim curl http://localhost:8080/health
```

### Prometheus Metrics (Future Enhancement)

```
# Metrics to expose:
snmp_sim_devices_total
snmp_sim_packets_received_total
snmp_sim_packets_sent_total
snmp_sim_response_time_seconds
snmp_sim_file_descriptors_used
snmp_sim_memory_bytes_used
```

## Expected Output Examples

### Successful GET

```
$ snmpget -v 2c -c public localhost:20000 1.3.6.1.2.1.1.5.0
SNMPv2-MIB::sysName.0 = STRING "Device-0"
```

### Successful GETNEXT

```
$ snmpgetnext -v 2c -c public localhost:20000 1.3.6.1.2.1.1.4
SNMPv2-MIB::sysContact.0 = STRING "admin@example.com"
```

### Successful WALK

```
$ snmpwalk -v 2c -c public localhost:20000 1.3.6.1.2.1.1 | head
SNMPv2-MIB::sysDescr.0 = STRING "Linux device"
SNMPv2-MIB::sysObjectID.0 = OID: enterprises.9.9.46.1
SNMPv2-MIB::sysUpTime.0 = Timeticks: (123456) 0:20:34.56
SNMPv2-MIB::sysContact.0 = STRING "admin@example.com"
SNMPv2-MIB::sysName.0 = STRING "Device-0"
```

## Additional Resources

- [Net-SNMP Documentation](http://www.net-snmp.org/)
- [SNMP RFC 1905-1907](https://tools.ietf.org/html/rfc1905)
- [GoSNMP Library](https://github.com/gosnmp/gosnmp)
  
