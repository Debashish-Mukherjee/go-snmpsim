//go:build stress

package engine

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
	"github.com/gosnmp/gosnmp"
)

func TestStress2000CiscoIOSDevices(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	snmprec := filepath.Join(repoRoot, "examples", "testdata", "zabbix-48port-switch.snmprec")
	portStart := 20000
	deviceCount := 2000
	portEnd := portStart + deviceCount

	sim, err := NewSimulator("127.0.0.1", portStart, portEnd, deviceCount, snmprec, "", "", v3.Config{Enabled: false})
	if err != nil {
		t.Fatalf("NewSimulator: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = sim.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		sim.Stop()
	})

	time.Sleep(2 * time.Second)

	stats := sim.Statistics()
	if got := stats["active_listeners"].(int); got != deviceCount {
		t.Fatalf("expected %d active listeners, got %d", deviceCount, got)
	}

	var failures atomic.Int64
	start := time.Now()
	queryOID := "1.3.6.1.2.1.1.5.0"
	workers := 128
	ports := make(chan int, deviceCount)
	for port := portStart; port < portEnd; port++ {
		ports <- port
	}
	close(ports)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for port := range ports {
				client := &gosnmp.GoSNMP{
					Target:    "127.0.0.1",
					Port:      uint16(port),
					Version:   gosnmp.Version2c,
					Community: "public",
					Timeout:   1200 * time.Millisecond,
					Retries:   0,
				}
				if err := client.Connect(); err != nil {
					failures.Add(1)
					continue
				}
				pkt, err := client.Get([]string{queryOID})
				_ = client.Conn.Close()
				if err != nil || pkt == nil || len(pkt.Variables) == 0 {
					failures.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	failed := failures.Load()
	failureRate := float64(failed) / float64(deviceCount)
	t.Logf("2000-device GET sweep done in %s, failures=%d (%.2f%%)", elapsed, failed, failureRate*100)
	if failureRate > 0.02 {
		t.Fatalf("failure rate too high: %.2f%% (threshold 2%%)", failureRate*100)
	}

	bulkSamplePorts := 200
	if bulkSamplePorts > deviceCount {
		bulkSamplePorts = deviceCount
	}
	bulkFailures := int64(0)
	for i := 0; i < bulkSamplePorts; i++ {
		port := portStart + i*10
		if port >= portEnd {
			break
		}
		client := &gosnmp.GoSNMP{
			Target:    "127.0.0.1",
			Port:      uint16(port),
			Version:   gosnmp.Version2c,
			Community: "public",
			Timeout:   1500 * time.Millisecond,
			Retries:   0,
			MaxOids:   10,
		}
		if err := client.Connect(); err != nil {
			bulkFailures++
			continue
		}
		pkt, err := client.GetBulk([]string{"1.3.6.1.2.1.2.2.1"}, 0, 10)
		_ = client.Conn.Close()
		if err != nil || pkt == nil || len(pkt.Variables) == 0 {
			bulkFailures++
		}
	}

	bulkFailureRate := float64(bulkFailures) / float64(bulkSamplePorts)
	t.Logf("BULK sample=%d failures=%d (%.2f%%)", bulkSamplePorts, bulkFailures, bulkFailureRate*100)
	if bulkFailureRate > 0.05 {
		t.Fatalf("bulk failure rate too high: %.2f%% (threshold 5%%)", bulkFailureRate*100)
	}

	t.Logf("Stress suite OK: %d Cisco IOS-style devices across ports %d-%d", deviceCount, portStart, portEnd-1)
}
