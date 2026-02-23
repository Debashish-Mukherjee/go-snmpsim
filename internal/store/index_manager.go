package store

import (
	"fmt"
	"log"
	"sort"
	"sync"
)

// OIDIndexManager manages OID indexing and table traversal for Zabbix LLD
// Optimized for <100ms response times even with 1000+ devices
type OIDIndexManager struct {
	// Tables detected from OID database
	tables map[string]*SNMPTable

	// Pre-built sorted OID list for fast GetNext traversal
	sortedOIDs []string

	// Quick lookup: OID -> index in sortedOIDs (for binary search)
	oidToIndex map[string]int

	// Cache of table OID prefixes for quick lookup
	tableOIDs map[string]bool

	// Lock for thread-safe access
	mu sync.RWMutex

	// Statistics
	totalOIDs    int
	totalTables  int
	lastRebuild  int64
	rebuildCount int
}

// NewOIDIndexManager creates a new index manager
func NewOIDIndexManager() *OIDIndexManager {
	return &OIDIndexManager{
		tables:     make(map[string]*SNMPTable),
		sortedOIDs: make([]string, 0),
		oidToIndex: make(map[string]int),
		tableOIDs:  make(map[string]bool),
	}
}

// BuildIndex builds the complete index from OID database
// This is called once during startup and whenever OIDs are added
func (im *OIDIndexManager) BuildIndex(db *OIDDatabase) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// Collect all OIDs from database
	allOIDs := make([]*OIDEntry, 0)
	db.Walk(func(oid string, value *OIDValue) bool {
		allOIDs = append(allOIDs, &OIDEntry{
			OID:   oid,
			Type:  value.Type,
			Value: value.Value,
		})
		return true
	})

	// Detect table structures
	im.tables = DetectTableStructure(allOIDs)

	// Mark all table OID prefixes for quick lookup
	for entryOID := range im.tables {
		baseOID := ExtractTableBase(entryOID)
		im.tableOIDs[baseOID] = true
		im.tableOIDs[entryOID] = true
	}

	// Build sorted OID list for all non-table OIDs
	nonTableOIDs := make([]string, 0)
	for _, entry := range allOIDs {
		if !IsTableEntry(entry.OID) {
			nonTableOIDs = append(nonTableOIDs, entry.OID)
		}
	}

	// Combine: sorted non-table OIDs + table entries
	im.sortedOIDs = nonTableOIDs
	sort.Slice(im.sortedOIDs, func(i, j int) bool {
		return isOIDLess(im.sortedOIDs[i], im.sortedOIDs[j])
	})

	// Add table entries in deterministic OID order.
	tableEntryOIDs := make([]string, 0, len(im.tables))
	for entryOID := range im.tables {
		tableEntryOIDs = append(tableEntryOIDs, entryOID)
	}
	sort.Slice(tableEntryOIDs, func(i, j int) bool {
		return isOIDLess(tableEntryOIDs[i], tableEntryOIDs[j])
	})

	for _, entryOID := range tableEntryOIDs {
		table := im.tables[entryOID]
		tableOIDs := im.buildTableOIDList(table)
		im.sortedOIDs = append(im.sortedOIDs, tableOIDs...)
	}

	// Build index map
	im.oidToIndex = make(map[string]int)
	for i, oid := range im.sortedOIDs {
		im.oidToIndex[oid] = i
	}

	// Update statistics
	im.totalOIDs = len(im.sortedOIDs)
	im.totalTables = len(im.tables)
	im.rebuildCount++

	log.Printf("Index rebuilt: %d OIDs, %d tables detected", im.totalOIDs, im.totalTables)
	if im.totalTables > 0 {
		stats := GetTableStats(im.tables)
		log.Printf("Table statistics: %d total rows, %d total cells",
			stats.TotalRows, stats.TotalCells)
	}

	return nil
}

// GetNext returns the next OID in sequence (for GetNext operations)
// This is the critical path for Zabbix LLD - must be <5ms
func (im *OIDIndexManager) GetNext(oid string, db *OIDDatabase) (string, *OIDValue) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// Check if this is a table OID
	if im.isTableOID(oid) {
		return im.getNextTableOID(oid, db)
	}

	// Binary search for position
	idx := searchOIDPosition(im.sortedOIDs, oid)

	// If exact match, return next
	if idx < len(im.sortedOIDs) && im.sortedOIDs[idx] == oid {
		idx++
	}

	// Return next OID if exists
	if idx < len(im.sortedOIDs) {
		nextOID := im.sortedOIDs[idx]
		value := db.Get(nextOID)
		if value != nil {
			return nextOID, value
		}
		// Fallback: scan forward to find valid OID
		for i := idx + 1; i < len(im.sortedOIDs); i++ {
			if val := db.Get(im.sortedOIDs[i]); val != nil {
				return im.sortedOIDs[i], val
			}
		}
	}

	// End of MIB
	return "", &OIDValue{
		Type:  2, // EndOfMibView
		Value: nil,
	}
}

// GetNextBulk returns multiple next OIDs for GetBulk operations
// maxRepeaters: Zabbix default is 10, max is typically 128
// Returns: list of (OID, value) pairs, up to maxRepeaters items
func (im *OIDIndexManager) GetNextBulk(oid string, maxRepeaters int, db *OIDDatabase) []*getNextBulkResult {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// Limit repeaters to reasonable max (Zabbix limit)
	if maxRepeaters > 128 {
		maxRepeaters = 128
	}
	if maxRepeaters < 1 {
		maxRepeaters = 1
	}

	results := make([]*getNextBulkResult, 0, maxRepeaters)

	// For table OIDs, use table-aware traversal
	if im.isTableOID(oid) {
		return im.getNextBulkTable(oid, maxRepeaters, db)
	}

	// Binary search for starting position
	idx := searchOIDPosition(im.sortedOIDs, oid)
	if idx < len(im.sortedOIDs) && im.sortedOIDs[idx] == oid {
		idx++
	}

	// Collect next maxRepeaters OIDs
	count := 0
	for i := idx; i < len(im.sortedOIDs) && count < maxRepeaters; i++ {
		nextOID := im.sortedOIDs[i]
		if value := db.Get(nextOID); value != nil {
			results = append(results, &getNextBulkResult{
				OID:   nextOID,
				Value: value,
			})
			count++
		}
	}

	return results
}

// getNextTableOID handles GetNext for table OIDs
// Performance critical: must use pre-sorted table structure
func (im *OIDIndexManager) getNextTableOID(oid string, db *OIDDatabase) (string, *OIDValue) {
	// Parse the table OID
	entryOID, colIndex, rowIndex, err := ParseTableOID(oid)
	if err != nil {
		// oid might be the entry OID itself (e.g. "1.3.6.1.2.1.2.2.1") or the table
		// base (e.g. "1.3.6.1.2.1.2.2") — not a row entry. Find first cell of the table.
		entryKey := oid
		if _, ok := im.tables[entryKey]; !ok {
			entryKey = oid + ".1"
		}
		if table, ok := im.tables[entryKey]; ok {
			oidStr, _, _, val, found := table.GetFirstValue()
			if found && val != nil {
				valObj := db.Get(oidStr)
				if valObj == nil {
					valObj = &OIDValue{Type: 0, Value: val}
				}
				return oidStr, valObj
			}
			return im.getOIDAfterTable(entryKey, db)
		}
		// Not a valid table OID, return next non-table OID
		idx := searchOIDPosition(im.sortedOIDs, oid)
		if idx < len(im.sortedOIDs) && im.sortedOIDs[idx] == oid {
			idx++
		}
		if idx < len(im.sortedOIDs) {
			if val := db.Get(im.sortedOIDs[idx]); val != nil {
				return im.sortedOIDs[idx], val
			}
		}
		return "", nil
	}

	table, ok := im.tables[entryOID]
	if !ok {
		return "", nil
	}

	// Use table structure for efficient traversal
	nextOID, val, found := table.GetNextValue(colIndex, rowIndex)
	if found && val != nil {
		return nextOID, &OIDValue{
			Type:  0, // Will be determined from value
			Value: val,
		}
	}

	// Table exhausted, find next OID after this table
	return im.getOIDAfterTable(entryOID, db)
}

// getNextBulkTable handles GetBulk for table OIDs
func (im *OIDIndexManager) getNextBulkTable(baseOID string, maxRepeaters int, db *OIDDatabase) []*getNextBulkResult {
	results := make([]*getNextBulkResult, 0, maxRepeaters)

	// Find which table this OID belongs to
	entryOID, colIndex, rowIndex, err := ParseTableOID(baseOID)
	if err != nil {
		// Fall back to regular bulk
		return im.getNextBulkRegular(baseOID, maxRepeaters, db)
	}

	table, ok := im.tables[entryOID]
	if !ok {
		return im.getNextBulkRegular(baseOID, maxRepeaters, db)
	}

	// Column-wise traversal (efficient for Zabbix)
	// Zabbix LLD prefers column-at-a-time for table discovery
	cols := make([]int, 0, len(table.Columns))
	for col := range table.Columns {
		cols = append(cols, col)
	}
	sort.Ints(cols)

	// Find current column
	var startCol int
	colFound := false
	for _, c := range cols {
		if c >= colIndex {
			startCol = c
			colFound = true
			break
		}
	}
	if !colFound {
		startCol = cols[len(cols)-1]
	}

	// Traverse column-by-column first (Zabbix optimization)
	for _, col := range cols {
		if col < startCol && col != colIndex {
			continue
		}

		if col == colIndex {
			// Current column: continue from rowIndex
			nextRow, val, found := table.GetNextRowForColumn(col, rowIndex)
			if found && val != nil {
				oid := fmt.Sprintf("%s.%d.%s", entryOID, col, nextRow)
				results = append(results, &getNextBulkResult{
					OID: oid,
					Value: &OIDValue{
						Type:  0,
						Value: val,
					},
				})
				if len(results) >= maxRepeaters {
					return results
				}
			}
		} else if col > colIndex {
			// Next columns: start from first row
			if len(table.SortedRowIDs) > 0 {
				rowID := table.SortedRowIDs[0]
				if val, ok, _ := table.GetValue(col, rowID); ok && val != nil {
					oid := fmt.Sprintf("%s.%d.%s", entryOID, col, rowID)
					results = append(results, &getNextBulkResult{
						OID: oid,
						Value: &OIDValue{
							Type:  0,
							Value: val,
						},
					})
					if len(results) >= maxRepeaters {
						return results
					}
				}
			}
		}
	}

	return results
}

// getNextBulkRegular handles GetBulk for non-table OIDs
func (im *OIDIndexManager) getNextBulkRegular(baseOID string, maxRepeaters int, db *OIDDatabase) []*getNextBulkResult {
	results := make([]*getNextBulkResult, 0, maxRepeaters)

	idx := searchOIDPosition(im.sortedOIDs, baseOID)
	if idx < len(im.sortedOIDs) && im.sortedOIDs[idx] == baseOID {
		idx++
	}

	count := 0
	for i := idx; i < len(im.sortedOIDs) && count < maxRepeaters; i++ {
		nextOID := im.sortedOIDs[i]
		if value := db.Get(nextOID); value != nil {
			results = append(results, &getNextBulkResult{
				OID:   nextOID,
				Value: value,
			})
			count++
		}
	}

	return results
}

// getOIDAfterTable finds the next OID after table exhaustion
func (im *OIDIndexManager) getOIDAfterTable(entryOID string, db *OIDDatabase) (string, *OIDValue) {
	// Find this table's position in sorted OIDs
	table := im.tables[entryOID]
	if table == nil {
		return "", nil
	}

	// Get last OID in this table
	lastTableOID := im.lastTableOID(table)

	// Find next OID after table
	idx := searchOIDPosition(im.sortedOIDs, lastTableOID)
	if idx < len(im.sortedOIDs) && im.sortedOIDs[idx] == lastTableOID {
		idx++
	}

	for i := idx; i < len(im.sortedOIDs); i++ {
		if value := db.Get(im.sortedOIDs[i]); value != nil {
			return im.sortedOIDs[i], value
		}
	}

	return "", nil
}

// isTableOID checks if an OID references a table component
func (im *OIDIndexManager) isTableOID(oid string) bool {
	if im.tableOIDs[oid] {
		return true
	}
	for i := len(oid) - 1; i >= 0; i-- {
		if oid[i] != '.' {
			continue
		}
		if im.tableOIDs[oid[:i]] {
			return true
		}
	}
	return false
}

func searchOIDPosition(sortedOIDs []string, target string) int {
	return sort.Search(len(sortedOIDs), func(i int) bool {
		return !isOIDLess(sortedOIDs[i], target)
	})
}

// buildTableOIDList builds an ordered list of all OIDs in a table
func (im *OIDIndexManager) buildTableOIDList(table *SNMPTable) []string {
	oids := make([]string, 0)

	// Columns in order
	cols := make([]int, 0, len(table.Columns))
	for col := range table.Columns {
		cols = append(cols, col)
	}
	sort.Ints(cols)

	// Rows in order
	rows := table.SortedRowIDs

	// Build column-major order (Zabbix preference)
	for _, col := range cols {
		for _, row := range rows {
			oid := fmt.Sprintf("%s.%d.%s", table.EntryOID, col, row)
			oids = append(oids, oid)
		}
	}

	return oids
}

// lastTableOID returns the last OID in a table
func (im *OIDIndexManager) lastTableOID(table *SNMPTable) string {
	cols := make([]int, 0, len(table.Columns))
	for col := range table.Columns {
		cols = append(cols, col)
	}
	sort.Ints(cols)

	rows := table.SortedRowIDs
	if len(rows) > 0 && len(cols) > 0 {
		lastCol := cols[len(cols)-1]
		lastRow := rows[len(rows)-1]
		return fmt.Sprintf("%s.%d.%s", table.EntryOID, lastCol, lastRow)
	}
	return ""
}

// GetTableStructures returns all detected tables
func (im *OIDIndexManager) GetTableStructures() map[string]*SNMPTable {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.tables
}

// GetStats returns index statistics
func (im *OIDIndexManager) GetStats() map[string]interface{} {
	im.mu.RLock()
	defer im.mu.RUnlock()

	return map[string]interface{}{
		"total_oids":      im.totalOIDs,
		"total_tables":    im.totalTables,
		"rebuild_count":   im.rebuildCount,
		"sorted_oids_len": len(im.sortedOIDs),
	}
}

// getNextBulkResult represents a single result in GetBulk response
type getNextBulkResult struct {
	OID   string
	Value *OIDValue
}
