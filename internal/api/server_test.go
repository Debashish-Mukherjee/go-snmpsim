package api

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func freeUDPPort() (int, bool) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		return 0, false
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port, true
}

func TestHandleSNMPTestWithoutTester(t *testing.T) {
	s := NewServer(":0")
	body := bytes.NewBufferString(`{"test_type":"get","oids":["1.3.6.1.2.1.1.1.0"],"port_start":20000,"port_end":20000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/test/snmp", body)
	rec := httptest.NewRecorder()

	s.handleSNMPTest(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleWorkloadsWithoutManager(t *testing.T) {
	s := NewServer(":0")
	req := httptest.NewRequest(http.MethodGet, "/api/workloads", nil)
	rec := httptest.NewRecorder()

	s.handleWorkloads(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleStartStopLifecycle(t *testing.T) {
	s := NewServer(":0")
	port, ok := freeUDPPort()
	if !ok {
		t.Skip("UDP sockets unavailable in this environment")
	}

	payload := map[string]interface{}{
		"port_start":   port,
		"port_end":     port + 1,
		"devices":      1,
		"listen_addr":  "127.0.0.1",
		"snmprec_file": "",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/api/start", bytes.NewReader(raw))
	startRec := httptest.NewRecorder()
	s.handleStart(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("start status = %d, want %d, body=%s", startRec.Code, http.StatusOK, startRec.Body.String())
	}

	s.mu.RLock()
	running := s.status.IsRunning
	sim := s.simulator
	cancel := s.simCancel
	s.mu.RUnlock()
	if !running || sim == nil || cancel == nil {
		t.Fatalf("simulator state invalid after start: running=%v simNil=%v cancelNil=%v", running, sim == nil, cancel == nil)
	}

	stopReq := httptest.NewRequest(http.MethodPost, "/api/stop", nil)
	stopRec := httptest.NewRecorder()
	s.handleStop(stopRec, stopReq)
	if stopRec.Code != http.StatusOK {
		t.Fatalf("stop status = %d, want %d", stopRec.Code, http.StatusOK)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.status.IsRunning || s.simulator != nil || s.simCancel != nil {
		t.Fatalf("simulator state invalid after stop: running=%v simNil=%v cancelNil=%v", s.status.IsRunning, s.simulator == nil, s.simCancel == nil)
	}
}
