# Quick Start Guide - SNMP Simulator

## 5-Minute Local Setup

### Option 1: Run Binary Directly

```bash
# Navigate to project directory
cd /home/debashish/trials/go-snmpsim

# Run with default settings (100 devices, ports 20000-30000)
./go-snmpsim

# Or with custom parameters
./go-snmpsim -port-start=20000 -port-end=21000 -devices=500 -listen=0.0.0.0
```

### Option 2: Run with Docker

```bash
# Start container
docker run -d \
  --name snmpsim \
  -p 20000-30000:20000-30000/udp \
  -e GOMAXPROCS=4 \
  go-snmpsim:latest

# View logs
docker logs -f snmpsim

# Stop
docker stop snmpsim
```

### Option 3: Run with Docker Compose

```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f snmpsim

# Stop services
docker-compose down
```

## Verify It's Working

### Test Single Port

```bash
# Check if port is responding
nc -zv localhost 20000

# Or with SNMP
snmpget -v 2c -c public localhost:20000 1.3.6.1.2.1.1.5.0
```

### Test Multiple Ports

```bash
# Quick test of 10 random ports
bash test.sh 20000 10
```

### Full Walk Test

```bash
# Walk system OIDs
snmpwalk -v 2c -c public localhost:20000 1.3.6.1.2.1.1

# Walk interfaces
snmpwalk -v 2c -c public localhost:20000 1.3.6.1.2.1.2
```

## Configuration Examples

### Small Test Environment (10 devices)
```bash
./go-snmpsim -port-start=20000 -port-end=20010 -devices=10 -listen=0.0.0.0
```

### Medium Lab Setup (500 devices)
```bash
./go-snmpsim -port-start=20000 -port-end=20500 -devices=500 -listen=0.0.0.0
```

### Production Scale (5000 devices on port range 20K-30K)
```bash
# Build with optimizations
CGO_ENABLED=0 go build -ldflags "-s -w" -o snmpsim .

# Increase file descriptors first
ulimit -n 65536

# Run with full range
./go-snmpsim -port-start=20000 -port-end=30000 -devices=5000 -listen=0.0.0.0
```

## Integration with Monitoring Tools

### Nagios/Icinga
```bash
/usr/lib64/nagios/plugins/check_snmp \
  -H localhost:20000 \
  -C public \
  -o sysUpTime.0
```

### Manual Verification
```bash
# Test connection to port 20000
echo "Testing port 20000:"
snmpget -v 2c -c public -t 2 localhost:20000 .1.3.6.1.2.1.1.5.0 && echo "✓ Success" || echo "✗ Failed"

# Test connection to port 20500
echo "Testing port 20500:"
snmpget -v 2c -c public -t 2 localhost:20500 .1.3.6.1.2.1.1.5.0 && echo "✓ Success" || echo "✗ Failed"
```

## Common Issues & Solutions

### Port Already in Use
```bash
# Find process using port
lsof -i :20000

# Kill it
kill -9 <PID>
```

### File Descriptor Limit Exceeded
```bash
# Check current limit
ulimit -n

# Increase for current session
ulimit -n 65536

# Or make permanent in /etc/security/limits.conf
echo "username soft nofile 65536" | sudo tee -a /etc/security/limits.conf
echo "username hard nofile 65536" | sudo tee -a /etc/security/limits.conf
```

### No Response from Simulator
```bash
# Check if simulator is running
ps aux | grep go-snmpsim | grep -v grep

# Check listening ports
netstat -tulnp | grep 20000

# Verify network connectivity
nc -zv 127.0.0.1 20000
```

## Performance Verification

### Single Query Test
```bash
time snmpget -v 2c -c public localhost:20000 1.3.6.1.2.1.1.5.0
```

### Bulk Query Test (100 devices)
```bash
#!/bin/bash
for i in {0..99}; do
  port=$((20000 + $i))
  snmpget -v 2c -c public localhost:$port 1.3.6.1.2.1.1.5.0 2>/dev/null &
done
wait
echo "All queries completed"
```

### Monitor Resource Usage
```
# Method 1: Docker stats
docker stats snmpsim --no-stream

# Method 2: Process monitoring
top -p $(pgrep go-snmpsim)

# Method 3: Check file descriptors
lsof -p $(pgrep go-snmpsim) | wc -l
```

## Next Steps

1. **Load Testing**: See [TESTING.md](TESTING.md) for comprehensive testing guide
2. **Custom OIDs**: Create `.snmprec` file and load with `-snmprec` flag
3. **Scale Up**: Increase device count and monitor resource usage
4. **Production Deploy**: Check [README.md](README.md) for deployment best practices

## Support

For detailed documentation, see:
- [README.md](README.md) - Full architecture and features
- [TESTING.md](TESTING.md) - Comprehensive testing guide
- [docker-compose.yml](docker-compose.yml) - Docker deployment config

---

**Example Output:**
```
$ ./go-snmpsim -port-start=20000 -port-end=20010 -devices=5

2026/02/17 11:30:45 Starting SNMP Simulator
2026/02/17 11:30:45 Port range: 20000-20010
2026/02/17 11:30:45 Number of devices: 5
2026/02/17 11:30:45 File descriptor limit OK: 1024 (need ~105)
2026/02/17 11:30:45 Loaded 34 default OIDs
2026/02/17 11:30:45 Created 5 virtual agents across ports 20000-20004
2026/02/17 11:30:45 Started 5 UDP listeners
2026/02/17 11:30:45 Simulator started successfully

Press Ctrl+C to stop...

$ snmpget -v 2c -c public localhost:20000 1.3.6.1.2.1.1.5.0
SNMPv2-MIB::sysName.0 = STRING "Device-0"
```
