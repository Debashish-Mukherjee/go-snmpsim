package store

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// LoadSNMPrecFile loads OIDs from a .snmprec, snmpwalk, or text file
// Automatically detects format: snmprec (OID|TYPE|VALUE), snmpwalk named (MIB::), or snmpwalk numeric (.1.3...)
// Also supports template syntax: OID|TYPE|VALUE|#1-48 for range expansion
func LoadSNMPrecFile(db *OIDDatabase, filePath string) (int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	dataStr := string(data)

	// Check if this is snmpwalk format (named or numeric) or .snmprec with possible templates
	// Snmpwalk formats have " = " separators, .snmprec has "|" separators
	isSnmpwalk := strings.Contains(dataStr, " = ") && !strings.Contains(dataStr, "|")

	var count int

	if isSnmpwalk {
		// Parse as snmpwalk output (named or numeric format)
		parsedDB, err := ParseSnmpwalkOutput(data)
		if err != nil {
			return 0, fmt.Errorf("failed to parse snmpwalk output: %w", err)
		}

		// Merge parsed OIDs into provided database
		parsedDB.Walk(func(oid string, value *OIDValue) bool {
			db.Insert(oid, value)
			count++
			return true
		})
	} else {
		// Parse as .snmprec format with potential templates
		count, err = loadSnmprec(db, dataStr)
		if err != nil {
			return 0, err
		}
	}

	return count, nil
}

// loadSnmprec parses .snmprec format with template and device mapping support
// Format: OID|TYPE|VALUE or OID|TYPE|VALUE|#RANGE or OID|TYPE|VALUE@PORT
func loadSnmprec(db *OIDDatabase, content string) (int, error) {
	lines := strings.Split(content, "\n")

	// First pass: collect templates and regular entries
	templates, regularEntries, err := CollectTemplates(lines)
	if err != nil {
		return 0, err
	}

	// Load regular entries into database
	count := 0
	for _, entry := range regularEntries {
		db.Insert(entry.OID, &OIDValue{
			Type:  entry.Type,
			Value: entry.Value,
		})
		count++
	}

	// Detect indices from loaded entries
	indices := DetectIndicesFromOIDs(regularEntries)
	log.Printf("Detected %d unique indices from loaded OIDs: %v", len(indices), indices)

	// Expand templates using detected indices
	if len(templates) > 0 {
		expanded := ExpandTemplates(templates, indices)
		for _, entry := range expanded {
			db.Insert(entry.OID, &OIDValue{
				Type:  entry.Type,
				Value: entry.Value,
			})
			count++
		}

		// Log template expansion statistics
		stats := GetTemplateStats(templates, expanded, count)
		log.Printf("Template expansion: %d templates expanded to %d OIDs (%.1fx coverage)",
			stats.TotalTemplates, stats.ExpandedOIDs, stats.CoverageFactor)
	}

	return count, nil
}

// LoadDeviceMappings loads device-specific OID overrides from a .snmprec file
// Supports formats:
//
//	OID|TYPE|VALUE              -> default (all devices/ports)
//	OID|TYPE|VALUE@20000        -> specific port 20000
//	OID|TYPE|VALUE@device-1     -> specific device ID
//
// Returns a DeviceOIDMapping for use by VirtualAgent
func LoadDeviceMappings(filePath string) (*DeviceOIDMapping, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read device mapping file: %w", err)
	}

	dataStr := string(data)
	lines := strings.Split(dataStr, "\n")

	// Collect and parse device mappings
	mapping := NewDeviceOIDMapping()

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Try parsing as device mapping (has @port or @device-id)
		if IsDeviceOID(line) {
			entry, err := ParseDeviceOID(line)
			if err != nil {
				log.Printf("Warning: failed to parse device mapping '%s': %v", line, err)
				continue
			}
			mapping.AddEntry(entry)
		}
	}

	mapping.LogStats()
	return mapping, nil
}

// parseOIDValue converts string representation to actual value
func parseOIDValue(typeStr, valueStr string) (interface{}, error) {
	typeStr = strings.ToLower(strings.TrimSpace(typeStr))

	switch typeStr {
	case "integer", "int", "i":
		val, err := strconv.ParseInt(valueStr, 10, 32)
		return int(val), err

	case "counter32", "gauge32", "counter", "gauge", "c32":
		val, err := strconv.ParseInt(valueStr, 10, 32)
		return uint32(val), err

	case "counter64", "c64":
		val, err := strconv.ParseInt(valueStr, 10, 64)
		return uint64(val), err

	case "timeticks", "tt", "ticks":
		val, err := strconv.ParseInt(valueStr, 10, 32)
		return uint32(val), err

	case "octetstring", "string", "s":
		return valueStr, nil

	case "objectidentifier", "oid", "o":
		return valueStr, nil

	case "ipaddress", "ip":
		return valueStr, nil

	case "opaque":
		return valueStr, nil

	case "nsapaddress":
		return valueStr, nil

	case "bits":
		return valueStr, nil

	default:
		return nil, fmt.Errorf("unknown type: %s", typeStr)
	}
}

// getSNMPType returns the appropriate gosnmp type for a type string
func getSNMPType(typeStr string) gosnmp.Asn1BER {
	typeStr = strings.ToLower(strings.TrimSpace(typeStr))

	switch typeStr {
	case "integer", "int", "i":
		return gosnmp.Integer
	case "counter32", "counter", "c32":
		return gosnmp.Counter32
	case "counter64", "c64":
		return gosnmp.Counter64
	case "gauge32", "gauge", "g":
		return gosnmp.Gauge32
	case "timeticks", "tt", "ticks":
		return gosnmp.TimeTicks
	case "octetstring", "string", "s":
		return gosnmp.OctetString
	case "objectidentifier", "oid", "o":
		return gosnmp.ObjectIdentifier
	case "ipaddress", "ip":
		return gosnmp.IPAddress
	case "opaque":
		return gosnmp.Opaque
	case "nsapaddress":
		return gosnmp.NsapAddress
	case "bits":
		return gosnmp.BitString
	default:
		return gosnmp.OctetString
	}
}

// GenerateDefaultSNMPrecFile creates a default .snmprec file
func GenerateDefaultSNMPrecFile(filePath string) error {
	content := `# SNMP Simulator Record File (.snmprec)
# Format: OID|TYPE|VALUE
# Supported types: integer, counter32, gauge32, timeticks, octetstring, objectidentifier, ipaddress

# System group (1.3.6.1.2.1.1)
1.3.6.1.2.1.1.1.0|octetstring|Simulated SNMP Device
1.3.6.1.2.1.1.2.0|objectidentifier|1.3.6.1.4.1.9.9.46.1
1.3.6.1.2.1.1.3.0|timeticks|0
1.3.6.1.2.1.1.4.0|octetstring|admin@example.com
1.3.6.1.2.1.1.5.0|octetstring|device-simulator
1.3.6.1.2.1.1.6.0|octetstring|Virtual Lab
1.3.6.1.2.1.1.7.0|integer|72
1.3.6.1.2.1.1.8.0|timeticks|100

# Interfaces group (1.3.6.1.2.1.2)
1.3.6.1.2.1.2.1.0|integer|3
1.3.6.1.2.1.2.2.1.1.1|integer|1
1.3.6.1.2.1.2.2.1.2.1|octetstring|eth0
1.3.6.1.2.1.2.2.1.3.1|integer|6
1.3.6.1.2.1.2.2.1.4.1|integer|1500
1.3.6.1.2.1.2.2.1.5.1|integer|1000000000
1.3.6.1.2.1.2.2.1.8.1|integer|1
1.3.6.1.2.1.2.2.1.10.1|counter32|1000000

1.3.6.1.2.1.2.2.1.1.2|integer|2
1.3.6.1.2.1.2.2.1.2.2|octetstring|eth1
1.3.6.1.2.1.2.2.1.3.2|integer|6
1.3.6.1.2.1.2.2.1.4.2|integer|1500
1.3.6.1.2.1.2.2.1.5.2|integer|1000000000
1.3.6.1.2.1.2.2.1.10.2|counter32|2000000

1.3.6.1.2.1.2.2.1.1.3|integer|3
1.3.6.1.2.1.2.2.1.2.3|octetstring|eth2
1.3.6.1.2.1.2.2.1.3.3|integer|6
1.3.6.1.2.1.2.2.1.4.3|integer|1500
1.3.6.1.2.1.2.2.1.5.3|integer|1000000000
1.3.6.1.2.1.2.2.1.10.3|counter32|1500000

# IP group (1.3.6.1.2.1.4)
1.3.6.1.2.1.4.1.0|integer|1
1.3.6.1.2.1.4.20.1.1.192.168.1.1|ipaddress|192.168.1.1
1.3.6.1.2.1.4.20.1.2.192.168.1.1|integer|1
1.3.6.1.2.1.4.20.1.3.192.168.1.1|octetstring|255.255.255.0
1.3.6.1.2.1.4.20.1.4.192.168.1.1|integer|1

# TCP group (1.3.6.1.2.1.6)
1.3.6.1.2.1.6.1.0|integer|100
1.3.6.1.2.1.6.9.0|integer|5
1.3.6.1.2.1.6.10.0|counter32|1000
1.3.6.1.2.1.6.11.0|counter32|50
1.3.6.1.2.1.6.12.0|counter32|10
1.3.6.1.2.1.6.13.0|counter32|5
1.3.6.1.2.1.6.14.0|counter32|50000
1.3.6.1.2.1.6.15.0|counter32|100

# UDP group (1.3.6.1.2.1.7)
1.3.6.1.2.1.7.1.0|counter32|100000
1.3.6.1.2.1.7.2.0|counter32|50000
1.3.6.1.2.1.7.3.0|counter32|10
1.3.6.1.2.1.7.4.0|counter32|5

# SNMP group (1.3.6.1.2.1.11)
1.3.6.1.2.1.11.1.0|counter32|1000
1.3.6.1.2.1.11.2.0|counter32|100
1.3.6.1.2.1.11.3.0|counter32|0
1.3.6.1.2.1.11.4.0|counter32|0
1.3.6.1.2.1.11.5.0|counter32|50
1.3.6.1.2.1.11.6.0|counter32|100
1.3.6.1.2.1.11.30.0|counter32|5
`

	return os.WriteFile(filePath, []byte(content), 0644)
}
