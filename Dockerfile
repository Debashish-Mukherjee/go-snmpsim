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
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o snmpsim .

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tcpdump netcat-openbsd

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/snmpsim .

# Create default data directory
RUN mkdir -p /app/data

# Expose port range needed for SNMP (example: 20000-30000)
# Note: Docker doesn't support port ranges in EXPOSE, so we document it
# Users should run with: docker run -p 20000-30000:20000-30000/udp

# Set environment for file descriptors
ENV GOMAXPROCS=8

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD nc -zv localhost 20000 || exit 1

# Default command
ENTRYPOINT ["/app/snmpsim"]

# Default arguments
CMD ["-port-start=20000", "-port-end=30000", "-devices=1000", "-listen=0.0.0.0"]
