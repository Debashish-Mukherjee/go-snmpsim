package store

import (
	"strings"
	"testing"

	"github.com/gosnmp/gosnmp"
)

func TestOIDIndexManagerGetNextUsesNumericOrdering(t *testing.T) {
	db := NewOIDDatabase()
	db.BatchInsert(map[string]*OIDValue{
		"1.3.6.1.2.1.1.2.0":  {Type: gosnmp.Integer, Value: 2},
		"1.3.6.1.2.1.1.10.0": {Type: gosnmp.Integer, Value: 10},
		"1.3.6.1.2.1.1.20.0": {Type: gosnmp.Integer, Value: 20},
	})
	db.SortOIDs()

	im := NewOIDIndexManager()
	if err := im.BuildIndex(db); err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	next, _ := im.GetNext("1.3.6.1.2.1.1.2.0", db)
	if next != "1.3.6.1.2.1.1.10.0" {
		t.Fatalf("GetNext returned %q, want %q", next, "1.3.6.1.2.1.1.10.0")
	}
}

func TestOIDIndexManagerBuildIndexDeterministicOrder(t *testing.T) {
	db := NewOIDDatabase()
	db.BatchInsert(map[string]*OIDValue{
		"1.3.6.1.2.1.1.1.0":        {Type: gosnmp.OctetString, Value: "sysDescr"},
		"1.3.6.1.2.1.2.2.1.2.1":    {Type: gosnmp.OctetString, Value: "ifDescr1"},
		"1.3.6.1.2.1.2.2.1.2.2":    {Type: gosnmp.OctetString, Value: "ifDescr2"},
		"1.3.6.1.2.1.31.1.1.1.1.1": {Type: gosnmp.OctetString, Value: "ifName1"},
		"1.3.6.1.2.1.31.1.1.1.1.2": {Type: gosnmp.OctetString, Value: "ifName2"},
	})
	db.SortOIDs()

	im := NewOIDIndexManager()
	if err := im.BuildIndex(db); err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	baseline := strings.Join(im.sortedOIDs, ",")
	for i := 0; i < 20; i++ {
		if err := im.BuildIndex(db); err != nil {
			t.Fatalf("BuildIndex() iteration %d error = %v", i, err)
		}
		got := strings.Join(im.sortedOIDs, ",")
		if got != baseline {
			t.Fatalf("BuildIndex produced non-deterministic order on iteration %d", i)
		}
	}

	for i := 1; i < len(im.sortedOIDs); i++ {
		if isOIDLess(im.sortedOIDs[i], im.sortedOIDs[i-1]) {
			t.Fatalf("sortedOIDs out of numeric order at index %d: %q before %q", i, im.sortedOIDs[i], im.sortedOIDs[i-1])
		}
	}
}
