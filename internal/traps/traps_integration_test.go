package traps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not available")
	}
}

func TestTrapEmissionV2CWithSnmptrapd(t *testing.T) {
	requireDocker(t)

	port := 29162
	container, cleanup := startTrapdContainer(t, port, false)
	defer cleanup()

	m, err := NewManager(Config{
		Targets:   []string{fmt.Sprintf("127.0.0.1:%d", port)},
		Version:   "v2c",
		Community: "public",
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	m.Start()
	defer m.Stop()

	m.EnqueueSetEvent(7, 20000, "1.3.6.1.2.1.1.5.0", "OctetString", "test-host")
	time.Sleep(2 * time.Second)

	logs := dockerLogs(t, container)
	if !strings.Contains(logs, "SNMPv2-SMI::enterprises.55555.0.3") {
		t.Fatalf("expected set trap OID in snmptrapd logs, got:\n%s", logs)
	}
	if !strings.Contains(logs, "1.3.6.1.2.1.1.5.0") {
		t.Fatalf("expected set target OID varbind in logs, got:\n%s", logs)
	}
}

func TestTrapEmissionV3WithSnmptrapd(t *testing.T) {
	requireDocker(t)

	port := 29163
	container, cleanup := startTrapdContainer(t, port, true)
	defer cleanup()

	m, err := NewManager(Config{
		Targets:     []string{fmt.Sprintf("127.0.0.1:%d", port)},
		Version:     "v3",
		V3User:      "simuser",
		V3Auth:      "SHA",
		V3AuthKey:   "authpass123",
		V3Priv:      "AES128",
		V3PrivKey:   "privpass123",
		OnVariation: true,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	m.Start()
	defer m.Stop()

	m.EnqueueVariationEvent(1, 20000, "1.3.6.1.2.1.2.2.1.10.1", "value-changed")
	time.Sleep(3 * time.Second)

	logs := dockerLogs(t, container)
	if !strings.Contains(logs, "SNMPv2-SMI::enterprises.55555.0.2") {
		t.Fatalf("expected variation trap OID in snmptrapd logs, got:\n%s", logs)
	}
	if !strings.Contains(logs, "value-changed") {
		t.Fatalf("expected variation detail varbind in snmptrapd logs, got:\n%s", logs)
	}
}

func startTrapdContainer(t *testing.T, port int, v3 bool) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "snmptrapd.conf")
	config := ""
	if v3 {
		config += "createUser simuser SHA authpass123 AES privpass123\n"
		config += "authUser log,execute,net simuser\n"
	} else {
		config += "authCommunity log,execute,net public\n"
	}
	if err := os.WriteFile(confPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write snmptrapd config: %v", err)
	}

	name := fmt.Sprintf("gosnmpsim-trapd-%d", time.Now().UnixNano())
	command := fmt.Sprintf("apk add --no-cache net-snmp net-snmp-tools >/dev/null && snmptrapd -f -Lo -C -c /conf/snmptrapd.conf udp:127.0.0.1:%d", port)
	run := exec.Command("docker", "run", "-d", "--rm", "--network", "host", "--name", name, "-v", tmpDir+":/conf", "alpine:3.20", "sh", "-lc", command)
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("start snmptrapd container: %v\n%s", err, string(out))
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		logs := dockerLogs(t, name)
		if strings.Contains(logs, "NET-SNMP") || strings.Contains(logs, "snmptrapd") {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	cleanup := func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	}
	return name, cleanup
}

func dockerLogs(t *testing.T, name string) string {
	t.Helper()
	out, _ := exec.Command("docker", "logs", name).CombinedOutput()
	return string(out)
}
