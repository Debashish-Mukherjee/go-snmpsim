package store

import (
	"fmt"
	"sort"
	"strings"
)

// TableColumn represents a column in an SNMP table
type TableColumn struct {
	ColIndex  int    // Column index (e.g., 2 in 1.3.6.1.2.1.2.2.1.2)
	OID       string // Full OID prefix (e.g., 1.3.6.1.2.1.2.2.1.2)
	Name      string // Human-readable name (e.g., "ifDescr")
	Syntax    string // SNMP syntax type
	Access    string // "read-only", "read-write" etc
	MaxAccess string // Maximum access level
}

// TableRow represents an indexed row in a table
type TableRow struct {
	Index   string              // Row index value (e.g., "1", "2", "eth0")
	Indices []interface{}       // For multi-index tables
	Values  map[int]interface{} // column index -> value
}

// SNMPTable represents an SNMP table structure
// Tables follow the pattern: BASE.COLUMN.INDEX
// Example: 1.3.6.1.2.1.2.2 is ifTable
//
//	1.3.6.1.2.1.2.2.1 is ifEntry (conceptual row)
//	1.3.6.1.2.1.2.2.1.2.1 is ifDescr.1 (column 2, index 1)
type SNMPTable struct {
	BaseOID      string               // e.g., 1.3.6.1.2.1.2.2 (ifTable)
	EntryOID     string               // e.g., 1.3.6.1.2.1.2.2.1 (ifEntry, always ends in .1)
	Name         string               // Human-readable name (e.g., "ifTable")
	Description  string               // Table description
	Columns      map[int]*TableColumn // Column index -> column info
	Rows         map[string]*TableRow // Row index -> row data
	SortedRowIDs []string             // Pre-sorted row indices for efficient traversal
	MinRow       string               // First row ID (for GetNext optimization)
	MaxRow       string               // Last row ID (for GetNext optimization)
}

// NewSNMPTable creates a new SNMP table
func NewSNMPTable(baseOID, entryOID, name string) *SNMPTable {
	return &SNMPTable{
		BaseOID:      baseOID,
		EntryOID:     entryOID,
		Name:         name,
		Columns:      make(map[int]*TableColumn),
		Rows:         make(map[string]*TableRow),
		SortedRowIDs: make([]string, 0),
	}
}

// AddColumn adds a column definition to the table
func (t *SNMPTable) AddColumn(colIndex int, oid, name, syntax string) {
	t.Columns[colIndex] = &TableColumn{
		ColIndex: colIndex,
		OID:      oid,
		Name:     name,
		Syntax:   syntax,
	}
}

// AddRow adds a row to the table
func (t *SNMPTable) AddRow(rowIndex string, values map[int]interface{}) {
	t.Rows[rowIndex] = &TableRow{
		Index:  rowIndex,
		Values: values,
	}
	t.rebuildSortedRows()
}

// GetValue retrieves a value from the table
// Returns: value, column exists, row exists
func (t *SNMPTable) GetValue(colIndex int, rowIndex string) (interface{}, bool, bool) {
	row, rowExists := t.Rows[rowIndex]
	if !rowExists {
		return nil, false, false
	}

	_, colExists := t.Columns[colIndex]
	if !colExists {
		return nil, false, true
	}

	val, ok := row.Values[colIndex]
	return val, ok, true
}

// GetNextValue finds the next value in table traversal order
// Used by GetNext and GetBulk operations
// Returns: next OID, value, found
func (t *SNMPTable) GetNextValue(colIndex int, rowIndex string) (string, interface{}, bool) {
	// Find current position using numeric-aware search
	rowPos := searchRowIDs(t.SortedRowIDs, rowIndex)

	// Try next row in current column
	if rowPos < len(t.SortedRowIDs)-1 {
		nextRowID := t.SortedRowIDs[rowPos+1]
		if val, colExists, _ := t.GetValue(colIndex, nextRowID); colExists {
			oid := fmt.Sprintf("%s.%d.%s", t.EntryOID, colIndex, nextRowID)
			return oid, val, true
		}
	}

	// Next column, first row
	cols := make([]int, 0, len(t.Columns))
	for col := range t.Columns {
		cols = append(cols, col)
	}
	sort.Ints(cols)

	colPos := sort.SearchInts(cols, colIndex)
	if colPos < len(cols)-1 {
		nextCol := cols[colPos+1]
		if len(t.SortedRowIDs) > 0 {
			firstRow := t.SortedRowIDs[0]
			if val, ok, _ := t.GetValue(nextCol, firstRow); ok {
				oid := fmt.Sprintf("%s.%d.%s", t.EntryOID, nextCol, firstRow)
				return oid, val, true
			}
		}
	}

	// End of table
	return "", nil, false
}

// GetNextRowForColumn finds the next row in a column
// Used for efficient column traversal
func (t *SNMPTable) GetNextRowForColumn(colIndex int, currentRowIndex string) (string, interface{}, bool) {
	rowPos := searchRowIDs(t.SortedRowIDs, currentRowIndex)

	for i := rowPos + 1; i < len(t.SortedRowIDs); i++ {
		nextRow := t.SortedRowIDs[i]
		if val, ok, _ := t.GetValue(colIndex, nextRow); ok {
			return nextRow, val, true
		}
	}
	return "", nil, false
}

// GetFirstValue returns the first value in the table (for GetNext optimization)
func (t *SNMPTable) GetFirstValue() (string, int, string, interface{}, bool) {
	if len(t.SortedRowIDs) == 0 {
		return "", 0, "", nil, false
	}

	cols := make([]int, 0, len(t.Columns))
	for col := range t.Columns {
		cols = append(cols, col)
	}
	sort.Ints(cols)

	for _, col := range cols {
		row := t.SortedRowIDs[0]
		if val, ok, _ := t.GetValue(col, row); ok {
			oid := fmt.Sprintf("%s.%d.%s", t.EntryOID, col, row)
			return oid, col, row, val, true
		}
	}
	return "", 0, "", nil, false
}

// GetAllValues returns all values in a column, sorted by row
func (t *SNMPTable) GetAllValues(colIndex int) []interface{} {
	values := make([]interface{}, 0, len(t.SortedRowIDs))
	for _, rowID := range t.SortedRowIDs {
		if val, ok, _ := t.GetValue(colIndex, rowID); ok {
			values = append(values, val)
		}
	}
	return values
}

// RowCount returns the number of rows in the table
func (t *SNMPTable) RowCount() int {
	return len(t.Rows)
}

// ColumnCount returns the number of columns
func (t *SNMPTable) ColumnCount() int {
	return len(t.Columns)
}

// searchRowIDs finds the position of rowIndex in sorted SortedRowIDs using numeric OID ordering.
// If rowIndex is not found, returns the position where it would be inserted.
func searchRowIDs(sortedIDs []string, rowIndex string) int {
	return sort.Search(len(sortedIDs), func(i int) bool {
		return !isOIDLess(sortedIDs[i], rowIndex)
	})
}

// rebuildSortedRows rebuilds the sorted row index list
// Called after modifications to maintain consistent ordering
func (t *SNMPTable) rebuildSortedRows() {
	t.SortedRowIDs = make([]string, 0, len(t.Rows))
	for rowID := range t.Rows {
		t.SortedRowIDs = append(t.SortedRowIDs, rowID)
	}
	// Use numeric OID ordering so row "2" sorts before "10"
	sort.Slice(t.SortedRowIDs, func(i, j int) bool {
		return isOIDLess(t.SortedRowIDs[i], t.SortedRowIDs[j])
	})

	if len(t.SortedRowIDs) > 0 {
		t.MinRow = t.SortedRowIDs[0]
		t.MaxRow = t.SortedRowIDs[len(t.SortedRowIDs)-1]
	}
}

// ParseTableOID extracts table structure from an OID
// Pattern: 1.3.6.1.2.1.2.2.1.2.1
//
//	├─ BaseOID: 1.3.6.1.2.1.2.2
//	├─ EntryOID: 1.3.6.1.2.1.2.2.1
//	├─ ColumnIndex: 2
//	└─ RowIndex: 1
//
// Returns: entryOID, columnIndex, rowIndex, error
func ParseTableOID(oid string) (string, int, string, error) {
	// Minimum: BASE.1.COLUMN.INDEX (at least 4 components after base)
	parts := strings.Split(oid, ".")

	if len(parts) < 5 {
		return "", 0, "", fmt.Errorf("OID too short for table: %s", oid)
	}

	// Entry OID is typically BASE.1
	entryOID := strings.Join(parts[:len(parts)-2], ".")
	if len(parts) < 2 || parts[len(parts)-3] != "1" {
		return "", 0, "", fmt.Errorf("invalid table entry format: %s", oid)
	}

	// Column index is second-to-last component
	var colIndex int
	_, err := fmt.Sscanf(parts[len(parts)-2], "%d", &colIndex)
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid column index: %s", parts[len(parts)-2])
	}

	// Row index is last component
	rowIndex := parts[len(parts)-1]

	return entryOID, colIndex, rowIndex, nil
}

// IsTableEntry checks if an OID is part of a table entry
// Tables have pattern: ENTRY.COLUMN.INDEX where ENTRY ends in .1 and INDEX >= 1.
// Scalar OIDs always end in .0 (instance 0) and must NOT be classified as table entries.
func IsTableEntry(oid string) bool {
	parts := strings.Split(oid, ".")
	if len(parts) < 4 {
		return false
	}

	// Scalars always end in .0 — never a table row.
	if parts[len(parts)-1] == "0" {
		return false
	}

	// Row instance must be a positive integer >= 1.
	var rowIndex int
	if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &rowIndex); err != nil || rowIndex < 1 {
		return false
	}

	// Column index (second-to-last component) must be a non-negative integer.
	var colIndex int
	if _, err := fmt.Sscanf(parts[len(parts)-2], "%d", &colIndex); err != nil {
		return false
	}

	// Check if it ends in .1.COLUMN.INDEX pattern (entry indicator)
	if len(parts) >= 3 && parts[len(parts)-3] == "1" {
		return true
	}

	return false
}

// ExtractTableBase extracts the table base OID from an entry OID
// e.g., 1.3.6.1.2.1.2.2.1 -> 1.3.6.1.2.1.2.2
func ExtractTableBase(entryOID string) string {
	parts := strings.Split(entryOID, ".")
	if len(parts) > 0 && parts[len(parts)-1] == "1" {
		return strings.Join(parts[:len(parts)-1], ".")
	}
	return entryOID
}

// DetectTableStructure analyzes loaded OIDs to detect table structures
// Returns a map of EntryOID -> SNMPTable
func DetectTableStructure(entries []*OIDEntry) map[string]*SNMPTable {
	tables := make(map[string]*SNMPTable)

	// Group OIDs by entry (BASE.1)
	entryGroups := make(map[string][]*OIDEntry)

	for _, entry := range entries {
		if IsTableEntry(entry.OID) {
			entryOID, colIndex, rowIndex, err := ParseTableOID(entry.OID)
			if err != nil {
				continue
			}

			if entryGroups[entryOID] == nil {
				baseOID := ExtractTableBase(entryOID)
				entryGroups[entryOID] = make([]*OIDEntry, 0)
				tables[entryOID] = NewSNMPTable(baseOID, entryOID, baseOID)
			}

			// Add to this entry's group
			entryGroups[entryOID] = append(entryGroups[entryOID], entry)

			// Add row value to table
			table := tables[entryOID]
			table.AddColumn(colIndex, entry.OID, "", "")
			if table.Rows[rowIndex] == nil {
				table.Rows[rowIndex] = &TableRow{
					Index:  rowIndex,
					Values: make(map[int]interface{}),
				}
			}
			table.Rows[rowIndex].Values[colIndex] = entry.Value
		}
	}

	// Rebuild sorted rows for each table
	for _, table := range tables {
		table.rebuildSortedRows()
	}

	return tables
}

// TableStats provides statistics about detected tables
type TableStats struct {
	TableCount int
	TotalRows  int
	TotalCells int
	ByTable    map[string]*TableDetail
}

// TableDetail provides stats for a single table
type TableDetail struct {
	Name      string
	RowCount  int
	ColCount  int
	CellCount int
}

// GetTableStats analyzes detected tables
func GetTableStats(tables map[string]*SNMPTable) *TableStats {
	stats := &TableStats{
		TableCount: len(tables),
		ByTable:    make(map[string]*TableDetail),
	}

	for entryOID, table := range tables {
		detail := &TableDetail{
			Name:      table.Name,
			RowCount:  table.RowCount(),
			ColCount:  table.ColumnCount(),
			CellCount: table.RowCount() * table.ColumnCount(),
		}
		stats.ByTable[entryOID] = detail
		stats.TotalRows += detail.RowCount
		stats.TotalCells += detail.CellCount
	}

	return stats
}
