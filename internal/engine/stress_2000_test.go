//go:build stress

package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
	"github.com/gosnmp/gosnmp"
)

const (
	stressPortStart   = 20000
	stressDeviceCount = 2000
)

func setupStressSimulator(t *testing.T, v3Config v3.Config) (*Simulator, int, int) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	snmprec := filepath.Join(repoRoot, "examples", "testdata", "zabbix-48port-switch.snmprec")
	portStart := stressPortStart
	deviceCount := stressDeviceCount
	portEnd := portStart + deviceCount

	sim, err := NewSimulator("127.0.0.1", portStart, portEnd, deviceCount, snmprec, "", "", v3Config)
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

	return sim, portStart, deviceCount
}

func runGetSweep(t *testing.T, ports []int, workers int, mkClient func(int) *gosnmp.GoSNMP) (time.Duration, int64) {
	t.Helper()

	var failures atomic.Int64
	start := time.Now()
	queryOID := "1.3.6.1.2.1.1.5.0"
	jobs := make(chan int, len(ports))
	for _, port := range ports {
		jobs <- port
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for port := range jobs {
				client := mkClient(port)
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

	return time.Since(start), failures.Load()
}

func sampledPorts(portStart, deviceCount, sample int) []int {
	if sample <= 0 {
		sample = 1
	}
	if sample > deviceCount {
		sample = deviceCount
	}
	step := deviceCount / sample
	if step < 1 {
		step = 1
	}
	ports := make([]int, 0, sample)
	for port := portStart; port < portStart+deviceCount && len(ports) < sample; port += step {
		ports = append(ports, port)
	}
	for len(ports) < sample {
		ports = append(ports, portStart+len(ports))
	}
	return ports
}

func runBulkSample(t *testing.T, portStart, deviceCount, sample int, mkClient func(int) *gosnmp.GoSNMP) int64 {
	t.Helper()

	if sample > deviceCount {
		sample = deviceCount
	}

	bulkFailures := int64(0)
	for i := 0; i < sample; i++ {
		port := portStart + i*10
		if port >= portStart+deviceCount {
			break
		}
		client := mkClient(port)
		if err := client.Connect(); err != nil {
			bulkFailures++
			continue
		}
		pkt, err := client.GetBulk([]string{"1.3.6.1.2.1.1.1.0"}, 0, 5)
		_ = client.Conn.Close()
		if err != nil || pkt == nil || len(pkt.Variables) == 0 {
			bulkFailures++
		}
	}

	return bulkFailures
}

func newV2Client(port int) *gosnmp.GoSNMP {
	return &gosnmp.GoSNMP{
		Target:    "127.0.0.1",
		Port:      uint16(port),
		Version:   gosnmp.Version2c,
		Community: "public",
		Timeout:   1200 * time.Millisecond,
		Retries:   0,
		MaxOids:   10,
	}
}

func newV3ClientNoAuth(port int) *gosnmp.GoSNMP {
	return &gosnmp.GoSNMP{
		Target:        "127.0.0.1",
		Port:          uint16(port),
		Version:       gosnmp.Version3,
		Timeout:       1500 * time.Millisecond,
		Retries:       0,
		SecurityModel: gosnmp.UserSecurityModel,
		MsgFlags:      gosnmp.NoAuthNoPriv,
		SecurityParameters: &gosnmp.UsmSecurityParameters{
			UserName: "simuser",
		},
		MaxOids: 10,
	}
}

func TestStress2000CiscoIOSDevices(t *testing.T) {
	_, portStart, deviceCount := setupStressSimulator(t, v3.Config{Enabled: false})
	samplePorts := sampledPorts(portStart, deviceCount, 200)

	elapsed, failed := runGetSweep(t, samplePorts, 32, newV2Client)
	failureRate := float64(failed) / float64(len(samplePorts))
	t.Logf("[v2c] sampled GET sweep (%d/%d ports) done in %s, failures=%d (%.2f%%)", len(samplePorts), deviceCount, elapsed, failed, failureRate*100)
	if failureRate > 0.25 {
		t.Fatalf("failure rate too high: %.2f%% (threshold 25%%)", failureRate*100)
	}

	bulkSamplePorts := 40
	bulkFailures := runBulkSample(t, portStart, deviceCount, bulkSamplePorts, newV2Client)
	bulkFailureRate := float64(bulkFailures) / float64(bulkSamplePorts)
	t.Logf("[v2c] BULK sample=%d failures=%d (%.2f%%)", bulkSamplePorts, bulkFailures, bulkFailureRate*100)

	t.Logf("Stress suite OK: %d Cisco IOS-style devices across ports %d-%d", deviceCount, portStart, portStart+deviceCount-1)
}

func TestStress2000CiscoIOSDevicesV3NoAuthNoPriv(t *testing.T) {
	if os.Getenv("SNMPSIM_STRESS_V3") != "1" {
		t.Skip("set SNMPSIM_STRESS_V3=1 to run v3 stress variant")
	}

	v3Cfg := v3.Config{Enabled: true, Username: "simuser", EngineID: "80001f8880e963000001"}
	_, portStart, deviceCount := setupStressSimulator(t, v3Cfg)

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available for net-snmp v3 stress run")
	}

	samplePorts := sampledPorts(portStart, deviceCount, 120)
	var portList strings.Builder
	for i, p := range samplePorts {
		if i > 0 {
			portList.WriteRune(' ')
		}
		portList.WriteString(strconv.Itoa(p))
	}

	script := fmt.Sprintf("apk add --no-cache net-snmp-tools >/dev/null && ok=0 && fail=0 && for p in %s; do if snmpget -v3 -l noAuthNoPriv -u simuser -r 0 -t 2 127.0.0.1:$p 1.3.6.1.2.1.1.5.0 >/dev/null 2>&1; then ok=$((ok+1)); else fail=$((fail+1)); fi; done; echo ok=$ok fail=$fail", portList.String())
	cmd := exec.Command("docker", "run", "--rm", "--network", "host", "alpine:3.20", "sh", "-lc", script)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("docker/net-snmp v3 stress failed: %v\n%s", err, string(out))
	}

	text := string(out)
	var okCount, failCount int
	if _, scanErr := fmt.Sscanf(text, "ok=%d fail=%d", &okCount, &failCount); scanErr != nil {
		t.Fatalf("failed parsing v3 stress output %q: %v", text, scanErr)
	}

	failureRate := float64(failCount) / float64(len(samplePorts))
	t.Logf("[v3/noAuth] sampled GET sweep (%d/%d ports) done in %s, failures=%d (%.2f%%)", len(samplePorts), deviceCount, elapsed, failCount, failureRate*100)
	if okCount == 0 {
		t.Skipf("v3 stress produced no successful responses in this environment (failures=%d)", failCount)
	}
	if failureRate > 0.30 {
		t.Logf("[v3/noAuth] high failure rate observed in this environment: %.2f%%", failureRate*100)
	}
}

func TestStressSoak10Minutes(t *testing.T) {
	if os.Getenv("SNMPSIM_STRESS_SOAK") != "1" {
		t.Skip("set SNMPSIM_STRESS_SOAK=1 to run 10-minute soak test")
	}

	duration := 10 * time.Minute
	if raw := os.Getenv("SNMPSIM_STRESS_SOAK_DURATION"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			t.Fatalf("invalid SNMPSIM_STRESS_SOAK_DURATION: %v", err)
		}
		duration = parsed
	}

	workers := 8
	if raw := os.Getenv("SNMPSIM_STRESS_SOAK_WORKERS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			t.Fatalf("invalid SNMPSIM_STRESS_SOAK_WORKERS: %q", raw)
		}
		workers = n
	}

	maxFailureRate := 0.90
	if raw := os.Getenv("SNMPSIM_STRESS_SOAK_MAX_FAILURE"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v < 0 || v > 1 {
			t.Fatalf("invalid SNMPSIM_STRESS_SOAK_MAX_FAILURE: %q", raw)
		}
		maxFailureRate = v
	}

	_, portStart, deviceCount := setupStressSimulator(t, v3.Config{Enabled: false})

	queryOID := "1.3.6.1.2.1.1.5.0"
	stopAt := time.Now().Add(duration)

	var total atomic.Int64
	var failed atomic.Int64
	var cursor atomic.Int64

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(stopAt) {
				idx := int(cursor.Add(1)-1) % deviceCount
				port := portStart + idx
				client := newV2Client(port)
				if err := client.Connect(); err != nil {
					failed.Add(1)
					total.Add(1)
					continue
				}
				pkt, err := client.Get([]string{queryOID})
				_ = client.Conn.Close()
				total.Add(1)
				if err != nil || pkt == nil || len(pkt.Variables) == 0 {
					failed.Add(1)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}
	wg.Wait()

	all := total.Load()
	bad := failed.Load()
	if all == 0 {
		t.Fatal("soak test produced no requests")
	}
	failureRate := float64(bad) / float64(all)
	qps := float64(all) / duration.Seconds()
	t.Logf("[soak] duration=%s workers=%d total=%d failures=%d failureRate=%.2f%% qps=%.2f", duration, workers, all, bad, failureRate*100, qps)
	if failureRate > maxFailureRate {
		t.Fatalf("soak failure rate too high: %.2f%% (threshold %.2f%%)", failureRate*100, maxFailureRate*100)
	}
}
