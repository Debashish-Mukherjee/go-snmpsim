package variation

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
)

func TestCounterMonotonic(t *testing.T) {
	v := NewCounterMonotonic(3)
	pdu := PDU{Name: "1.3.6.1.2.1.2.2.1.10.1", Type: gosnmp.Counter32, Value: uint32(100)}

	p1, err := v.Apply(time.Now(), pdu)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	p2, _ := v.Apply(time.Now(), pdu)

	if p1.Value.(uint32) != 103 || p2.Value.(uint32) != 106 {
		t.Fatalf("unexpected counter progression: got %v then %v", p1.Value, p2.Value)
	}
}

func TestRandomJitterDeterministic(t *testing.T) {
	v1 := NewRandomJitter(5, 42)
	v2 := NewRandomJitter(5, 42)
	pdu := PDU{Name: "1.3.6.1.2.1.2.2.1.10.1", Type: gosnmp.Counter32, Value: uint32(1000)}

	a1, _ := v1.Apply(time.Now(), pdu)
	a2, _ := v1.Apply(time.Now(), pdu)
	b1, _ := v2.Apply(time.Now(), pdu)
	b2, _ := v2.Apply(time.Now(), pdu)

	if a1.Value != b1.Value || a2.Value != b2.Value {
		t.Fatalf("jitter should be deterministic for same seed")
	}
}

func TestStep(t *testing.T) {
	v := NewStep(2*time.Second, 10)
	pdu := PDU{Name: "1.3.6.1.2.1.2.2.1.10.2", Type: gosnmp.Counter32, Value: uint32(50)}
	t0 := time.Unix(0, 0)

	p1, _ := v.Apply(t0, pdu)
	p2, _ := v.Apply(t0.Add(1*time.Second), pdu)
	p3, _ := v.Apply(t0.Add(5*time.Second), pdu)

	if p1.Value.(uint32) != 50 || p2.Value.(uint32) != 50 || p3.Value.(uint32) != 70 {
		t.Fatalf("unexpected step values: %v %v %v", p1.Value, p2.Value, p3.Value)
	}
}

func TestPeriodicReset(t *testing.T) {
	v := NewPeriodicReset(3 * time.Second)
	pdu := PDU{Name: "1.3.6.1.2.1.2.2.1.10.3", Type: gosnmp.Counter32, Value: uint32(10)}
	t0 := time.Unix(0, 0)

	p1, _ := v.Apply(t0, pdu)
	p2, _ := v.Apply(t0.Add(1*time.Second), pdu)
	p3, _ := v.Apply(t0.Add(4*time.Second), pdu)

	if p1.Value.(uint32) != 11 || p2.Value.(uint32) != 12 || p3.Value.(uint32) != 10 {
		t.Fatalf("unexpected periodic reset values: %v %v %v", p1.Value, p2.Value, p3.Value)
	}
}

func TestDropOIDAndTimeout(t *testing.T) {
	pdu := PDU{Name: "1.3.6.1.2.1.1.1.0", Type: gosnmp.OctetString, Value: "x"}

	_, err := (&DropOID{}).Apply(time.Now(), pdu)
	if !errors.Is(err, ErrDropOID) {
		t.Fatalf("expected ErrDropOID, got %v", err)
	}

	_, err = (&Timeout{Delay: 0}).Apply(time.Now(), pdu)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
}

func TestBinderLongestPrefixChain(t *testing.T) {
	b, err := NewBinder([]bindingSpec{
		{
			Prefix: "1.3.6.1.2.1.2",
			Variations: []variationSpec{
				{Type: "counterMonotonic", Delta: 1},
			},
		},
		{
			Prefix: "1.3.6.1.2.1.2.2.1.10",
			Variations: []variationSpec{
				{Type: "counterMonotonic", Delta: 5},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBinder error: %v", err)
	}

	pdu := PDU{Name: "1.3.6.1.2.1.2.2.1.10.1", Type: gosnmp.Counter32, Value: uint32(100)}
	out, err := b.Apply(time.Now(), pdu)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if out.Value.(uint32) != 105 {
		t.Fatalf("expected longest-prefix variation delta 5, got %v", out.Value)
	}
}

func TestLoadBinderFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "variations.yaml")
	data := `bindings:
  - prefix: "1.3.6.1.2.1.2.2.1.10"
    variations:
      - type: counterMonotonic
        delta: 2
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	b, err := LoadBinder(path)
	if err != nil {
		t.Fatalf("LoadBinder error: %v", err)
	}

	pdu := PDU{Name: "1.3.6.1.2.1.2.2.1.10.1", Type: gosnmp.Counter32, Value: uint32(5)}
	out, err := b.Apply(time.Now(), pdu)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if out.Value.(uint32) != 7 {
		t.Fatalf("expected counter 7 after variation, got %v", out.Value)
	}
}
