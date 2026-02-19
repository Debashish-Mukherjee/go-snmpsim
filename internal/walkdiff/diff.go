package walkdiff

import (
	"fmt"

	"github.com/debashish-mukherjee/go-snmpsim/internal/snmprecfmt"
)

type Difference struct {
	OID        string
	Kind       string
	LeftType   string
	LeftValue  string
	RightType  string
	RightValue string
}

type Result struct {
	LeftCount  int
	RightCount int
	Diffs      []Difference
}

func (r Result) Identical() bool {
	return len(r.Diffs) == 0
}

func CompareFiles(leftPath, rightPath string) (Result, error) {
	leftEntries, err := snmprecfmt.ReadFile(leftPath)
	if err != nil {
		return Result{}, fmt.Errorf("read left file: %w", err)
	}
	rightEntries, err := snmprecfmt.ReadFile(rightPath)
	if err != nil {
		return Result{}, fmt.Errorf("read right file: %w", err)
	}

	leftMap := make(map[string]snmprecfmt.Entry, len(leftEntries))
	for _, e := range leftEntries {
		leftMap[e.OID] = e
	}
	rightMap := make(map[string]snmprecfmt.Entry, len(rightEntries))
	for _, e := range rightEntries {
		rightMap[e.OID] = e
	}

	keys := make([]string, 0, len(leftMap)+len(rightMap))
	seen := make(map[string]struct{}, len(leftMap)+len(rightMap))
	for oid := range leftMap {
		keys = append(keys, oid)
		seen[oid] = struct{}{}
	}
	for oid := range rightMap {
		if _, ok := seen[oid]; !ok {
			keys = append(keys, oid)
		}
	}

	entries := make([]snmprecfmt.Entry, 0, len(keys))
	for _, oid := range keys {
		entries = append(entries, snmprecfmt.Entry{OID: oid})
	}
	snmprecfmt.SortEntries(entries)

	diffs := make([]Difference, 0)
	for _, e := range entries {
		oid := e.OID
		left, leftOK := leftMap[oid]
		right, rightOK := rightMap[oid]

		switch {
		case leftOK && !rightOK:
			diffs = append(diffs, Difference{
				OID:       oid,
				Kind:      "missing-in-right",
				LeftType:  left.Type,
				LeftValue: left.Value,
			})
		case !leftOK && rightOK:
			diffs = append(diffs, Difference{
				OID:        oid,
				Kind:       "missing-in-left",
				RightType:  right.Type,
				RightValue: right.Value,
			})
		case left.Type != right.Type || left.Value != right.Value:
			diffs = append(diffs, Difference{
				OID:        oid,
				Kind:       "value-mismatch",
				LeftType:   left.Type,
				LeftValue:  left.Value,
				RightType:  right.Type,
				RightValue: right.Value,
			})
		}
	}

	return Result{LeftCount: len(leftEntries), RightCount: len(rightEntries), Diffs: diffs}, nil
}
