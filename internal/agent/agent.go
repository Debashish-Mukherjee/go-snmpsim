package agent

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/store"
	"github.com/gosnmp/gosnmp"
)

// VirtualAgent represents a single simulated SNMP agent
type VirtualAgent struct {
	deviceID      int
	port          int
	sysName       string
	oidDB         *store.OIDDatabase
	indexManager  *store.OIDIndexManager  // Index manager for Zabbix LLD (table-aware)
	deviceMapping *store.DeviceOIDMapping // Device-specific OID overrides
	deviceOverlay map[string]interface{}  // Device-specific value overrides
	uptime        uint32
	startTime     time.Time
	lastPoll      time.Time
	pollCount     atomic.Int64

	mu sync.RWMutex
}

// NewVirtualAgent creates a new virtual SNMP agent
func NewVirtualAgent(deviceID int, port int, sysName string, oidDB *store.OIDDatabase) *VirtualAgent {
	return &VirtualAgent{
		deviceID:      deviceID,
		port:          port,
		sysName:       sysName,
		oidDB:         oidDB,
		indexManager:  nil,
		deviceMapping: nil,
		deviceOverlay: make(map[string]interface{}),
		startTime:     time.Now(),
		lastPoll:      time.Now(),
	}
}

// SetIndexManager assigns the index manager for Zabbix LLD support
func (va *VirtualAgent) SetIndexManager(im *store.OIDIndexManager) {
	va.mu.Lock()
	defer va.mu.Unlock()
	va.indexManager = im
}

// SetDeviceMapping assigns device-specific OID mappings to this agent
func (va *VirtualAgent) SetDeviceMapping(mapping *store.DeviceOIDMapping) {
	va.mu.Lock()
	defer va.mu.Unlock()
	va.deviceMapping = mapping
}

// HandlePacket processes an incoming SNMP packet and returns a response
func (va *VirtualAgent) HandlePacket(packet []byte) []byte {
	count := va.pollCount.Add(1)
	va.lastPoll = time.Now()

	// Log packet reception (sample every 1000th for high-volume scenarios)
	if count%1000 == 0 {
		log.Printf("Device %d (Port %d): Received packet #%d",
			va.deviceID, va.port, count)
	}

	decoder := gosnmp.GoSNMP{Version: gosnmp.Version2c, Community: "public"}
	req, err := decoder.SnmpDecodePacket(packet)
	if err != nil {
		log.Printf("Device %d: Failed to parse SNMP packet: %v", va.deviceID, err)
		return nil
	}

	switch req.PDUType {
	case gosnmp.GetNextRequest:
		return va.handleGetNextRequest(req)
	case gosnmp.SetRequest:
		return va.handleSetRequest(req)
	case gosnmp.GetBulkRequest:
		return va.handleGetBulkRequest(req)
	default:
		return va.handleGetRequest(req)
	}
}

// handleGetRequest processes GET requests
func (va *VirtualAgent) handleGetRequest(req *gosnmp.SnmpPacket) []byte {
	// Pre-allocate response variables
	vars := make([]gosnmp.SnmpPDU, 0, len(req.Variables))
	
	// Process each variable with minimal lock time
	for _, v := range req.Variables {
		va.mu.RLock()
		value := va.getOIDValue(v.Name)
		va.mu.RUnlock()
		
		vars = append(vars, gosnmp.SnmpPDU{
			Name:  v.Name,
			Type:  value.Type,
			Value: value.Value,
		})
	}

	// Marshal response without holding lock
	outPacket := &gosnmp.SnmpPacket{
		Version:   req.Version,
		Community: req.Community,
		PDUType:   gosnmp.GetResponse,
		RequestID: req.RequestID,
		Variables: vars,
	}

	// Marshal response
	data, err := outPacket.MarshalMsg()
	if err != nil {
		log.Printf("Device %d: Failed to marshal response: %v", va.deviceID, err)
		return nil
	}

	return data
}

// handleGetNextRequest processes GETNEXT requests (walk operation)
func (va *VirtualAgent) handleGetNextRequest(req *gosnmp.SnmpPacket) []byte {
	// Pre-allocate response variables
	vars := make([]gosnmp.SnmpPDU, 0, len(req.Variables))
	
	// Process each variable with minimal lock time
	for _, v := range req.Variables {
		va.mu.RLock()
		nextOID, val := va.getNextOID(v.Name)
		va.mu.RUnlock()
		
		if val != nil {
			vars = append(vars, gosnmp.SnmpPDU{
				Name:  nextOID,
				Type:  val.Type,
				Value: val.Value,
			})
		}
	}

	// Marshal response without holding lock
	outPacket := &gosnmp.SnmpPacket{
		Version:   req.Version,
		Community: req.Community,
		PDUType:   gosnmp.GetResponse,
		RequestID: req.RequestID,
		Variables: vars,
	}

	data, err := outPacket.MarshalMsg()
	if err != nil {
		log.Printf("Device %d: Failed to marshal response: %v", va.deviceID, err)
		return nil
	}

	return data
}

// handleGetBulkRequest processes GETBULK requests (efficient walk)
// Zabbix default: NonRepeaters=0, MaxRepeaters=10
func (va *VirtualAgent) handleGetBulkRequest(req *gosnmp.SnmpPacket) []byte {
	// Pre-allocate response variables
	nonRepeaters := int(req.NonRepeaters)
	if nonRepeaters < 0 {
		nonRepeaters = 0
	}
	maxRepeaters := int(req.MaxRepetitions)
	if maxRepeaters <= 0 {
		maxRepeaters = 10
	}

	vars := make([]gosnmp.SnmpPDU, 0, len(req.Variables)*maxRepeaters)

	// Process each variable with minimal lock time
	for i, v := range req.Variables {
		if i < nonRepeaters {
			va.mu.RLock()
			nextOID, val := va.getNextOID(v.Name)
			va.mu.RUnlock()
			
			if val != nil {
				vars = append(vars, gosnmp.SnmpPDU{
					Name:  nextOID,
					Type:  val.Type,
					Value: val.Value,
				})
			}
			continue
		}

		// For repeaters, get multiple consecutive OIDs
		currentOID := v.Name
		for r := 0; r < maxRepeaters; r++ {
			va.mu.RLock()
			nextOID, val := va.getNextOID(currentOID)
			va.mu.RUnlock()
			
			if val == nil || val.Type == gosnmp.EndOfMibView {
				break
			}
			vars = append(vars, gosnmp.SnmpPDU{
				Name:  nextOID,
				Type:  val.Type,
				Value: val.Value,
			})
			currentOID = nextOID
		}
	}

	outPacket := &gosnmp.SnmpPacket{
		Version:   req.Version,
		Community: req.Community,
		PDUType:   gosnmp.GetResponse,
		RequestID: req.RequestID,
		Variables: vars,
	}

	data, err := outPacket.MarshalMsg()
	if err != nil {
		log.Printf("Device %d: Failed to marshal GETBULK response: %v", va.deviceID, err)
		return nil
	}

	return data
}

// handleSetRequest returns read-only error response
func (va *VirtualAgent) handleSetRequest(req *gosnmp.SnmpPacket) []byte {
	outPacket := &gosnmp.SnmpPacket{
		Version:    req.Version,
		Community:  req.Community,
		PDUType:    gosnmp.GetResponse,
		RequestID:  req.RequestID,
		Error:      4, // readOnly error (SNMPv2)
		ErrorIndex: 1,
		Variables:  []gosnmp.SnmpPDU{},
	}

	data, err := outPacket.MarshalMsg()
	if err != nil {
		log.Printf("Device %d: Failed to marshal SET response: %v", va.deviceID, err)
		return nil
	}

	return data
}

// getOIDValue retrieves the value for a specific OID
// Priority: device mapping (port/device-specific) > device overlay > system OIDs > OID database
func (va *VirtualAgent) getOIDValue(oid string) *store.OIDValue {
	oid = normalizeOID(oid)
	// Check device mapping first (highest priority)
	if va.deviceMapping != nil {
		if val := va.deviceMapping.GetOID(oid, va.port, va.sysName); val != nil {
			return val
		}
	}

	// Check device overlay second
	if val, ok := va.deviceOverlay[oid]; ok {
		return &store.OIDValue{
			Type:  gosnmp.OctetString,
			Value: val,
		}
	}

	// Check for special system OIDs
	if val := va.getSystemOID(oid); val != nil {
		return val
	}

	// Query OID database
	val := va.oidDB.Get(oid)
	if val != nil {
		return val
	}

	// Return noSuchObject
	return &store.OIDValue{
		Type:  gosnmp.NoSuchObject,
		Value: nil,
	}
}

// getNextOID retrieves the next OID after the given one
// Uses index manager if available for table-aware traversal (Zabbix LLD)
func (va *VirtualAgent) getNextOID(oid string) (string, *store.OIDValue) {
	oid = normalizeOID(oid)
	// Try index manager first (optimized for table traversal)
	if va.indexManager != nil {
		return va.indexManager.GetNext(oid, va.oidDB)
	}

	// Fallback: basic database traversal
	nextOID := va.oidDB.GetNext(oid)
	if nextOID == "" {
		return oid, &store.OIDValue{
			Type:  gosnmp.EndOfMibView,
			Value: nil,
		}
	}

	value := va.getOIDValue(nextOID)
	return nextOID, value
}

func normalizeOID(oid string) string {
	if len(oid) > 0 && oid[0] == '.' {
		return oid[1:]
	}
	return oid
}

// getSystemOID returns system-specific OID values
func (va *VirtualAgent) getSystemOID(oid string) *store.OIDValue {
	switch oid {
	case "1.3.6.1.2.1.1.3.0": // sysUpTime
		uptime := uint32(time.Since(va.startTime).Seconds() * 100)
		return &store.OIDValue{
			Type:  gosnmp.TimeTicks,
			Value: uptime,
		}

	case "1.3.6.1.2.1.1.5.0": // sysName
		return &store.OIDValue{
			Type:  gosnmp.OctetString,
			Value: va.sysName,
		}

	case "1.3.6.1.2.1.1.6.0": // sysLocation
		return &store.OIDValue{
			Type:  gosnmp.OctetString,
			Value: fmt.Sprintf("Simulated-Device-%d", va.deviceID),
		}

	case "1.3.6.1.2.1.25.3.2.1.5.1": // hrDeviceIndex (random CPU load simulation)
		cpuLoad := rand.Intn(100)
		return &store.OIDValue{
			Type:  gosnmp.Integer,
			Value: cpuLoad,
		}
	}

	return nil
}

// SetOIDValue sets a device-specific OID value (overlay)
func (va *VirtualAgent) SetOIDValue(oid string, value interface{}) {
	va.mu.Lock()
	defer va.mu.Unlock()
	va.deviceOverlay[oid] = value
}

// GetStatistics returns agent statistics
func (va *VirtualAgent) GetStatistics() map[string]interface{} {
	va.mu.RLock()
	defer va.mu.RUnlock()

	uptime := uint32(time.Since(va.startTime).Seconds())
	return map[string]interface{}{
		"device_id":  va.deviceID,
		"port":       va.port,
		"sysName":    va.sysName,
		"uptime":     uptime,
		"poll_count": va.pollCount.Load(),
		"last_poll":  va.lastPoll.Format(time.RFC3339),
	}
}
