.DEFAULT_GOAL := build

# Build variables
VERSION ?= 1.0.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Binary name
BINARY_NAME := snmpsim
BINARY_PATH := ./$(BINARY_NAME)
CMD_PATH := ./cmd/snmpsim

# Go variables
GO := go
GOFLAGS := -v
GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)

# Docker variables
DOCKER_IMAGE := go-snmpsim
DOCKER_TAG := latest
DOCKER_CONTAINER ?= snmpsim-alpine

.PHONY: help build run clean test lint docker docker-start docker-stop docker-logs docker-clean

help:
	@echo "SNMP Simulator - Available targets:"
	@echo ""
	@echo "  make build           - Build the binary"
	@echo "  make run             - Build and run the simulator"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make test            - Run tests (if any)"
	@echo "  make lint            - Run linters"
	@echo "  make fmt             - Format code"
	@echo "  make docker          - Build Docker image"
	@echo "  make docker-start    - Start simulator in Docker container (detached)"
	@echo "  make docker-stop     - Stop and remove simulator container"
	@echo "  make docker-logs     - Show simulator container logs"
	@echo "  make docker-clean    - Clean Docker artifacts"
	@echo "  make install         - Install dependencies"
	@echo "  make help            - Show this help message"
	@echo ""
	@echo "Docker options:"
	@echo "  DOCKER_CONTAINER     - Container name (default: $(DOCKER_CONTAINER))"
	@echo ""

## Build Targets

install:
	@echo "Installing Go modules..."
	$(GO) mod download
	$(GO) mod tidy

build: install
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_PATH) $(CMD_PATH)
	@echo "✓ Binary built: $(BINARY_PATH)"
	@ls -lh $(BINARY_PATH)

build-release: install
	@echo "Building optimized release binary..."
	CGO_ENABLED=0 $(GO) build -ldflags "-s -w $(LDFLAGS)" -o $(BINARY_PATH)-release $(CMD_PATH)
	@echo "✓ Release binary built (stripped and optimized)"
	@ls -lh $(BINARY_PATH)-release

clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_PATH) $(BINARY_PATH)-release
	$(GO) clean
	$(GO) clean -testcache
	@echo "✓ Clean complete"

## Run Targets

run: build
	@echo "Starting SNMP Simulator..."
	$(BINARY_PATH) -port-start=20000 -port-end=30000 -devices=100 -listen=0.0.0.0

run-small: build
	@echo "Starting SNMP Simulator (small test)..."
	$(BINARY_PATH) -port-start=20000 -port-end=20010 -devices=5 -listen=0.0.0.0

run-large: build
	@echo "Starting SNMP Simulator (large scale)..."
	ulimit -n 65536 || true
	$(BINARY_PATH) -port-start=20000 -port-end=30000 -devices=1000 -listen=0.0.0.0

## Code Quality Targets

test:
	@echo "Running tests..."
	$(GO) test -v ./...

fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	@echo "✓ Code formatted"

lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

vet:
	@echo "Running go vet..."
	$(GO) vet ./...

## Docker Targets

docker: install
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "✓ Docker image built"
	@docker images | grep $(DOCKER_IMAGE)

docker-start: docker
	@echo "Starting SNMP Simulator in Docker..."
	docker run -d \
		--name $(DOCKER_CONTAINER) \
		-p 20000-30000:20000-30000/udp \
		-p 8080:8080 \
		-v $(PWD)/config:/app/config \
		-e GOMAXPROCS=4 \
		$(DOCKER_IMAGE):$(DOCKER_TAG) \
		-port-start=20000 -port-end=30000 -devices=100 -web-port=8080 -listen=0.0.0.0
	@echo "✓ Simulator started (100 devices on ports 20000-30000, Web UI on 8080)"
	docker ps | grep $(DOCKER_CONTAINER)

docker-stop:
	@echo "Stopping SNMP Simulator container..."
	docker rm -f $(DOCKER_CONTAINER) 2>/dev/null || true
	@echo "✓ Container stopped and removed"

docker-logs:
	docker logs -f $(DOCKER_CONTAINER)

docker-clean: docker-stop
	@echo "Cleaning Docker images..."
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) 2>/dev/null || true
	@echo "✓ Docker cleanup complete"

## Utility Targets

check-deps:
	@echo "Checking dependencies..."
	@which snmpget > /dev/null && echo "✓ snmpget found" || echo "✗ snmpget not found (install net-snmp-utils)"
	@which nc > /dev/null && echo "✓ nc (netcat) found" || echo "✗ nc not found"
	@which docker > /dev/null && echo "✓ Docker found" || echo "✗ Docker not found"

test-connectivity:
	@echo "Testing connectivity to port 20000..."
	@nc -zv -w 1 localhost 20000 || echo "Port 20000 not responding"

check-fd-limit:
	@echo "Checking file descriptor limit..."
	@current=$$(ulimit -n); echo "Current limit: $$current"; \
	if [ $$current -lt 1024 ]; then \
		echo "⚠ WARNING: Increase with: ulimit -n 65536"; \
	else \
		echo "✓ Limit OK"; \
	fi

info:
	@echo "=== SNMP Simulator Build Info ==="
	@echo "Version: $(VERSION)"
	@echo "Build: $(BUILD_TIME)"
	@echo "Commit: $(GIT_COMMIT)"
	@echo "OS: $(GOOS)"
	@echo "Arch: $(GOARCH)"
	@echo "Go Version: $$($(GO) version)"
	@echo ""

.PHONY: all
all: clean lint test build docker
	@echo "✓ All targets completed"
