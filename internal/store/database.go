package store

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/armon/go-radix"
	"github.com/gosnmp/gosnmp"
)

// OIDDatabase manages OID storage with efficient lookup and walk operations
type OIDDatabase struct {
	tree       *radix.Tree
	sortedOIDs []string // Pre-sorted OIDs for efficient GetNext
	mu         sync.RWMutex
}

// OIDValue represents a value in the OID database
type OIDValue struct {
	Type  gosnmp.Asn1BER
	Value interface{}
}

// NewOIDDatabase creates a new OID database
func NewOIDDatabase() *OIDDatabase {
	return &OIDDatabase{
		tree:       radix.New(),
		sortedOIDs: make([]string, 0),
	}
}

// Insert adds an OID and its value to the database
func (odb *OIDDatabase) Insert(oid string, value *OIDValue) {
	odb.mu.Lock()
	defer odb.mu.Unlock()

	odb.tree.Insert(oid, value)
	odb.sortedOIDs = append(odb.sortedOIDs, oid)
}

// Get retrieves the value for an OID
func (odb *OIDDatabase) Get(oid string) *OIDValue {
	odb.mu.RLock()
	defer odb.mu.RUnlock()

	if val, ok := odb.tree.Get(oid); ok {
		return val.(*OIDValue)
	}
	return nil
}

// GetNext retrieves the next OID after the given one (for GETNEXT operations)
func (odb *OIDDatabase) GetNext(oid string) string {
	odb.mu.RLock()
	defer odb.mu.RUnlock()

	// Find the next OID in sorted order
	for i := 0; i < len(odb.sortedOIDs)-1; i++ {
		if odb.sortedOIDs[i] == oid {
			return odb.sortedOIDs[i+1]
		}
		// Handle OID ordering (sub-OIDs)
		if isOIDLess(oid, odb.sortedOIDs[i]) {
			return odb.sortedOIDs[i]
		}
	}

	// No next OID found
	return ""
}

// Walk traverses all OIDs in the database (used for bulk operations)
func (odb *OIDDatabase) Walk(callback func(oid string, value *OIDValue) bool) {
	odb.mu.RLock()
	defer odb.mu.RUnlock()

	for _, oid := range odb.sortedOIDs {
		val, _ := odb.tree.Get(oid)
		if !callback(oid, val.(*OIDValue)) {
			break
		}
	}
}

// GetAll returns all OIDs (for debugging/inspection)
func (odb *OIDDatabase) GetAll() map[string]*OIDValue {
	odb.mu.RLock()
	defer odb.mu.RUnlock()

	result := make(map[string]*OIDValue)
	for _, oid := range odb.sortedOIDs {
		val, _ := odb.tree.Get(oid)
		result[oid] = val.(*OIDValue)
	}
	return result
}

// SortOIDs sorts all OIDs for efficient traversal
func (odb *OIDDatabase) SortOIDs() {
	odb.mu.Lock()
	defer odb.mu.Unlock()

	// Quick sort of OIDs
	quickSortOIDs(odb.sortedOIDs, 0, len(odb.sortedOIDs)-1)
}

// isOIDLess compares two OIDs lexicographically
// OID format: 1.3.6.1.2.1.1.1.0 (dotted decimal notation)
func isOIDLess(oid1, oid2 string) bool {
	parts1 := strings.Split(oid1, ".")
	parts2 := strings.Split(oid2, ".")

	minLen := len(parts1)
	if len(parts2) < minLen {
		minLen = len(parts2)
	}

	// Compare numeric components
	for i := 0; i < minLen; i++ {
		var num1, num2 int
		_, _ = fmt.Sscanf(parts1[i], "%d", &num1)
		_, _ = fmt.Sscanf(parts2[i], "%d", &num2)

		if num1 != num2 {
			return num1 < num2
		}
	}

	// If all compared parts are equal, shorter OID is less
	return len(parts1) < len(parts2)
}

// quickSortOIDs sorts OID array in-place
func quickSortOIDs(oids []string, low, high int) {
	if low < high {
		partIdx := partitionOIDs(oids, low, high)
		quickSortOIDs(oids, low, partIdx-1)
		quickSortOIDs(oids, partIdx+1, high)
	}
}

// partitionOIDs partitions OID array for quicksort
func partitionOIDs(oids []string, low, high int) int {
	pivot := oids[high]
	i := low - 1

	for j := low; j < high; j++ {
		if isOIDLess(oids[j], pivot) {
			i++
			oids[i], oids[j] = oids[j], oids[i]
		}
	}
	oids[i+1], oids[high] = oids[high], oids[i+1]
	return i + 1
}

// LoadOIDDatabase creates and loads a database from various sources
func LoadOIDDatabase(snmprecFile string) (*OIDDatabase, error) {
	db := NewOIDDatabase()

	// Load from .snmprec file if provided
	if snmprecFile != "" {
		count, err := LoadSNMPrecFile(db, snmprecFile)
		if err != nil {
			log.Printf("Warning: Could not load .snmprec file: %v", err)
		} else {
			log.Printf("Loaded %d OIDs from %s", count, snmprecFile)
		}
	}

	// Load default OID templates
	loadDefaultOIDs(db)

	// Sort OIDs for efficient GetNext operations
	db.SortOIDs()

	return db, nil
}

// loadDefaultOIDs loads a default set of system OIDs
func loadDefaultOIDs(db *OIDDatabase) {
	defaults := map[string]*OIDValue{
		// System group
		"1.3.6.1.2.1.1.1.0":     {Type: gosnmp.OctetString, Value: "Simulated SNMP Agent"},
		"1.3.6.1.2.1.1.2.0":     {Type: gosnmp.ObjectIdentifier, Value: "1.3.6.1.4.1.9.9.46.1"},
		"1.3.6.1.2.1.1.4.0":     {Type: gosnmp.OctetString, Value: "admin@example.com"},
		"1.3.6.1.2.1.1.7.0":     {Type: gosnmp.Integer, Value: 72},
		"1.3.6.1.2.1.1.8.0":     {Type: gosnmp.TimeTicks, Value: 0},
		"1.3.6.1.2.1.1.9.1.2.1": {Type: gosnmp.ObjectIdentifier, Value: "1.3.6.1.6.3.1.1.4.1.0"},

		// Interfaces group
		"1.3.6.1.2.1.2.1.0":      {Type: gosnmp.Integer, Value: 2},
		"1.3.6.1.2.1.2.2.1.1.1":  {Type: gosnmp.Integer, Value: 1},
		"1.3.6.1.2.1.2.2.1.2.1":  {Type: gosnmp.OctetString, Value: "eth0"},
		"1.3.6.1.2.1.2.2.1.3.1":  {Type: gosnmp.Integer, Value: 6},
		"1.3.6.1.2.1.2.2.1.4.1":  {Type: gosnmp.Integer, Value: 1500},
		"1.3.6.1.2.1.2.2.1.5.1":  {Type: gosnmp.Integer, Value: 1000000000},
		"1.3.6.1.2.1.2.2.1.10.1": {Type: gosnmp.Counter32, Value: uint32(1000000)},

		// IP group
		"1.3.6.1.2.1.4.1.0":                {Type: gosnmp.Integer, Value: 1},
		"1.3.6.1.2.1.4.20.1.1.192.168.1.1": {Type: gosnmp.OctetString, Value: "192.168.1.1"},
		"1.3.6.1.2.1.4.20.1.2.192.168.1.1": {Type: gosnmp.Integer, Value: 1},
		"1.3.6.1.2.1.4.20.1.3.192.168.1.1": {Type: gosnmp.OctetString, Value: "255.255.255.0"},

		// TCP group
		"1.3.6.1.2.1.6.1.0":  {Type: gosnmp.Integer, Value: 100},
		"1.3.6.1.2.1.6.14.0": {Type: gosnmp.Counter32, Value: uint32(50000)},

		// UDP group
		"1.3.6.1.2.1.7.1.0": {Type: gosnmp.Counter32, Value: uint32(100000)},
		"1.3.6.1.2.1.7.2.0": {Type: gosnmp.Counter32, Value: uint32(50000)},

		// SNMP group
		"1.3.6.1.2.1.11.1.0": {Type: gosnmp.Counter32, Value: uint32(1000)},
		"1.3.6.1.2.1.11.3.0": {Type: gosnmp.Counter32, Value: uint32(0)},
		"1.3.6.1.2.1.11.4.0": {Type: gosnmp.Counter32, Value: uint32(0)},
		"1.3.6.1.2.1.11.6.0": {Type: gosnmp.Counter32, Value: uint32(100)},
	}

	for oid, value := range defaults {
		db.Insert(oid, value)
	}

	log.Printf("Loaded %d default OIDs", len(defaults))
}
