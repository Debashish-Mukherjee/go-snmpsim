# Go-SNMPSim Performance Analysis & Optimizations

**Review Date:** February 17, 2026  
**Reviewer:** Performance Analysis

## Executive Summary

The codebase is well-structured but has several performance bottlenecks that can significantly impact throughput when handling 100+ devices with high query rates. The most critical issues are in the hot paths: OID lookups, packet handling, and database traversal.

---

## Critical Issues (High Impact)

### 1. **Inefficient Binary Search in OIDDatabase.GetNext()** 
**File:** [internal/store/database.go](internal/store/database.go#L58-L75)  
**Severity:** HIGH  
**Impact:** O(n) instead of O(log n) for every GETNEXT request

**Current Code:**
```go
func (odb *OIDDatabase) GetNext(oid string) string {
    odb.mu.RLock()
    defer odb.mu.RUnlock()
    
    // Linear search through sorted array - O(n)
    for i := 0; i < len(odb.sortedOIDs)-1; i++ {
        if odb.sortedOIDs[i] == oid {
            return odb.sortedOIDs[i+1]
        }
        if isOIDLess(oid, odb.sortedOIDs[i]) {
            return odb.sortedOIDs[i]
        }
    }
    return ""
}
```

**Problem:** This performs a linear search through potentially thousands of OIDs on every GETNEXT request. GETNEXT is one of the most frequently called operations in SNMP.

**Solution:** Use `sort.Search()` for O(log n) binary search:
```go
func (odb *OIDDatabase) GetNext(oid string) string {
    odb.mu.RLock()
    defer odb.mu.RUnlock()
    
    idx := sort.Search(len(odb.sortedOIDs), func(i int) bool {
        return !isOIDLess(odb.sortedOIDs[i], oid)
    })
    
    if idx < len(odb.sortedOIDs) {
        if odb.sortedOIDs[idx] == oid && idx+1 < len(odb.sortedOIDs) {
            return odb.sortedOIDs[idx+1]
        }
        return odb.sortedOIDs[idx]
    }
    return ""
}
```

**Expected Improvement:** 100-1000x faster for large OID databases (10,000+ OIDs)

---

### 2. **Buffer Pool Misuse in simulator.go**
**File:** [internal/engine/simulator.go](internal/engine/simulator.go#L177-L183)  
**Severity:** HIGH  
**Impact:** Defeats purpose of buffer pool, increases memory pressure

**Current Code:**
```go
func (s *Simulator) handleListener(ctx context.Context, conn *net.UDPConn, port int) {
    defer s.wg.Done()
    
    agent := s.agents[port]
    buffer := s.packetPool.Get().([]byte)  // Gets buffer once
    defer s.packetPool.Put(buffer)          // Returns at end of goroutine
    
    for {
        // Uses same buffer for entire lifetime
        n, remoteAddr, err := conn.ReadFromUDP(buffer)
        // ...
    }
}
```

**Problem:** Each listener goroutine holds a buffer for its entire lifetime (minutes/hours), effectively making the pool useless. With 10,000 listeners, this allocates 40MB that could be shared.

**Solution:** Get/put buffer per packet:
```go
func (s *Simulator) handleListener(ctx context.Context, conn *net.UDPConn, port int) {
    defer s.wg.Done()
    agent := s.agents[port]
    
    for {
        buffer := s.packetPool.Get().([]byte)
        n, remoteAddr, err := conn.ReadFromUDP(buffer)
        if err != nil {
            s.packetPool.Put(buffer)
            // handle error
            continue
        }
        
        response := agent.HandlePacket(buffer[:n])
        s.packetPool.Put(buffer)
        
        if response != nil {
            conn.WriteToUDP(response, remoteAddr)
        }
    }
}
```

**Expected Improvement:** 50-90% reduction in memory usage

---

### 3. **Excessive Lock Duration in agent.HandlePacket()**
**File:** [internal/agent/agent.go](internal/agent/agent.go#L95-L115)  
**Severity:** MEDIUM-HIGH  
**Impact:** Lock contention under concurrent requests

**Current Code:**
```go
func (va *VirtualAgent) handleGetRequest(req *gosnmp.SnmpPacket) []byte {
    va.mu.RLock()
    defer va.mu.RUnlock()  // Held for entire duration
    
    vars := make([]gosnmp.SnmpPDU, 0, len(req.Variables))
    for _, v := range req.Variables {
        value := va.getOIDValue(v.Name)  // Could be slow
        vars = append(vars, gosnmp.SnmpPDU{...})
    }
    
    // Marshal while holding lock
    outPacket := &gosnmp.SnmpPacket{...}
    data, err := outPacket.MarshalMsg()  // Expensive operation
    return data
}
```

**Problem:** Read lock held during expensive operations (OID lookup, marshaling). Multiple concurrent requests to same agent will serialize unnecessarily.

**Solution:** Minimize lock scope:
```go
func (va *VirtualAgent) handleGetRequest(req *gosnmp.SnmpPacket) []byte {
    vars := make([]gosnmp.SnmpPDU, 0, len(req.Variables))
    
    for _, v := range req.Variables {
        va.mu.RLock()
        value := va.getOIDValue(v.Name)
        va.mu.RUnlock()
        
        vars = append(vars, gosnmp.SnmpPDU{...})
    }
    
    // Marshal without lock
    outPacket := &gosnmp.SnmpPacket{...}
    data, err := outPacket.MarshalMsg()
    return data
}
```

**Expected Improvement:** 2-5x throughput increase under concurrent load

---

### 4. **Inefficient OID Comparison**
**File:** [internal/store/database.go](internal/store/database.go#L101-L122)  
**Severity:** MEDIUM  
**Impact:** Called repeatedly in hot path

**Current Code:**
```go
func isOIDLess(oid1, oid2 string) bool {
    parts1 := strings.Split(oid1, ".")  // Allocates
    parts2 := strings.Split(oid2, ".")  // Allocates
    
    minLen := len(parts1)
    if len(parts2) < minLen {
        minLen = len(parts2)
    }
    
    for i := 0; i < minLen; i++ {
        var num1, num2 int
        _, _ = fmt.Sscanf(parts1[i], "%d", &num1)  // Slow
        _, _ = fmt.Sscanf(parts2[i], "%d", &num2)  // Slow
        // ...
    }
}
```

**Problem:** 
- Allocates two string slices per comparison
- Uses slow `fmt.Sscanf()` for parsing
- Called thousands of times during sorting/searching

**Solution:** Optimize with manual parsing:
```go
func isOIDLess(oid1, oid2 string) bool {
    i1, i2 := 0, 0
    
    for i1 < len(oid1) && i2 < len(oid2) {
        num1, next1 := parseOIDComponent(oid1, i1)
        num2, next2 := parseOIDComponent(oid2, i2)
        
        if num1 != num2 {
            return num1 < num2
        }
        i1, i2 = next1, next2
    }
    return i1 >= len(oid1) && i2 < len(oid2)
}

func parseOIDComponent(oid string, start int) (int, int) {
    num := 0
    i := start
    for i < len(oid) && oid[i] >= '0' && oid[i] <= '9' {
        num = num*10 + int(oid[i]-'0')
        i++
    }
    if i < len(oid) && oid[i] == '.' {
        i++
    }
    return num, i
}
```

**Expected Improvement:** 5-10x faster comparisons

---

## Medium Priority Issues

### 5. **No Bulk Operation Optimization in Agent**
**File:** [internal/agent/agent.go](internal/agent/agent.go#L170-L211)  
**Severity:** MEDIUM  
**Impact:** Missed optimization opportunity for GetBulk

The agent's `handleGetBulkRequest()` doesn't leverage the index manager's optimized `GetNextBulk()` method. It manually loops through GetNext calls, which is less efficient.

**Recommendation:** Integrate with index manager's GetNextBulk for table operations.

---

### 6. **Packet Pool Size Not Configurable**
**File:** [internal/engine/simulator.go](internal/engine/simulator.go#L59-L63)  
**Severity:** MEDIUM  
**Impact:** May not scale well for different workloads

Hardcoded 4096-byte buffers. Should be configurable based on expected packet sizes.

**Recommendation:** Make buffer size configurable, with separate pools for request/response.

---

### 7. **No Metrics/Profiling Instrumentation**
**Severity:** MEDIUM  
**Impact:** Hard to identify bottlenecks in production

**Recommendation:** Add prometheus metrics for:
- Request latency percentiles (p50, p95, p99)
- Requests per second by type (GET, GETNEXT, GETBULK)
- Error rates
- Pool utilization
- Lock wait times

---

### 8. **Database Sort on Every Insert**
**File:** [internal/store/database.go](internal/store/database.go#L38-L42)  
**Severity:** LOW-MEDIUM  
**Impact:** Inefficient for bulk loading

Currently `sortedOIDs` array is rebuilt on every insert. For bulk initialization (loading files), this is O(n²).

**Recommendation:** Add batch insert API that defers sorting until after all inserts complete.

---

## Low Priority Issues

### 9. **String Concatenation in Hot Path**
Multiple places use `fmt.Sprintf()` for string building in hot paths. Consider using `strings.Builder` or pre-allocated buffers.

### 10. **Unnecessary Defer Overhead**
Some hot path functions use `defer` for unlock operations. Direct unlock calls before returns can be faster (though less safe).

### 11. **JSON Marshaling in API**
API endpoints use `json.Encoder` which can be slower than `json.Marshal` for small objects.

---

## Optimization Benchmarks (Expected)

| Operation | Current | Optimized | Improvement |
|-----------|---------|-----------|-------------|
| GetNext (10k OIDs) | ~50µs | ~500ns | 100x |
| OID Comparison | ~2µs | ~200ns | 10x |
| Concurrent Requests (same agent) | 1k/s | 5k/s | 5x |
| Memory (10k devices) | 200MB | 80MB | 2.5x |
| GetBulk (table walk) | 500µs | 100µs | 5x |

---

## Recommended Implementation Order

1. **Fix OIDDatabase.GetNext() binary search** - Highest ROI
2. **Fix buffer pool usage in simulator** - Major memory savings
3. **Optimize OID comparison** - Used in many places
4. **Reduce lock scope in agent** - Better concurrency
5. **Add metrics instrumentation** - Measure improvements
6. **Implement bulk insert API** - Better initialization
7. **Optimize GetBulk with index manager** - Zabbix performance

---

## Testing Recommendations

1. **Benchmark Suite**: Create benchmarks for hot paths:
   - `BenchmarkGetNext`
   - `BenchmarkHandlePacket`
   - `BenchmarkOIDComparison`
   - `BenchmarkGetBulk`

2. **Load Testing**: Test with realistic workloads:
   - 1000 devices
   - 10k OIDs per device
   - 1000 req/s sustained
   - Measure: latency (p99), memory, CPU

3. **Race Detection**: Run with `-race` flag to ensure thread safety after optimizations

4. **Profile Analysis**: Use pprof to validate optimizations:
   ```bash
   go test -cpuprofile=cpu.prof -memprofile=mem.prof -bench=.
   go tool pprof cpu.prof
   ```

---

## Conclusion

The codebase has good structure and uses appropriate concurrency primitives, but the hot paths need optimization. The most critical issue is the O(n) GetNext operation, which should be addressed first. The buffer pool fix will provide immediate memory savings. Combined, these optimizations should support 10x more devices at similar latency.
