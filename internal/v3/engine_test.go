package v3

import (
	"path/filepath"
	"testing"
)

func TestEngineStateStorePersistsBoots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engine-state.json")
	store, err := NewEngineStateStore(path)
	if err != nil {
		t.Fatalf("NewEngineStateStore: %v", err)
	}
	engineID := GenerateEngineID("test-seed")

	boots1, err := store.EnsureBoots(engineID)
	if err != nil {
		t.Fatalf("EnsureBoots: %v", err)
	}
	boots2, err := store.EnsureBoots(engineID)
	if err != nil {
		t.Fatalf("EnsureBoots: %v", err)
	}
	if boots2 <= boots1 {
		t.Fatalf("boots not incremented: boots1=%d boots2=%d", boots1, boots2)
	}

	store2, err := NewEngineStateStore(path)
	if err != nil {
		t.Fatalf("NewEngineStateStore(2): %v", err)
	}
	boots3, err := store2.EnsureBoots(engineID)
	if err != nil {
		t.Fatalf("EnsureBoots(3): %v", err)
	}
	if boots3 <= boots2 {
		t.Fatalf("boots not persisted: boots2=%d boots3=%d", boots2, boots3)
	}
}
