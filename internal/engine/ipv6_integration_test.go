package engine

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
	"github.com/gosnmp/gosnmp"
)

func TestWalkViaIPv6Loopback(t *testing.T) {
	if _, err := net.ListenPacket("udp6", "[::1]:0"); err != nil {
		t.Skipf("ipv6 loopback unavailable: %v", err)
	}

	port, err := freeUDPPort6()
	if err != nil {
		t.Fatalf("free ipv6 port: %v", err)
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	snmprec := filepath.Join(repoRoot, "sample-rich.snmprec")

	sim, err := NewSimulator("127.0.0.1", port, port+1, 1, snmprec, "", "", v3.Config{Enabled: false})
	if err != nil {
		t.Fatalf("new simulator: %v", err)
	}
	sim.SetListenAddr6("::1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = sim.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		sim.Stop()
	})

	time.Sleep(600 * time.Millisecond)

	client := &gosnmp.GoSNMP{
		Target:    "::1",
		Port:      uint16(port),
		Version:   gosnmp.Version2c,
		Community: "public",
		Timeout:   2 * time.Second,
		Retries:   0,
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("connect ipv6: %v", err)
	}
	defer client.Conn.Close()

	count := 0
	err = client.Walk("1.3.6.1.2.1.1", func(variable gosnmp.SnmpPDU) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("ipv6 walk: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least one varbind from ipv6 walk")
	}
}

func freeUDPPort6() (int, error) {
	addr, err := net.ResolveUDPAddr("udp6", "[::1]:0")
	if err != nil {
		return 0, err
	}
	conn, err := net.ListenUDP("udp6", addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port, nil
}
