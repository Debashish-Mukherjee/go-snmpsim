package store

import (
	"fmt"
	"log"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// SnmpwalkFormat represents detected input format
type SnmpwalkFormat int

const (
	FormatUnknown      SnmpwalkFormat = iota
	FormatNamedWithMIB                // SNMPv2-MIB::sysDescr.0 = ...
	FormatNumeric                     // .1.3.6.1.2.1.1.1.0 = ...
	FormatSnmprec                     // 1.3.6.1.2.1.1.1.0|octetstring|...
)

// OIDEntry represents a parsed OID record
type OIDEntry struct {
	OID   string
	Type  gosnmp.Asn1BER
	Value interface{}
}

// ParseSnmpwalkOutput detects format and parses snmpwalk output
// Returns OIDDatabase populated with discovered OIDs
func ParseSnmpwalkOutput(data []byte) (*OIDDatabase, error) {
	dataStr := string(data)

	// 1. Detect format
	format := detectFormat(dataStr)

	// 2. Parse based on format
	var oidEntries []*OIDEntry
	var err error

	switch format {
	case FormatNamedWithMIB:
		oidEntries, err = parseNamedFormat(dataStr)
	case FormatNumeric:
		oidEntries, err = parseNumericFormat(dataStr)
	case FormatSnmprec:
		// Treat as normal .snmprec
		oidEntries, err = parseSnmprec(dataStr)
	default:
		return nil, fmt.Errorf("unknown format")
	}

	if err != nil {
		return nil, err
	}

	// 3. Build OIDDatabase from entries
	db := NewOIDDatabase()
	for _, entry := range oidEntries {
		db.Insert(entry.OID, &OIDValue{
			Type:  entry.Type,
			Value: entry.Value,
		})
	}

	return db, nil
}

// detectFormat identifies input format
func detectFormat(data string) SnmpwalkFormat {
	// Check for named format: "SNMPv2-MIB::" or similar
	if strings.Contains(data, "SNMPv2-MIB::") ||
		strings.Contains(data, "SNMPv2-SMI::") ||
		strings.Contains(data, "SNMPv2-CONF::") ||
		strings.Contains(data, "-MIB::") {
		return FormatNamedWithMIB
	}

	// Check for numeric format: starts with "." on first non-empty line
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, ".") {
			return FormatNumeric
		}
		// If we find a line starting with a digit, check if it's .snmprec
		if strings.Contains(trimmed, "|") {
			return FormatSnmprec
		}
		// First non-empty line determines format
		break
	}

	// Default fallback: try numeric
	if strings.Count(data, ".") > strings.Count(data, "::") {
		return FormatNumeric
	}

	return FormatUnknown
}

// parseNamedFormat parses "SNMPv2-MIB::sysDescr.0 = STRING ..." format
func parseNamedFormat(data string) ([]*OIDEntry, error) {
	var entries []*OIDEntry

	lines := strings.Split(data, "\n")
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entry, err := parseNamedLine(line)
		if err != nil {
			log.Printf("Warning: Failed to parse named format line %d: %v", lineNum+1, err)
			continue
		}

		if entry != nil {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// parseNamedLine extracts OID, type, value from named format line
// Input: "SNMPv2-MIB::sysDescr.0 = STRING "Linux device""
// Input: "SNMPv2-MIB::sysUpTime.0 = Timeticks: (123456789) 14:18:08.89"
func parseNamedLine(line string) (*OIDEntry, error) {
	// Split on " = "
	parts := strings.SplitN(line, " = ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid format: %s", line)
	}

	// Parse LHS: extract OID
	lhs := strings.TrimSpace(parts[0])
	oid, err := extractOIDFromNamed(lhs)
	if err != nil {
		return nil, err
	}

	// Parse RHS: extract type and value
	rhs := strings.TrimSpace(parts[1])
	snmpType, value, err := parseTypedValue(rhs)
	if err != nil {
		return nil, err
	}

	return &OIDEntry{
		OID:   oid,
		Type:  snmpType,
		Value: value,
	}, nil
}

// extractOIDFromNamed converts "SNMPv2-MIB::sysDescr.0" to "1.3.6.1.2.1.1.1.0"
func extractOIDFromNamed(named string) (string, error) {
	// Pattern: "MIBNAME::objectName[.index]"
	parts := strings.SplitN(named, "::", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid named OID: %s", named)
	}

	mibName := strings.TrimSpace(parts[0])
	objectPart := strings.TrimSpace(parts[1])

	// Lookup MIB OID
	oid := lookupMIBOID(mibName, objectPart)
	if oid == "" {
		return "", fmt.Errorf("unknown MIB object: %s::%s", mibName, objectPart)
	}

	return oid, nil
}

// lookupMIBOID maps MIB object names to OIDs
// Example: SNMPv2-MIB::sysDescr.0 -> 1.3.6.1.2.1.1.1.0
func lookupMIBOID(mibName, objectName string) string {
	// Built-in common OID mappings
	mibMapping := map[string]string{
		// System group (1.3.6.1.2.1.1)
		"sysDescr":        "1.3.6.1.2.1.1.1.0",
		"sysObjectID":     "1.3.6.1.2.1.1.2.0",
		"sysUpTime":       "1.3.6.1.2.1.1.3.0",
		"sysContact":      "1.3.6.1.2.1.1.4.0",
		"sysName":         "1.3.6.1.2.1.1.5.0",
		"sysLocation":     "1.3.6.1.2.1.1.6.0",
		"sysServices":     "1.3.6.1.2.1.1.7.0",
		"sysORLastChange": "1.3.6.1.2.1.1.8.0",

		// Interfaces group (1.3.6.1.2.1.2)
		"ifNumber":        "1.3.6.1.2.1.2.1.0",
		"ifIndex":         "1.3.6.1.2.1.2.2.1.1",
		"ifDescr":         "1.3.6.1.2.1.2.2.1.2",
		"ifType":          "1.3.6.1.2.1.2.2.1.3",
		"ifMtu":           "1.3.6.1.2.1.2.2.1.4",
		"ifSpeed":         "1.3.6.1.2.1.2.2.1.5",
		"ifPhysAddress":   "1.3.6.1.2.1.2.2.1.6",
		"ifAdminStatus":   "1.3.6.1.2.1.2.2.1.7",
		"ifOperStatus":    "1.3.6.1.2.1.2.2.1.8",
		"ifLastChange":    "1.3.6.1.2.1.2.2.1.9",
		"ifInOctets":      "1.3.6.1.2.1.2.2.1.10",
		"ifInUcastPkts":   "1.3.6.1.2.1.2.2.1.11",
		"ifInNUcastPkts":  "1.3.6.1.2.1.2.2.1.12",
		"ifInDiscards":    "1.3.6.1.2.1.2.2.1.13",
		"ifInErrors":      "1.3.6.1.2.1.2.2.1.20",
		"ifOutOctets":     "1.3.6.1.2.1.2.2.1.16",
		"ifOutUcastPkts":  "1.3.6.1.2.1.2.2.1.17",
		"ifOutNUcastPkts": "1.3.6.1.2.1.2.2.1.18",
		"ifOutDiscards":   "1.3.6.1.2.1.2.2.1.19",
		"ifOutErrors":     "1.3.6.1.2.1.2.2.1.24",
		"ifName":          "1.3.6.1.2.1.31.1.1.1.1",
		"ifHighSpeed":     "1.3.6.1.2.1.31.1.1.1.15",

		// IP group (1.3.6.1.2.1.4)
		"ipForwarding":      "1.3.6.1.2.1.4.1.0",
		"ipDefaultTTL":      "1.3.6.1.2.1.4.2.0",
		"ipInReceives":      "1.3.6.1.2.1.4.3.0",
		"ipInHdrErrors":     "1.3.6.1.2.1.4.4.0",
		"ipInAddrErrors":    "1.3.6.1.2.1.4.5.0",
		"ipForwDatagrams":   "1.3.6.1.2.1.4.6.0",
		"ipInUnknownProtos": "1.3.6.1.2.1.4.7.0",
		"ipInDiscards":      "1.3.6.1.2.1.4.8.0",
		"ipInDelivers":      "1.3.6.1.2.1.4.9.0",
		"ipOutRequests":     "1.3.6.1.2.1.4.10.0",
		"ipOutDiscards":     "1.3.6.1.2.1.4.11.0",
		"ipOutNoRoutes":     "1.3.6.1.2.1.4.12.0",
		"ipReasmTimeout":    "1.3.6.1.2.1.4.13.0",
		"ipReasmReqds":      "1.3.6.1.2.1.4.14.0",
		"ipReasmOKs":        "1.3.6.1.2.1.4.15.0",
		"ipReasmFails":      "1.3.6.1.2.1.4.16.0",
		"ipFragOKs":         "1.3.6.1.2.1.4.17.0",
		"ipFragFails":       "1.3.6.1.2.1.4.18.0",
		"ipFragCreates":     "1.3.6.1.2.1.4.19.0",

		// TCP group (1.3.6.1.2.1.6)
		"tcpRtoAlgorithm": "1.3.6.1.2.1.6.1.0",
		"tcpRtoMin":       "1.3.6.1.2.1.6.2.0",
		"tcpRtoMax":       "1.3.6.1.2.1.6.3.0",
		"tcpMaxConn":      "1.3.6.1.2.1.6.4.0",
		"tcpActiveOpens":  "1.3.6.1.2.1.6.5.0",
		"tcpPassiveOpens": "1.3.6.1.2.1.6.6.0",
		"tcpAttemptFails": "1.3.6.1.2.1.6.7.0",
		"tcpEstabResets":  "1.3.6.1.2.1.6.8.0",
		"tcpCurrEstab":    "1.3.6.1.2.1.6.9.0",
		"tcpInSegs":       "1.3.6.1.2.1.6.10.0",
		"tcpOutSegs":      "1.3.6.1.2.1.6.11.0",
		"tcpRetransSegs":  "1.3.6.1.2.1.6.12.0",

		// UDP group (1.3.6.1.2.1.7)
		"udpInDatagrams":  "1.3.6.1.2.1.7.1.0",
		"udpNoPorts":      "1.3.6.1.2.1.7.2.0",
		"udpInErrors":     "1.3.6.1.2.1.7.3.0",
		"udpOutDatagrams": "1.3.6.1.2.1.7.4.0",

		// SNMP group (1.3.6.1.2.1.11)
		"snmpInTotalRqvdPdus":    "1.3.6.1.2.1.11.1.0",
		"snmpInTotalInvalidMsgs": "1.3.6.1.2.1.11.3.0",
		"snmpInASNParseErrs":     "1.3.6.1.2.1.11.6.0",
		"snmpOutTotalReqPdus":    "1.3.6.1.2.1.11.2.0",
		"snmpOutTotalRespPdus":   "1.3.6.1.2.1.11.4.0",
		"snmpOutGenErrs":         "1.3.6.1.2.1.11.5.0",
	}

	// Extract base and index parts
	// "ifDescr.1" -> "ifDescr" + ".1"
	// "sysDescr.0" -> "sysDescr" + ".0"
	parts := strings.SplitN(objectName, ".", 2)
	baseName := parts[0]
	index := ""
	if len(parts) > 1 {
		index = "." + parts[1]
	}

	if baseOID, ok := mibMapping[baseName]; ok {
		// If MIB mapping includes .0, don't add index
		if strings.HasSuffix(baseOID, ".0") {
			return baseOID
		}
		return baseOID + index
	}

	return ""
}

// parseNumericFormat parses ".1.3.6.1.2.1.1.1.0 = STRING "value"" format
func parseNumericFormat(data string) ([]*OIDEntry, error) {
	var entries []*OIDEntry

	lines := strings.Split(data, "\n")
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entry, err := parseNumericLine(line)
		if err != nil {
			log.Printf("Warning: Failed to parse numeric format line %d: %v", lineNum+1, err)
			continue
		}

		if entry != nil {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// parseNumericLine extracts OID and value from numeric format line
// Input: ".1.3.6.1.2.1.1.1.0 = STRING "Linux device""
func parseNumericLine(line string) (*OIDEntry, error) {
	// Split on " = "
	parts := strings.SplitN(line, " = ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid format: %s", line)
	}

	// Extract OID (remove leading dot if present)
	oid := strings.TrimSpace(parts[0])
	oid = strings.TrimPrefix(oid, ".")

	// Parse RHS: extract type and value
	rhs := strings.TrimSpace(parts[1])
	snmpType, value, err := parseTypedValue(rhs)
	if err != nil {
		return nil, err
	}

	return &OIDEntry{
		OID:   oid,
		Type:  snmpType,
		Value: value,
	}, nil
}

// parseSnmprec parses standard .snmprec format "OID|TYPE|VALUE"
func parseSnmprec(data string) ([]*OIDEntry, error) {
	var entries []*OIDEntry

	lines := strings.Split(data, "\n")
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse line: OID|TYPE|VALUE
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			log.Printf("Warning: Invalid .snmprec line %d: %s", lineNum+1, line)
			continue
		}

		oid := strings.TrimSpace(parts[0])
		typeStr := strings.TrimSpace(parts[1])
		valueStr := strings.TrimSpace(parts[2])

		// Parse type and value
		value, err := parseOIDValue(typeStr, valueStr)
		if err != nil {
			log.Printf("Warning: Failed to parse OID %s on line %d: %v", oid, lineNum+1, err)
			continue
		}

		entries = append(entries, &OIDEntry{
			OID:   oid,
			Type:  getSNMPType(typeStr),
			Value: value,
		})
	}

	return entries, nil
}

// parseTypedValue extracts SNMP type and value from RHS
// Input examples:
// - "STRING "Linux device""
// - "Timeticks: (123456789) 14:18:08.89"
// - "INTEGER 1000000000"
// - "Counter32: 987654321"
// - "Hex-STRING: 00 11 22 33 44 55"
// - "OID: .1.3.6.1.4.1.9.9.46.1"
func parseTypedValue(rhs string) (gosnmp.Asn1BER, interface{}, error) {
	rhs = strings.TrimSpace(rhs)

	// Handle STRING "..."
	if strings.HasPrefix(rhs, "STRING") {
		value := extractQuotedString(rhs)
		return gosnmp.OctetString, value, nil
	}

	// Handle INTEGER ...
	if strings.HasPrefix(rhs, "INTEGER") {
		value := extractInteger(rhs)
		return gosnmp.Integer, value, nil
	}

	// Handle Timeticks: (123456) ...
	if strings.HasPrefix(rhs, "Timeticks:") {
		value := extractTimeticks(rhs)
		return gosnmp.TimeTicks, value, nil
	}

	// Handle Counter32: ...
	if strings.HasPrefix(rhs, "Counter32:") {
		value := extractCounter32(rhs)
		return gosnmp.Counter32, value, nil
	}

	// Handle Counter64: ...
	if strings.HasPrefix(rhs, "Counter64:") {
		value := extractCounter64(rhs)
		return gosnmp.Counter64, value, nil
	}

	// Handle Gauge32: ...
	if strings.HasPrefix(rhs, "Gauge32:") {
		value := extractGauge32(rhs)
		return gosnmp.Gauge32, value, nil
	}

	// Handle OID: ...
	if strings.HasPrefix(rhs, "OID:") {
		value := extractOID(rhs)
		return gosnmp.ObjectIdentifier, value, nil
	}

	// Handle Hex-STRING: ...
	if strings.HasPrefix(rhs, "Hex-STRING:") {
		value := extractHexString(rhs)
		return gosnmp.OctetString, value, nil
	}

	// Default: treat as string
	return gosnmp.OctetString, rhs, nil
}

// Helper functions to extract values from typed representations

func extractQuotedString(s string) string {
	// Extract content between quotes
	start := strings.Index(s, "\"")
	end := strings.LastIndex(s, "\"")
	if start >= 0 && end > start {
		return s[start+1 : end]
	}
	return ""
}

func extractInteger(s string) int {
	parts := strings.Fields(s)
	if len(parts) >= 2 {
		val := 0
		fmt.Sscanf(parts[1], "%d", &val)
		return val
	}
	return 0
}

func extractTimeticks(s string) uint32 {
	// Format: "Timeticks: (123456789) 14:18:08.89"
	start := strings.Index(s, "(")
	end := strings.Index(s, ")")
	if start >= 0 && end > start {
		val := 0
		fmt.Sscanf(s[start+1:end], "%d", &val)
		return uint32(val)
	}
	return 0
}

func extractCounter32(s string) uint32 {
	parts := strings.Fields(s)
	if len(parts) >= 2 {
		val := 0
		fmt.Sscanf(parts[1], "%d", &val)
		return uint32(val)
	}
	return 0
}

func extractCounter64(s string) uint64 {
	parts := strings.Fields(s)
	if len(parts) >= 2 {
		val := uint64(0)
		fmt.Sscanf(parts[1], "%d", &val)
		return val
	}
	return 0
}

func extractGauge32(s string) uint32 {
	parts := strings.Fields(s)
	if len(parts) >= 2 {
		val := 0
		fmt.Sscanf(parts[1], "%d", &val)
		return uint32(val)
	}
	return 0
}

func extractOID(s string) string {
	parts := strings.Fields(s)
	if len(parts) >= 2 {
		oid := strings.TrimPrefix(parts[1], ".")
		return oid
	}
	return ""
}

func extractHexString(s string) string {
	// Extract hex bytes: "Hex-STRING: 00 11 22 33"
	start := strings.Index(s, ":") + 1
	hexPart := strings.TrimSpace(s[start:])
	// Return as string representation
	return hexPart
}
