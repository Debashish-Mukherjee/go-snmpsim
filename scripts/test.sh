#!/bin/bash
# Test SNMP Simulator connectivity

echo "=== SNMP Simulator Test Suite ==="
echo ""

# Test parameters
PORT_START=${1:-20000}
PORT_SAMPLES=${2:-10}
HOST=${3:-localhost}

echo "Testing $PORT_SAMPLES random ports starting from $PORT_START..."
echo ""

# Generate random ports to test
for i in $(seq 1 $PORT_SAMPLES); do
    PORT=$((PORT_START + RANDOM % 1000))
    echo "Test $i - Port $PORT:"
    
    # Try snmpget
    if command -v snmpget &> /dev/null; then
        snmpget -v 2c -c public -t 1 -r 1 $HOST:$PORT 1.3.6.1.2.1.1.5.0 2>/dev/null && echo "  ✓ sysName query successful" || echo "  ✗ sysName query failed"
    else
        # Fallback to nc (netcat) for basic connectivity
        if nc -zv $HOST $PORT 2>/dev/null; then
            echo "  ✓ Port is open and listening"
        else
            echo "  ✗ Port is not responding"
        fi
    fi
    
    echo ""
done

echo ""
echo "=== Test Complete ==="
echo ""
echo "Useful SNMP commands:"
echo "  Get single OID:  snmpget -v 2c -c public localhost:20000 1.3.6.1.2.1.1.5.0"
echo "  Walk tree:       snmpwalk -v 2c -c public localhost:20000 1.3.6.1.2.1"
echo "  Bulk walk:       snmpbulkwalk -v 2c -c public localhost:20000 1.3.6.1.2.1"
echo "  Check port:      nc -zv localhost 20000"
echo ""
