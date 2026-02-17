#!/bin/bash
# Standalone deployment script for SNMP Simulator

set -e

PORT_START=${1:-20000}
PORT_END=${2:-30000}
DEVICES=${3:-100}
LISTEN=${4:-0.0.0.0}

echo "=== SNMP Simulator Deployment ==="
echo "Port Range: $PORT_START-$PORT_END"
echo "Devices: $DEVICES"
echo "Listen Address: $LISTEN"
echo ""

# Check file descriptors
echo "Checking system configuration..."
CURRENT_FD=$(ulimit -n)
REQUIRED_FD=$((($PORT_END - $PORT_START) + 100))

echo "Current file descriptor limit: $CURRENT_FD"
echo "Required for this config: $REQUIRED_FD"

if [ $CURRENT_FD -lt $REQUIRED_FD ]; then
    echo "WARNING: File descriptor limit may be insufficient!"
    echo "To increase, run: ulimit -n $((REQUIRED_FD * 2))"
    echo ""
fi

# Build the binary
echo "Building SNMP Simulator..."
go build -o snmpsim .

# Run the simulator
echo "Starting SNMP Simulator..."
./snmpsim \
    -port-start=$PORT_START \
    -port-end=$PORT_END \
    -devices=$DEVICES \
    -listen=$LISTEN
