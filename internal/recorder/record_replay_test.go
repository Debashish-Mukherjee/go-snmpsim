package recorder

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/engine"
	"github.com/debashish-mukherjee/go-snmpsim/internal/snmprecfmt"
	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
	"github.com/debashish-mukherjee/go-snmpsim/internal/walkdiff"
	"github.com/gosnmp/gosnmp"
)

func TestRecordReplayDiffIdentical(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.snmprec")
	firstRecord := filepath.Join(tmpDir, "recorded.snmprec")
	secondRecord := filepath.Join(tmpDir, "replayed.snmprec")

	content := `1.3.6.1.2.1.1.1.0|octetstring|Mock Device
1.3.6.1.2.1.1.2.0|objectidentifier|1.3.6.1.4.1.9.9.46.1
1.3.6.1.2.1.1.3.0|timeticks|12345
1.3.6.1.2.1.1.5.0|octetstring|mock-host
`
	if err := os.WriteFile(sourceFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	portA := freeUDPPort(t)
	startSimulator(t, sourceFile, portA)

	entriesA, err := Record(Options{
		Target:    "127.0.0.1",
		Port:      uint16(portA),
		Community: "public",
		Roots:     []string{"1.3.6.1.2.1.1"},
		Exclude:   []string{"1.3.6.1.2.1.1.3"},
		MaxOIDs:   3,
		Timeout:   1500 * time.Millisecond,
		Retries:   0,
	})
	if err != nil {
		t.Fatalf("record source: %v", err)
	}
	if len(entriesA) == 0 {
		t.Fatal("expected non-empty recording")
	}
	if err := snmprecfmt.WriteFile(firstRecord, entriesA); err != nil {
		t.Fatalf("write first recording: %v", err)
	}

	portB := freeUDPPort(t)
	startSimulator(t, firstRecord, portB)

	entriesB, err := Record(Options{
		Target:    "127.0.0.1",
		Port:      uint16(portB),
		Community: "public",
		Roots:     []string{"1.3.6.1.2.1.1"},
		Exclude:   []string{"1.3.6.1.2.1.1.3"},
		MaxOIDs:   3,
		Timeout:   1500 * time.Millisecond,
		Retries:   0,
	})
	if err != nil {
		t.Fatalf("record replay: %v", err)
	}
	if err := snmprecfmt.WriteFile(secondRecord, entriesB); err != nil {
		t.Fatalf("write second recording: %v", err)
	}

	diffResult, err := walkdiff.CompareFiles(firstRecord, secondRecord)
	if err != nil {
		t.Fatalf("diff files: %v", err)
	}
	if !diffResult.Identical() {
		t.Fatalf("expected identical recordings, found %d differences", len(diffResult.Diffs))
	}
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve udp addr: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func startSimulator(t *testing.T, snmprecPath string, port int) {
	t.Helper()
	sim, err := engine.NewSimulator("127.0.0.1", port, port+1, 1, snmprecPath, "", "", v3.Config{Enabled: false})
	if err != nil {
		t.Fatalf("new simulator: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = sim.Start(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		sim.Stop()
	})

	client := &gosnmp.GoSNMP{
		Target:    "127.0.0.1",
		Port:      uint16(port),
		Version:   gosnmp.Version2c,
		Community: "public",
		Timeout:   500 * time.Millisecond,
		Retries:   0,
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := client.Connect(); err == nil {
			pkt, getErr := client.Get([]string{"1.3.6.1.2.1.1.1.0"})
			_ = client.Conn.Close()
			if getErr == nil && pkt != nil && len(pkt.Variables) > 0 {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("simulator on port %d did not become ready", port)
}
