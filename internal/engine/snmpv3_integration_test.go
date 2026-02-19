package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
)

func requireDockerAndSNMP(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skip("docker daemon not available")
	}
}

func startV3Simulator(t *testing.T) (context.CancelFunc, string) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	snmprec := filepath.Join(repoRoot, "sample-rich.snmprec")
	if _, err := os.Stat(snmprec); err != nil {
		t.Fatalf("missing sample-rich.snmprec: %v", err)
	}

	cfg := v3.Config{
		Enabled: true,
		Username: "simuser",
		Auth: v3.AuthSHA1,
		AuthKey: "authpass123",
		Priv: v3.PrivAES128,
		PrivKey: "privpass123",
	}

	sim, err := NewSimulator("0.0.0.0", 20000, 20002, 1, snmprec, cfg)
	if err != nil {
		t.Fatalf("NewSimulator: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = sim.Start(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		sim.Stop()
	})

	time.Sleep(2 * time.Second)
	return cancel, "127.0.0.1:20000"
}

func runSNMPCmd(t *testing.T, target string, args ...string) (string, error) {
	t.Helper()
	base := []string{"run", "--rm", "--network", "host", "alpine:3.20", "sh", "-lc", "apk add --no-cache net-snmp-tools >/dev/null && " + strings.Join(args, " ")}
	cmd := exec.Command("docker", base...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestSNMPv3WalkAuthNoPrivAndAuthPriv(t *testing.T) {
	requireDockerAndSNMP(t)
	_, target := startV3Simulator(t)

	cmd1 := "snmpwalk -v3 -l authNoPriv -u simuser -a SHA -A authpass123 " + target + " 1.3.6.1.2.1.1"
	out1, err := runSNMPCmd(t, target, cmd1)
	if err != nil {
		t.Fatalf("authNoPriv walk failed: %v\n%s", err, out1)
	}
	if !strings.Contains(out1, "sysDescr") {
		t.Fatalf("authNoPriv walk missing sysDescr output:\n%s", out1)
	}

	cmd2 := "snmpwalk -v3 -l authPriv -u simuser -a SHA -A authpass123 -x AES -X privpass123 " + target + " 1.3.6.1.2.1.2.2.1"
	out2, err := runSNMPCmd(t, target, cmd2)
	if err != nil {
		t.Fatalf("authPriv walk failed: %v\n%s", err, out2)
	}
	if !strings.Contains(out2, "ifDescr") {
		t.Fatalf("authPriv walk missing ifDescr output:\n%s", out2)
	}
}

func TestSNMPv3NegativeReports(t *testing.T) {
	requireDockerAndSNMP(t)
	_, target := startV3Simulator(t)

	// Each negative test scenario triggers a Report PDU from the simulator.
	// The report contains a USM stats OID; we verify it's present by checking
	// for the OID's BER bytes in the snmpget -d (hex dump) debug output.
	//
	// OIDs and their trailing hex (stripped of spaces from dump):
	//   unknownEngineIDs  1.3.6.1.6.3.15.1.1.4.0  → ...030F010104 00
	//   notInTimeWindows  1.3.6.1.6.3.15.1.1.2.0  → ...030F010102 00
	//   wrongDigests      1.3.6.1.6.3.15.1.1.5.0  → ...030F010105 00
	//
	// We strip spaces from hex lines and search for the unique suffix.
	// snmpget -d lines look like:
	//   "        0000: 30 3E 02 01  03 30 11 02  04 5B 7E C5  17 02 03 00    0>...0...[~...."
	// The hex offset is a 4-digit hex number followed by ": ", then up to 16 bytes
	// in groups of 4 separated by double spaces, then 4 spaces and the ASCII column.
	// Format: "<spaces><NNNN>: <HH HH HH HH  HH HH HH HH  ...>    <ascii>"
	extractHexDump := func(raw string) string {
		var hexOnly strings.Builder
		for _, line := range strings.Split(raw, "\n") {
			// Find the "NNNN: " offset pattern — look for colon followed by space
			// and ensure the part before it (trimmed) is a 4-char hex address
			colonIdx := strings.Index(line, ": ")
			if colonIdx < 0 {
				continue
			}
			prefix := strings.TrimSpace(line[:colonIdx])
			// prefix should be a 4-char hex offset like "0000", "0016", "0096" etc.
			if len(prefix) != 4 {
				continue
			}
			isHexOffset := true
			for _, c := range prefix {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
					isHexOffset = false
					break
				}
			}
			if !isHexOffset {
				continue
			}
			// Extract hex portion after "NNNN: "
			hexPart := line[colonIdx+2:]
			// The hex section is 50 chars (16 bytes: "XX XX XX XX  XX XX XX XX  XX XX XX XX  XX XX XX XX")
			// followed by spaces and then the ASCII column.
			if len(hexPart) > 50 {
				hexPart = hexPart[:50]
			}
			// Strip spaces
			for _, c := range hexPart {
				if c != ' ' {
					hexOnly.WriteRune(c)
				}
			}
		}
		return strings.ToUpper(hexOnly.String())
	}

	cases := []struct {
		name   string
		cmd    string
		expect string // hex bytes (no spaces) unique to this report OID
	}{
		{
			// unknownEngineID: send with a wrong engineID (not matching simulator's)
			name:   "unknownEngineID",
			cmd:    "snmpget -On -d -r 0 -t 5 -v3 -l noAuthNoPriv -u simuser -e 0102030405060708 " + target + " 1.3.6.1.2.1.1.3.0",
			expect: "030F01010400",
		},
		{
			// notInTimeWindow: force boots=1,time=999999 which won't match simulator boots=0
			name:   "notInTimeWindow",
			cmd:    "snmpget -On -d -r 0 -t 5 -v3 -l authNoPriv -u simuser -a SHA -A authpass123 -Z 1,999999 " + target + " 1.3.6.1.2.1.1.3.0",
			expect: "030F01010200",
		},
		{
			// wrongDigest: correct user but wrong passphrase triggers HMAC mismatch Report
			name:   "wrongDigest",
			cmd:    "snmpget -On -d -r 0 -t 5 -v3 -l authNoPriv -u simuser -a SHA -A wrongpass123 " + target + " 1.3.6.1.2.1.1.3.0",
			expect: "030F01010500",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, _ := runSNMPCmd(t, target, tc.cmd)
			hexOut := extractHexDump(out)
			if !strings.Contains(hexOut, tc.expect) {
				t.Fatalf("expected Report OID bytes %q not found in hex dump\noutput:\n%s", tc.expect, out)
			}
		})
	}
}
