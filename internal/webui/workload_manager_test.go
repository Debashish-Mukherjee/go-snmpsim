package webui

import (
	"testing"
)

func TestWorkloadManagerRejectsInvalidName(t *testing.T) {
	wm := NewWorkloadManager(t.TempDir())
	err := wm.SaveWorkload(&Workload{
		Name: "invalid/name",
		OIDs: []string{"1.3.6.1.2.1.1.1.0"},
	})
	if err == nil {
		t.Fatalf("expected invalid workload name to fail")
	}
}

func TestWorkloadManagerBlocksPathTraversalDelete(t *testing.T) {
	wm := NewWorkloadManager(t.TempDir())
	err := wm.DeleteWorkload("../../etc/passwd")
	if err == nil {
		t.Fatalf("expected traversal workload name to fail")
	}
}

func TestWorkloadManagerSaveAndLoadMaxRepeaters(t *testing.T) {
	wm := NewWorkloadManager(t.TempDir())
	want := &Workload{
		Name:         "prod_48_port",
		Description:  "test",
		TestType:     "bulkwalk",
		OIDs:         []string{"1.3.6.1.2.1.2.2.1"},
		PortStart:    20000,
		PortEnd:      20000,
		MaxRepeaters: 25,
	}
	if err := wm.SaveWorkload(want); err != nil {
		t.Fatalf("SaveWorkload() error = %v", err)
	}
	got, err := wm.LoadWorkload(want.Name)
	if err != nil {
		t.Fatalf("LoadWorkload() error = %v", err)
	}
	if got.MaxRepeaters != want.MaxRepeaters {
		t.Fatalf("max_repeaters = %d, want %d", got.MaxRepeaters, want.MaxRepeaters)
	}
}
