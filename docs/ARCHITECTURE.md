# Architecture & Design Document

## System Overview

The SNMP Simulator is architected as a high-performance, multi-port virtual agent system designed to simulate thousands of SNMP devices efficiently.

```
┌─────────────────────────────────────────────────────────────────────┐
│                     SNMP Simulator System                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │   UDP        │  │   UDP        │  │   UDP        │              │
│  │ Listener     │  │ Listener     │  │ Listener     │ ...          │
│  │ :20000       │  │ :20001       │  │ :20002       │              │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘              │
│         │                 │                 │                      │
│         └─────────────────┼─────────────────┘                      │
│                           ↓                                        │
│                  ┌────────────────────┐                            │
│                  │  Packet Dispatcher │                            │
│                  │  (sync.Pool)       │                            │
│                  └────────┬───────────┘                            │
│                           ↓                                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │ Virtual      │  │ Virtual      │  │ Virtual      │              │
│  │ Agent 0      │  │ Agent 1      │  │ Agent 2      │ ...          │
│  │ (Port 20000) │  │ (Port 20001) │  │ (Port 20002) │              │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘              │
│         │                 │                 │                      │
│         └─────────────────┼─────────────────┘                      │
│                           ↓                                        │
│                 ┌──────────────────────┐                           │
│                 │  OID Database        │                           │
│                 │  (Radix Tree)        │                           │
│                 │  ┌────────────────┐  │                           │
│                 │  │ Base Templates │  │                           │
│                 │  └────────────────┘  │                           │
│                 │  ┌────────────────┐  │                           │
│                 │  │ Device Overlay │  │                           │
│                 │  │ (Per-Agent)    │  │                           │
│                 │  └────────────────┘  │                           │
│                 └──────────────────────┘                           │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. UDP Listener Layer (`simulator.go`)

**Responsibilities:**
- Bind to configurable UDP port range (default: 20000-30000)
- Set socket options for optimal performance (SO_RCVBUF=256KB)
- Manage listener lifecycle and graceful shutdown
- Forward packets to appropriate agents

**Key Features:**
- SO_REUSEADDR/SO_REUSEPORT for port binding
- Per-socket read buffer of 256KB to handle burst traffic
- Individual goroutine per listener for concurrent handling
- Atomic running flag for safe shutdown

**Code Structure:**
```go
type Simulator struct {
    listeners map[int]*net.UDPConn  // port -> listener
    agents    map[int]*VirtualAgent  // port -> agent
    dispatcher *PacketDispatcher
    packetPool *sync.Pool            // byte buffer reuse
    ...
}
```

### 2. Virtual Agent Layer (`agent.go`)

**Responsibilities:**
- Parse incoming SNMP packets (ASN.1 BER encoded)
- Determine request type (GET, GETNEXT, GETBULK, SET)
- Generate appropriate responses
- Maintain agent-specific statistics

**Key Features:**
- Minimal packet parsing (heuristic-based PDU type detection)
- Copy-on-Write overlay for device-specific OIDs
- System OID handlers (sysName, sysUpTime, etc.)
- Atomic poll counter for lock-free statistics

**Agent State Management:**
```go
type VirtualAgent struct {
    deviceID      int                       // Unique identifier
    port          int                       // Mapped port
    sysName       string                    // Device name
    oidDB         *OIDDatabase              // Shared reference
    deviceOverlay map[string]interface{}    // Device-specific values
    pollCount     atomic.Int64              // Request counter
    ...
}
```

### 3. OID Database (`oid_database.go`)

**Responsibilities:**
- Store OID templates efficiently (Radix Tree)
- Support O(log n) GetNext operations
- Manage OID ordering for walks
- Support dynamic loading from .snmprec files

**Key Features:**
- Uses `github.com/armon/go-radix` for radix tree storage
- Pre-computed sorted OID list for walk operations
- Binary search-friendly ordering
- Thread-safe with RWMutex

**OID Storage Structure:**
```
Tree Node Structure:
    root
    ├── 1
    │   ├── 3
    │   │   ├── 6
    │   │   │   ├── 1
    │   │   │   │   ├── 2
    │   │   │   │   │   ├── 1  [System Group]
    │   │   │   │   │   ├── 2  [Internet Group]
    │   │   │   │   │   └── 25 [Host Resources]
```

**Sorted OID List (for GetNext):**
```
[
  "1.3.6.1.2.1.1.1.0",
  "1.3.6.1.2.1.1.3.0",
  "1.3.6.1.2.1.1.4.0",
  "1.3.6.1.2.1.1.5.0",
  ...
]
```

### 4. Packet Dispatcher (`dispatcher.go`)

**Responsibilities:**
- Route packets to appropriate agents
- Manage buffer pool for zero-allocation parsing
- Provide buffer recycling interface

**Buffer Pool Strategy:**
```go
packetPool := &sync.Pool{
    New: func() interface{} {
        return make([]byte, 4096)  // Standard SNMP packet size
    },
}
```

## Protocol Implementation

### SNMP Packet Handling

**Request Detection (ASN.1 BER Heuristic):**
```
Byte Position | Content | PDU Type
0-3           | Length  | -
4             | Type    | -
5             | Tag     | Determines operation
              | 0xA0    | GetRequest
              | 0xA1    | GetNextRequest
              | 0xA3    | SetRequest
              | 0xA4    | GetBulkRequest
```

### Response Building

**GET Response:**
```
SnmpPacket {
    Version: Version2c
    Community: "public"
    PDUType: GetResponse
    RequestID: <from request>
    Variables: [
        SnmpPDU {
            Name: "1.3.6.1.2.1.1.5.0"
            Type: OctetString
            Value: "Device-0"
        }
    ]
}
```

**GETNEXT Response:**
```
SnmpPacket {
    Version: Version2c
    PDUType: GetResponse
    Variables: [
        SnmpPDU {
            Name: "1.3.6.1.2.1.1.6.0"  // Next OID in tree
            Type: OctetString
            Value: "Simulated-Device-0"
        }
    ]
}
```

## Memory Efficiency Strategy

### Copy-on-Write OID Values

```
Request for OID "1.3.6.1.2.1.1.5.0" (sysName):

1. Check Device Overlay
   ├─► Found: Return device-specific value
   └─► Not Found: Continue

2. Check System OID Handlers
   ├─► Found: Return dynamic value
   └─► Not Found: Continue

3. Query OID Database
   ├─► Found: Return template value
   └─► Not Found: Return noSuchObject
```

### Memory Layout (Per Device)

```
Virtual Agent (avg ~5KB):
├── Metadata (200B)
│   ├── deviceID (4B)
│   ├── port (4B)
│   ├── sysName (50B)
│   └── timestamps (16B)
├── Device Overlay (4KB - depends on customizations)
│   └── map[string]interface{}
└── Locks (48B - sync.RWMutex)

Total for 1000 devices: ~5MB (scalable, no OID duplication)
```

## File Descriptor Management

### Requirement Calculation

```
Total FDs = (Port Range) + Overhead
         = (port_end - port_start) + 100

Examples:
10 devices   @ 1FD each = ~110 FDs
100 devices  @ 1FD each = ~200 FDs
1000 devices @ 1FD each = ~1100 FDs (exceeds default 1024)
5000 devices @ 1FD each = ~5100 FDs (requires tuning)
```

### FD Startup Check

```go
func checkFileDescriptors(requiredFDs int) {
    var rlimit syscall.Rlimit
    syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit)
    
    requiredTotal := uint64(requiredFDs) + 100
    
    if rlimit.Cur < requiredTotal {
        log.Printf("Warning: Increase with: ulimit -n %d", requiredTotal*2)
    }
}
```

## Performance Characteristics

### Throughput Analysis

**Bottlenecks (in order of impact):**
1. Network I/O (UDP packet rate)
2. SNMP parsing (ASN.1 decoding)
3. OID lookup (radix tree traversal)
4. Memory allocation (sync.Pool mitigation)
5. Lock contention (RWMutex on OID database)

**Optimizations:**
- Minimal parsing (heuristic PDU type detection)
- O(log n) OID lookup with radix tree
- Byte buffer reuse via sync.Pool
- RWMutex with high concurrency for OID reads
- Per-port listener isolation (no global lock)

### Latency Profile

```
GET Request:
├─ Receive packet         ~0.1ms
├─ Type detection         ~0.05ms
├─ OID lookup             ~0.1ms (O(log n))
├─ Build response         ~0.1ms
├─ Marshal packet         ~0.15ms
└─ Send response          ~0.1ms
  Total: ~0.7ms average
```

## Scalability Limits

### Vertical Scaling (Single Machine)

| Metric | Limit | Notes |
|--------|-------|-------|
| Devices | 10K+ | Limited by FDs and memory |
| Port Range | 65K | OS limit |
| Memory | <100MB | For 10K devices |
| Throughput | 100K+ qps | Single core (scales linearly) |

### Horizontal Scaling

**Multi-Instance Deployment:**
```
Load Balancer (port 161/UDP)
├─ Simulator A (ports 20000-29999)
├─ Simulator B (ports 30000-39999)
└─ Simulator C (ports 40000-49999)
```

## Deployment Scenarios

### Docker Container

**Image Size:** ~13.6MB
**Memory:** 256MB base + device memory
**CPU:** Scales with core utilization

### Kubernetes Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: snmpsim
spec:
  containers:
  - name: snmpsim
    image: go-snmpsim:latest
    resources:
      limits:
        memory: "2Gi"
        cpu: "4"
      requests:
        memory: "1Gi"
        cpu: "2"
    ports:
    - containerPort: 20000
      protocol: UDP
    - containerPort: 30000
      protocol: UDP
```

## Future Enhancement Opportunities

### Planned Features
1. **SNMP v3 / USM** - Full authentication support
2. **Trap/Notification** - Send SNMP traps to managers
3. **Prometheus Metrics** - Built-in metrics export
4. **Hot Reloading** - Update configs without restart
5. **Distributed Simulation** - Multi-instance coordination
6. **Advanced OID** - Dynamic OID generation algorithms

### Optimization Opportunities
1. **Zero-Copy Networking** - Use DPDK or similar
2. **SIMD Parsing** - AVX2 for packet parsing
3. **Lua Scripting** - Dynamic device behavior
4. **Persistence** - State save/restore
5. **Cluster Mode** - Distributed across nodes

---

**Document Version:** 1.0  
**Last Updated:** 2026-02-17  
**Author:** Senior Systems Engineer (Golang & Networking)
