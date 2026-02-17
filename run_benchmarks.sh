#!/bin/bash

# Performance benchmark runner for go-snmpsim optimizations
# Run this to validate performance improvements

set -e

echo "========================================="
echo "Running Performance Benchmarks"
echo "========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Change to project directory
cd "$(dirname "$0")"

echo -e "${YELLOW}1. Building binaries...${NC}"
go build -o /dev/null ./cmd/snmpsim/

echo ""
echo -e "${YELLOW}2. Running store package benchmarks...${NC}"
cd internal/store
go test -bench=. -benchmem -benchtime=3s | tee ../../benchmark_results.txt

echo ""
echo -e "${YELLOW}3. Running with CPU profiling...${NC}"
go test -bench=BenchmarkGetNext -cpuprofile=cpu.prof -memprofile=mem.prof -benchtime=10s

echo ""
echo -e "${GREEN}4. Generating CPU profile report...${NC}"
go tool pprof -top -nodecount=20 cpu.prof > ../../cpu_profile.txt
echo "CPU profile saved to: cpu_profile.txt"

echo ""
echo -e "${GREEN}5. Generating memory profile report...${NC}"
go tool pprof -top -nodecount=20 mem.prof > ../../mem_profile.txt
echo "Memory profile saved to: mem_profile.txt"

cd ../..

echo ""
echo -e "${YELLOW}6. Running race detector tests...${NC}"
go test -race ./internal/store -run TestGetNextCorrectness
go test -race ./internal/agent -run . -short

echo ""
echo "========================================="
echo -e "${GREEN}Benchmark suite complete!${NC}"
echo "========================================="
echo ""
echo "Results:"
echo "  - Benchmark results: benchmark_results.txt"
echo "  - CPU profile:       cpu_profile.txt"
echo "  - Memory profile:    mem_profile.txt"
echo ""
echo "To view interactive CPU profile:"
echo "  cd internal/store && go tool pprof -http=:8081 cpu.prof"
echo ""
echo "To compare with baseline (if you have old results):"
echo "  benchstat old_results.txt benchmark_results.txt"
echo ""
