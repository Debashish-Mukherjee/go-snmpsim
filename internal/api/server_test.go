package api

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/debashish-mukherjee/go-snmpsim/internal/webui"
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

func TestHandleSNMPTestStartsAsyncJob(t *testing.T) {
	s := NewServer(":0")
	s.SetSNMPTester(webui.NewSNMPTester())

	body := bytes.NewBufferString(`{"test_type":"get","oids":["1.3.6.1.2.1.1.1.0"],"port_start":20000,"port_end":20000,"duration_seconds":1,"interval_seconds":1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/test/snmp", body)
	rec := httptest.NewRecorder()
	s.handleSNMPTest(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["job_id"] == "" {
		t.Fatalf("expected job_id in response")
	}
}

func TestHandleTestJobWithoutTester(t *testing.T) {
	s := NewServer(":0")
	req := httptest.NewRequest(http.MethodGet, "/api/test/jobs/job-1", nil)
	rec := httptest.NewRecorder()
	s.handleTestJob(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestAPIMiddlewareAuth(t *testing.T) {
	t.Setenv("SNMPSIM_UI_API_TOKEN", "secret")
	s := NewServer(":0")
	handler := s.wrapMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-API-Token", "secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPIMiddlewareRateLimit(t *testing.T) {
	t.Setenv("SNMPSIM_UI_RATE_LIMIT_PER_SEC", "1")
	os.Unsetenv("SNMPSIM_UI_API_TOKEN")
	s := NewServer(":0")
	handler := s.wrapMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req1.RemoteAddr = "127.0.0.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", rec1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
}
