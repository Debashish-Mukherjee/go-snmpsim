package store

import (
	"fmt"
	"testing"

	"github.com/gosnmp/gosnmp"
)

// BenchmarkGetNext measures OID lookup performance
func BenchmarkGetNext(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("OIDs_%d", size), func(b *testing.B) {
			db := createTestDatabase(size)

			// Use a middle OID for testing
			testOID := fmt.Sprintf("1.3.6.1.2.1.2.2.1.10.%d", size/2)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = db.GetNext(testOID)
			}
		})
	}
}

// BenchmarkOIDComparison measures OID comparison performance
func BenchmarkOIDComparison(b *testing.B) {
	oid1 := "1.3.6.1.2.1.2.2.1.10.12345"
	oid2 := "1.3.6.1.2.1.2.2.1.10.12346"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isOIDLess(oid1, oid2)
	}
}

// BenchmarkParseOIDComponent measures component parsing performance
func BenchmarkParseOIDComponent(b *testing.B) {
	oid := "1.3.6.1.2.1.2.2.1.10.12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		num, next := parseOIDComponent(oid, 0)
		_, _ = num, next
	}
}

// BenchmarkBatchInsert measures batch insert performance
func BenchmarkBatchInsert(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("OIDs_%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				db := NewOIDDatabase()
				entries := make(map[string]*OIDValue, size)
				for j := 0; j < size; j++ {
					oid := fmt.Sprintf("1.3.6.1.2.1.2.2.1.10.%d", j)
					entries[oid] = &OIDValue{
						Type:  gosnmp.Counter32,
						Value: uint32(j * 1000),
					}
				}
				b.StartTimer()

				db.BatchInsert(entries)
			}
		})
	}
}

// BenchmarkDatabaseWalk measures database traversal performance
func BenchmarkDatabaseWalk(b *testing.B) {
	db := createTestDatabase(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		db.Walk(func(oid string, value *OIDValue) bool {
			count++
			return true
		})
	}
}

// createTestDatabase creates a test database with N OIDs
func createTestDatabase(n int) *OIDDatabase {
	db := NewOIDDatabase()

	// Create interface table entries
	for i := 1; i <= n; i++ {
		oid := fmt.Sprintf("1.3.6.1.2.1.2.2.1.10.%d", i)
		db.Insert(oid, &OIDValue{
			Type:  gosnmp.Counter32,
			Value: uint32(i * 1000),
		})
	}

	db.SortOIDs()
	return db
}

// TestGetNextCorrectness verifies GetNext still works correctly after optimization
func TestGetNextCorrectness(t *testing.T) {
	db := NewOIDDatabase()

	// Insert some test OIDs
	testOIDs := []string{
		"1.3.6.1.2.1.1.1.0",
		"1.3.6.1.2.1.1.2.0",
		"1.3.6.1.2.1.1.3.0",
		"1.3.6.1.2.1.2.1.0",
		"1.3.6.1.2.1.2.2.1.1.1",
		"1.3.6.1.2.1.2.2.1.1.2",
	}

	for _, oid := range testOIDs {
		db.Insert(oid, &OIDValue{Type: gosnmp.OctetString, Value: "test"})
	}
	db.SortOIDs()

	// Test GetNext
	tests := []struct {
		input    string
		expected string
	}{
		{"1.3.6.1.2.1.1.1.0", "1.3.6.1.2.1.1.2.0"},
		{"1.3.6.1.2.1.1.2.0", "1.3.6.1.2.1.1.3.0"},
		{"1.3.6.1.2.1.1.3.0", "1.3.6.1.2.1.2.1.0"},
		{"1.3.6.1.2.1.2.2.1.1.1", "1.3.6.1.2.1.2.2.1.1.2"},
		{"1.3.6.1.2.1.2.2.1.1.2", ""}, // Last OID
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := db.GetNext(tt.input)
			if got != tt.expected {
				t.Errorf("GetNext(%s) = %s; want %s", tt.input, got, tt.expected)
			}
		})
	}
}

// TestOIDComparisonCorrectness verifies OID comparison works correctly
func TestOIDComparisonCorrectness(t *testing.T) {
	tests := []struct {
		oid1     string
		oid2     string
		expected bool
	}{
		{"1.3.6", "1.3.7", true},
		{"1.3.7", "1.3.6", false},
		{"1.3.6.1", "1.3.6.2", true},
		{"1.3.6.1.2", "1.3.6.1.10", true},
		{"1.3.6.1.10", "1.3.6.1.2", false},
		{"1.3.6.1", "1.3.6.1.1", true}, // Shorter is less
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s<%s", tt.oid1, tt.oid2), func(t *testing.T) {
			got := isOIDLess(tt.oid1, tt.oid2)
			if got != tt.expected {
				t.Errorf("isOIDLess(%s, %s) = %v; want %v", tt.oid1, tt.oid2, got, tt.expected)
			}
		})
	}
}
