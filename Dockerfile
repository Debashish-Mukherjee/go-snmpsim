FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go.mod and go.sum
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o snmpsim ./cmd/snmpsim

# Final stage
FROM alpine:3.20

# Install runtime dependencies including net-snmp tools for SNMP testing
RUN apk --no-cache add \
    ca-certificates \
    tcpdump \
    netcat-openbsd \
    net-snmp-tools \
    curl

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/snmpsim .

# Copy web UI assets
COPY --from=builder /build/web ./web

# Create default data directories
RUN mkdir -p /app/data /app/config/workloads

# Expose port range for SNMP (20000-30000) and Web UI (8080)
# Docker doesn't support port ranges in EXPOSE, so we document the range
# Users should run with: docker run -p 20000-30000:20000-30000/udp -p 8080:8080
EXPOSE 8080

# Set environment for file descriptors
ENV GOMAXPROCS=8

# Health check using HTTP endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/status || exit 1

# Default command with Web UI enabled
ENTRYPOINT ["/app/snmpsim"]
CMD ["-port-start=20000", "-port-end=30000", "-devices=100", "-web-port=8080", "-listen=0.0.0.0"]
