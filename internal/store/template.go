package store

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// TemplateType defines the type of OID template expansion
type TemplateType int

const (
	TemplateNone       TemplateType = iota
	TemplateRange                   // #1-10, #0-47  -> explicit range
	TemplateExpression              // #1-$count     -> expression-based
	TemplateAutoDetect              // Auto-detect from file indices
)

// OIDTemplate represents a template OID with expansion rules
type OIDTemplate struct {
	OID        string
	Type       gosnmp.Asn1BER
	Value      interface{}
	Pattern    *TemplatePattern
	IsTemplate bool
}

// TemplatePattern defines how to expand a template
type TemplatePattern struct {
	Type       TemplateType
	StartIndex int            // For range: start
	EndIndex   int            // For range: end
	Step       int            // For range: step (usually 1)
	Expression string         // For expression: like "$device_count"
	Variables  map[string]int // Runtime variables
}

// ParseTemplateOID parses extended .snmprec format with templates
// Format: OID|TYPE|VALUE or OID|TYPE|VALUE|#RANGE_SPEC
// Examples:
//
//	1.3.6.1.2.1.2.2.1.5|integer|1000000000|#1-48
//	1.3.6.1.2.1.2.2.1.2|octetstring|eth0|#0-47
func ParseTemplateOID(line string) (*OIDTemplate, error) {
	parts := strings.SplitN(line, "|", 4)

	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid format: %s", line)
	}

	oid := strings.TrimSpace(parts[0])
	typeStr := strings.TrimSpace(parts[1])
	value := strings.TrimSpace(parts[2])

	template := &OIDTemplate{
		OID:        oid,
		Type:       getSNMPType(typeStr),
		Value:      parseTemplateValue(typeStr, value),
		IsTemplate: false,
	}

	// Check for template specification (4th field)
	if len(parts) == 4 {
		templateSpec := strings.TrimSpace(parts[3])
		pattern, err := parseTemplatePattern(templateSpec, oid)
		if err != nil {
			return nil, err
		}
		template.Pattern = pattern
		template.IsTemplate = true
	}

	return template, nil
}

// parseTemplatePattern parses template spec like "#1-10" or "#1-$count"
func parseTemplatePattern(spec string, oid string) (*TemplatePattern, error) {
	if !strings.HasPrefix(spec, "#") {
		return nil, fmt.Errorf("invalid template spec: %s", spec)
	}

	spec = strings.TrimPrefix(spec, "#")

	// Check for expression like "1-$device_count"
	if strings.Contains(spec, "$") {
		parts := strings.SplitN(spec, "-", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid expression format: #%s", spec)
		}

		start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid start index: %s", parts[0])
		}

		return &TemplatePattern{
			Type:       TemplateExpression,
			StartIndex: start,
			Expression: strings.TrimSpace(parts[1]),
			Variables:  make(map[string]int),
		}, nil
	}

	// Parse range like "1-48" or "0-47"
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range format: #%s", spec)
	}

	start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("invalid numeric range: #%s", spec)
	}

	if start > end {
		return nil, fmt.Errorf("invalid range: start (%d) > end (%d)", start, end)
	}

	return &TemplatePattern{
		Type:       TemplateRange,
		StartIndex: start,
		EndIndex:   end,
		Step:       1,
		Variables:  make(map[string]int),
	}, nil
}

// ExpandTemplates expands all templates in database with discovered indices
func ExpandTemplates(templates []*OIDTemplate, knownIndices []int) []*OIDEntry {
	var expanded []*OIDEntry

	for _, tmpl := range templates {
		if !tmpl.IsTemplate || tmpl.Pattern == nil {
			continue
		}

		entries := expandSingleTemplate(tmpl, knownIndices)
		expanded = append(expanded, entries...)
	}

	return expanded
}

// expandSingleTemplate expands one template OID
func expandSingleTemplate(tmpl *OIDTemplate, knownIndices []int) []*OIDEntry {
	var entries []*OIDEntry

	switch tmpl.Pattern.Type {
	case TemplateRange:
		// Explicit range: #1-48
		for i := tmpl.Pattern.StartIndex; i <= tmpl.Pattern.EndIndex; i += tmpl.Pattern.Step {
			oid := fmt.Sprintf("%s.%d", tmpl.OID, i)
			entries = append(entries, &OIDEntry{
				OID:   oid,
				Type:  tmpl.Type,
				Value: tmpl.Value,
			})
		}

	case TemplateAutoDetect:
		// Use detected indices from other OIDs
		if len(knownIndices) == 0 {
			// No indices detected, skip
			break
		}
		for _, idx := range knownIndices {
			oid := fmt.Sprintf("%s.%d", tmpl.OID, idx)
			entries = append(entries, &OIDEntry{
				OID:   oid,
				Type:  tmpl.Type,
				Value: tmpl.Value,
			})
		}

	case TemplateExpression:
		// Expression-based: #1-$count
		// For now, use detected indices starting from pattern's StartIndex
		if len(knownIndices) == 0 {
			break
		}
		for _, idx := range knownIndices {
			if idx >= tmpl.Pattern.StartIndex {
				oid := fmt.Sprintf("%s.%d", tmpl.OID, idx)
				entries = append(entries, &OIDEntry{
					OID:   oid,
					Type:  tmpl.Type,
					Value: tmpl.Value,
				})
			}
		}
	}

	return entries
}

// DetectIndicesFromOIDs analyzes OID set and extracts all unique indices
// Example: [1.3.6.1.2.1.2.2.1.2.1, 1.3.6.1.2.1.2.2.1.2.2, ...] -> [1, 2, 3, ...]
func DetectIndicesFromOIDs(entries []*OIDEntry) []int {
	indicesMap := make(map[int]bool)

	for _, entry := range entries {
		parts := strings.Split(entry.OID, ".")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			if idx, err := strconv.Atoi(lastPart); err == nil {
				indicesMap[idx] = true
			}
		}
	}

	// Convert map to sorted slice
	var result []int
	for idx := range indicesMap {
		result = append(result, idx)
	}
	sort.Ints(result)
	return result
}

// parseValue parses string value based on SNMP type
func parseTemplateValue(typeStr, valueStr string) interface{} {
	typeStr = strings.ToLower(strings.TrimSpace(typeStr))

	switch typeStr {
	case "integer", "int", "i":
		val, _ := strconv.ParseInt(valueStr, 10, 32)
		return int(val)

	case "counter32", "gauge32", "counter", "gauge", "c32":
		val, _ := strconv.ParseInt(valueStr, 10, 32)
		return uint32(val)

	case "counter64", "c64":
		val, _ := strconv.ParseInt(valueStr, 10, 64)
		return uint64(val)

	case "timeticks", "tt", "ticks":
		val, _ := strconv.ParseInt(valueStr, 10, 32)
		return uint32(val)

	case "octetstring", "string", "s":
		return valueStr

	case "objectidentifier", "oid", "o":
		return valueStr

	case "ipaddress", "ip":
		return valueStr

	case "opaque":
		return valueStr

	case "bits":
		return valueStr

	default:
		return valueStr
	}
}

// IsTemplateOID checks if a line contains template syntax
func IsTemplateOID(line string) bool {
	return strings.Contains(line, "|#") ||
		(strings.Count(line, "|") >= 3 && strings.Contains(line, "#"))
}

// CollectTemplates extracts all template OIDs from entries list
func CollectTemplates(lines []string) ([]*OIDTemplate, []*OIDEntry, error) {
	var templates []*OIDTemplate
	var regularEntries []*OIDEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if this is a template
		if IsTemplateOID(line) {
			tmpl, err := ParseTemplateOID(line)
			if err != nil {
				// Log warning but continue
				// ignoring malformed templates is safer than stopping
				continue
			}
			templates = append(templates, tmpl)
		} else {
			// Parse as regular OID entry
			// Parse as standard OID|TYPE|VALUE format
			parts := strings.SplitN(line, "|", 3)
			if len(parts) < 3 {
				continue
			}

			oid := strings.TrimSpace(parts[0])
			typeStr := strings.TrimSpace(parts[1])
			valueStr := strings.TrimSpace(parts[2])

			value := parseTemplateValue(typeStr, valueStr)

			regularEntries = append(regularEntries, &OIDEntry{
				OID:   oid,
				Type:  getSNMPType(typeStr),
				Value: value,
			})
		}
	}

	return templates, regularEntries, nil
}

// TemplateStats returns statistics about template expansion
type TemplateStats struct {
	TotalTemplates  int
	ExpandedOIDs    int
	TotalOIDsLoaded int
	CoverageFactor  float64 // Ratio of expanded OIDs to original file lines
}

// GetTemplateStats calculates expansion statistics
func GetTemplateStats(templates []*OIDTemplate, expanded []*OIDEntry, totalLoaded int) TemplateStats {
	totalExpanded := 0
	for range expanded {
		totalExpanded++
	}

	coverage := 1.0
	if len(templates) > 0 {
		coverage = float64(totalExpanded) / float64(len(templates))
	}

	return TemplateStats{
		TotalTemplates:  len(templates),
		ExpandedOIDs:    totalExpanded,
		TotalOIDsLoaded: totalLoaded,
		CoverageFactor:  coverage,
	}
}
