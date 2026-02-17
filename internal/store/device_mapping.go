package store

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// DeviceOIDEntry represents an OID value bound to a specific device/port
type DeviceOIDEntry struct {
	OID      string
	Port     int // 0 = default (no @port specified)
	Type     gosnmp.Asn1BER
	Value    interface{}
	DeviceID string // Optional device ID for routing
	Priority int    // Higher = more priority (port > deviceID > default)
}

// DeviceOIDMapping manages device-specific OID overrides
// Priority: port-specific (@20000) > device-specific (@device-1) > default
type DeviceOIDMapping struct {
	// oidsByPort: map[port]map[oid]entry - port-specific mappings
	oidsByPort map[int]map[string]*DeviceOIDEntry

	// oidsByDevice: map[deviceID]map[oid]entry - device-specific mappings
	oidsByDevice map[string]map[string]*DeviceOIDEntry

	// defaultOIDs: map[oid]entry - fallback for any device/port
	defaultOIDs map[string]*DeviceOIDEntry

	// allPorts: sorted list of all configured ports
	allPorts []int

	// allDeviceIDs: sorted list of all configured device IDs
	allDeviceIDs []string

	// stats for logging
	totalEntries    int
	portMappings    int
	deviceMappings  int
	defaultMappings int
}

// NewDeviceOIDMapping creates a new device mapping store
func NewDeviceOIDMapping() *DeviceOIDMapping {
	return &DeviceOIDMapping{
		oidsByPort:   make(map[int]map[string]*DeviceOIDEntry),
		oidsByDevice: make(map[string]map[string]*DeviceOIDEntry),
		defaultOIDs:  make(map[string]*DeviceOIDEntry),
		allPorts:     make([]int, 0),
		allDeviceIDs: make([]string, 0),
	}
}

// ParseDeviceOID parses extended .snmprec format with device routing
// Format variants:
//
//	OID|TYPE|VALUE              -> default (all devices/ports)
//	OID|TYPE|VALUE@20000        -> specific port 20000
//	OID|TYPE|VALUE@device-1     -> specific device ID
//
// Examples:
//
//	1.3.6.1.2.1.1.5.0|octetstring|router-1
//	1.3.6.1.2.1.1.5.0|octetstring|device-1@20000
//	1.3.6.1.2.1.1.5.0|octetstring|device-2@device-1
func ParseDeviceOID(line string) (*DeviceOIDEntry, error) {
	parts := strings.SplitN(line, "|", 4)

	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid OID entry format: %s", line)
	}

	oid := strings.TrimSpace(parts[0])
	typeStr := strings.TrimSpace(parts[1])
	valueWithRoute := strings.TrimSpace(parts[2])

	// Parse type
	snmpType, err := parseType(typeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid type for OID %s: %s", oid, typeStr)
	}

	// Initialize entry
	entry := &DeviceOIDEntry{
		OID:      oid,
		Type:     snmpType,
		Port:     0,  // default: not port-specific
		DeviceID: "", // default: not device-specific
		Priority: 0,  // default priority
	}

	// Parse value and route specification
	var value string
	var route string

	// Check if value has routing info: value@port or value@device-id
	atIndex := strings.LastIndex(valueWithRoute, "@")
	if atIndex != -1 {
		value = strings.TrimSpace(valueWithRoute[:atIndex])
		route = strings.TrimSpace(valueWithRoute[atIndex+1:])

		// Determine if route is port (numeric) or device ID (string)
		if port, err := strconv.Atoi(route); err == nil {
			// Numeric: it's a port number
			entry.Port = port
			entry.Priority = 2 // port-specific has highest priority
		} else {
			// Non-numeric: it's a device ID
			entry.DeviceID = route
			entry.Priority = 1 // device-specific has medium priority
		}
	} else {
		// No routing info: plain value
		value = valueWithRoute
		entry.Priority = 0 // default has lowest priority
	}

	// Parse value based on type
	parsedValue, err := parseMappingValue(snmpType, value)
	if err != nil {
		return nil, fmt.Errorf("failed to parse value '%s' for OID %s as %s: %v",
			value, oid, typeStr, err)
	}

	entry.Value = parsedValue
	return entry, nil
}

// AddEntry adds a device OID entry to the mapping
func (dm *DeviceOIDMapping) AddEntry(entry *DeviceOIDEntry) {
	if entry == nil {
		return
	}

	dm.totalEntries++

	if entry.Port > 0 {
		// Port-specific entry
		if dm.oidsByPort[entry.Port] == nil {
			dm.oidsByPort[entry.Port] = make(map[string]*DeviceOIDEntry)
			dm.allPorts = append(dm.allPorts, entry.Port)
		}
		dm.oidsByPort[entry.Port][entry.OID] = entry
		dm.portMappings++
	} else if entry.DeviceID != "" {
		// Device-specific entry
		if dm.oidsByDevice[entry.DeviceID] == nil {
			dm.oidsByDevice[entry.DeviceID] = make(map[string]*DeviceOIDEntry)
			dm.allDeviceIDs = append(dm.allDeviceIDs, entry.DeviceID)
		}
		dm.oidsByDevice[entry.DeviceID][entry.OID] = entry
		dm.deviceMappings++
	} else {
		// Default entry
		dm.defaultOIDs[entry.OID] = entry
		dm.defaultMappings++
	}
}

// GetOID retrieves the best matching OID value for a device/port
// Priority: port-specific > device-specific > default
func (dm *DeviceOIDMapping) GetOID(oid string, port int, deviceID string) *OIDValue {
	// Try port-specific first (highest priority)
	if portMap, ok := dm.oidsByPort[port]; ok {
		if entry, ok := portMap[oid]; ok {
			return &OIDValue{
				Type:  entry.Type,
				Value: entry.Value,
			}
		}
	}

	// Try device-specific second (medium priority)
	if deviceID != "" {
		if devMap, ok := dm.oidsByDevice[deviceID]; ok {
			if entry, ok := devMap[oid]; ok {
				return &OIDValue{
					Type:  entry.Type,
					Value: entry.Value,
				}
			}
		}
	}

	// Try default last (lowest priority)
	if entry, ok := dm.defaultOIDs[oid]; ok {
		return &OIDValue{
			Type:  entry.Type,
			Value: entry.Value,
		}
	}

	// Not found
	return nil
}

// GetStats returns mapping statistics
func (dm *DeviceOIDMapping) GetStats() (total, ports, devices, defaults int) {
	return dm.totalEntries, dm.portMappings, dm.deviceMappings, dm.defaultMappings
}

// LogStats logs mapping statistics
func (dm *DeviceOIDMapping) LogStats() {
	sort.Ints(dm.allPorts)
	sort.Strings(dm.allDeviceIDs)

	log.Printf("Device mapping stats:")
	log.Printf("  Total entries: %d", dm.totalEntries)
	log.Printf("  Port-specific: %d mappings for %d ports: %v",
		dm.portMappings, len(dm.allPorts), dm.allPorts)
	log.Printf("  Device-specific: %d mappings for %d device IDs: %v",
		dm.deviceMappings, len(dm.allDeviceIDs), dm.allDeviceIDs)
	log.Printf("  Default: %d mappings", dm.defaultMappings)
}

// parseValue parses value string based on SNMP type
func parseMappingValue(snmpType gosnmp.Asn1BER, value string) (interface{}, error) {
	switch snmpType {
	case gosnmp.OctetString:
		return value, nil

	case gosnmp.Integer:
		v, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return nil, err
		}
		return int(v), nil

	case gosnmp.Counter32:
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nil, err
		}
		return uint32(v), nil

	case gosnmp.Counter64:
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, err
		}
		return v, nil

	case gosnmp.Gauge32:
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nil, err
		}
		return uint32(v), nil

	case gosnmp.TimeTicks:
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nil, err
		}
		return uint32(v), nil

	case gosnmp.Opaque:
		return value, nil

	case gosnmp.ObjectIdentifier:
		return value, nil

	case gosnmp.IPAddress:
		return value, nil

	default:
		return value, nil
	}
}

// parseType converts type string to SNMP type
func parseType(typeStr string) (gosnmp.Asn1BER, error) {
	switch strings.ToLower(typeStr) {
	case "octetstring", "string":
		return gosnmp.OctetString, nil
	case "integer", "int":
		return gosnmp.Integer, nil
	case "counter32", "counter":
		return gosnmp.Counter32, nil
	case "counter64":
		return gosnmp.Counter64, nil
	case "gauge32", "gauge":
		return gosnmp.Gauge32, nil
	case "timeticks", "timetick":
		return gosnmp.TimeTicks, nil
	case "integer32", "int32":
		return gosnmp.Integer, nil
	case "opaque":
		return gosnmp.Opaque, nil
	case "oid", "objectidentifier":
		return gosnmp.ObjectIdentifier, nil
	case "ipaddress", "ipaddr":
		return gosnmp.IPAddress, nil
	default:
		return gosnmp.OctetString, fmt.Errorf("unknown type: %s", typeStr)
	}
}

// IsDeviceOID checks if a line contains device routing (@port or @device-id)
func IsDeviceOID(line string) bool {
	// Check if line has | separator and @ indicator
	if !strings.Contains(line, "|") {
		return false
	}

	// Look for @ after the value field (third |)
	parts := strings.SplitN(line, "|", 4)
	if len(parts) < 3 {
		return false
	}

	valueField := strings.TrimSpace(parts[2])
	return strings.Contains(valueField, "@")
}

// CollectDeviceMappings extracts device-specific OID entries from lines
// Returns: entries, regular OIDs, error
func CollectDeviceMappings(lines []string) ([]*DeviceOIDEntry, []*OIDEntry, error) {
	deviceEntries := make([]*DeviceOIDEntry, 0)
	regularEntries := make([]*OIDEntry, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if it's a device mapping
		if IsDeviceOID(line) {
			entry, err := ParseDeviceOID(line)
			if err != nil {
				return nil, nil, err
			}
			deviceEntries = append(deviceEntries, entry)
		} else {
			// Try parsing as regular OID
			entry, err := ParseOIDEntry(line)
			if err != nil {
				return nil, nil, err
			}
			regularEntries = append(regularEntries, entry)
		}
	}

	return deviceEntries, regularEntries, nil
}

// ParseOIDEntry parses a regular OID entry (without device routing)
// Format: OID|TYPE|VALUE
func ParseOIDEntry(line string) (*OIDEntry, error) {
	parts := strings.SplitN(line, "|", 3)

	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid OID entry format: %s", line)
	}

	oid := strings.TrimSpace(parts[0])
	typeStr := strings.TrimSpace(parts[1])
	value := strings.TrimSpace(parts[2])

	snmpType, err := parseType(typeStr)
	if err != nil {
		return nil, err
	}

	parsedValue, err := parseMappingValue(snmpType, value)
	if err != nil {
		return nil, err
	}

	return &OIDEntry{
		OID:   oid,
		Type:  snmpType,
		Value: parsedValue,
	}, nil
}
