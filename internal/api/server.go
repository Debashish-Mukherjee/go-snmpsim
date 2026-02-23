package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/engine"
	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
	"github.com/debashish-mukherjee/go-snmpsim/internal/webui"
	webstatic "github.com/debashish-mukherjee/go-snmpsim/web"
)

// Server handles HTTP API requests and WebSocket connections
type Server struct {
	simulator       *engine.Simulator
	workloadManager *webui.WorkloadManager
	snmpTester      *webui.SNMPTester
	httpServer      *http.Server
	simCancel       context.CancelFunc
	apiToken        string
	limiter         *requestLimiter
	mu              sync.RWMutex
	status          *SimulatorStatus
}

// SimulatorStatus contains current simulator metrics
type SimulatorStatus struct {
	IsRunning    bool   `json:"is_running"`
	TotalDevices int    `json:"total_devices"`
	PortStart    int    `json:"port_start"`
	PortEnd      int    `json:"port_end"`
	ListenAddr   string `json:"listen_addr"`
	StartTime    string `json:"start_time"`
	Uptime       string `json:"uptime"`
	TotalPolls   int64  `json:"total_polls"`
	AvgLatency   string `json:"avg_latency_ms"`
}

// NewServer creates a new API server
func NewServer(addr string) *Server {
	s := &Server{
		status: &SimulatorStatus{
			IsRunning: false,
		},
		apiToken: os.Getenv("SNMPSIM_UI_API_TOKEN"),
		limiter:  newRequestLimiterFromEnv(),
	}

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/api/start", s.handleStart)
	mux.HandleFunc("/api/stop", s.handleStop)
	mux.HandleFunc("/api/test/snmp", s.handleSNMPTest)
	mux.HandleFunc("/api/workloads", s.handleWorkloads)
	mux.HandleFunc("/api/workloads/save", s.handleSaveWorkload)
	mux.HandleFunc("/api/workloads/load", s.handleLoadWorkload)
	mux.HandleFunc("/api/workloads/delete", s.handleDeleteWorkload)
	mux.HandleFunc("/api/test/results", s.handleTestResults)
	mux.HandleFunc("/api/test/jobs/", s.handleTestJob)

	// Static files (embedded so they are independent of current working directory).
	uiFS, err := fs.Sub(webstatic.EmbeddedFiles, "ui")
	if err != nil {
		panic(fmt.Sprintf("load embedded ui assets: %v", err))
	}
	assetsFS, err := fs.Sub(webstatic.EmbeddedFiles, "assets")
	if err != nil {
		panic(fmt.Sprintf("load embedded asset files: %v", err))
	}
	mux.Handle("/", http.FileServer(http.FS(uiFS)))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS))))

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.wrapMiddleware(mux),
	}

	return s
}

// SetSimulator sets the running simulator instance
func (s *Server) SetSimulator(sim *engine.Simulator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.simulator = sim
	s.status.IsRunning = sim != nil
}

// SetSimulatorStatus sets the simulator status details
func (s *Server) SetSimulatorStatus(portStart, portEnd, numDevices int, listenAddr, startTime string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.IsRunning = true
	s.status.TotalDevices = numDevices
	s.status.PortStart = portStart
	s.status.PortEnd = portEnd
	s.status.ListenAddr = listenAddr
	s.status.StartTime = startTime
}

// SetWorkloadManager sets the workload manager
func (s *Server) SetWorkloadManager(wm *webui.WorkloadManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workloadManager = wm
}

// SetSNMPTester sets the SNMP tester
func (s *Server) SetSNMPTester(tester *webui.SNMPTester) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snmpTester = tester
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting Web UI on http://localhost%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Stop stops the HTTP server gracefully
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// handleStatus returns current simulator status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	status := *s.status
	sim := s.simulator
	tester := s.snmpTester
	s.mu.RUnlock()

	if sim != nil {
		if stats := sim.Statistics(); stats != nil {
			if totalPolls, ok := stats["total_polls"].(int64); ok {
				status.TotalPolls = totalPolls
			}
		}
	}
	if tester != nil {
		if last := tester.GetLastResults(); last != nil && last.TotalTests > 0 {
			status.AvgLatency = fmt.Sprintf("%.2f", last.AvgLatencyMs)
		}
	}
	if status.IsRunning && status.StartTime != "" {
		if startedAt, err := time.Parse(time.RFC3339, status.StartTime); err == nil {
			status.Uptime = formatUptime(time.Since(startedAt))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	sim := s.simulator
	s.mu.RUnlock()

	totalPolls := int64(0)
	virtualAgents := 0
	running := 0

	if sim != nil {
		if stats := sim.Statistics(); stats != nil {
			if total, ok := stats["total_polls"].(int64); ok {
				totalPolls = total
			}
			if count, ok := stats["virtual_agents"].(int); ok {
				virtualAgents = count
			}
			if isRunning, ok := stats["running"].(bool); ok && isRunning {
				running = 1
			}
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintln(w, "# HELP snmpsim_simulator_polls_total Total SNMP packets handled by simulator")
	fmt.Fprintln(w, "# TYPE snmpsim_simulator_polls_total counter")
	fmt.Fprintln(w, "snmpsim_simulator_polls_total "+strconv.FormatInt(totalPolls, 10))
	fmt.Fprintln(w, "# HELP snmpsim_simulator_agents Number of active simulator virtual agents")
	fmt.Fprintln(w, "# TYPE snmpsim_simulator_agents gauge")
	fmt.Fprintln(w, "snmpsim_simulator_agents "+strconv.Itoa(virtualAgents))
	fmt.Fprintln(w, "# HELP snmpsim_simulator_running Simulator running state (1 up, 0 down)")
	fmt.Fprintln(w, "# TYPE snmpsim_simulator_running gauge")
	fmt.Fprintln(w, "snmpsim_simulator_running "+strconv.Itoa(running))
}

// handleStart starts the simulator with given parameters
func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PortStart   int    `json:"port_start"`
		PortEnd     int    `json:"port_end"`
		Devices     int    `json:"devices"`
		ListenAddr  string `json:"listen_addr"`
		SNMPrecFile string `json:"snmprec_file"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.ListenAddr == "" {
		req.ListenAddr = "0.0.0.0"
	}
	if req.Devices == 0 {
		req.Devices = 10
	}
	if req.PortEnd <= req.PortStart {
		http.Error(w, "port_end must be greater than port_start", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.simulator != nil {
		s.mu.Unlock()
		http.Error(w, "simulator already running", http.StatusConflict)
		return
	}

	sim, err := engine.NewSimulator(
		req.ListenAddr,
		req.PortStart,
		req.PortEnd,
		req.Devices,
		req.SNMPrecFile,
		"",
		"",
		v3.Config{},
	)
	if err != nil {
		s.mu.Unlock()
		http.Error(w, fmt.Sprintf("failed to create simulator: %v", err), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := sim.Start(ctx); err != nil {
		cancel()
		s.mu.Unlock()
		http.Error(w, fmt.Sprintf("failed to start simulator: %v", err), http.StatusInternalServerError)
		return
	}

	s.simCancel = cancel
	s.simulator = sim
	s.status.IsRunning = true
	s.status.TotalDevices = req.Devices
	s.status.PortStart = req.PortStart
	s.status.PortEnd = req.PortEnd
	s.status.ListenAddr = req.ListenAddr
	s.status.StartTime = time.Now().Format(time.RFC3339)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Simulator started successfully",
	})
}

// handleStop stops the simulator
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	sim := s.simulator
	cancel := s.simCancel
	s.simulator = nil
	s.simCancel = nil
	s.status.IsRunning = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if sim != nil {
		sim.Stop()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "stopped",
		"message": "Simulator stopped successfully",
	})
}

// handleSNMPTest runs SNMP tests on configured devices
func (s *Server) handleSNMPTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TestType     string   `json:"test_type"` // get, getnext, bulkwalk
		OIDs         []string `json:"oids"`
		PortStart    int      `json:"port_start"`
		PortEnd      int      `json:"port_end"`
		Community    string   `json:"community"`
		Timeout      int      `json:"timeout"`
		MaxRepeaters int      `json:"max_repeaters"`
		Concurrency  int      `json:"concurrency"`
		Iterations   int      `json:"iterations"`
		IntervalSec  int      `json:"interval_seconds"`
		DurationSec  int      `json:"duration_seconds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.Community == "" {
		req.Community = "public"
	}
	if req.Timeout == 0 {
		req.Timeout = 5
	}

	s.mu.RLock()
	tester := s.snmpTester
	s.mu.RUnlock()
	if tester == nil {
		http.Error(w, "SNMP tester not configured", http.StatusServiceUnavailable)
		return
	}

	job, err := tester.StartTests(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to start tests: %v", err), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "test job started",
	})
}

// handleWorkloads returns list of saved workloads
func (s *Server) handleWorkloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	wm := s.workloadManager
	s.mu.RUnlock()
	if wm == nil {
		http.Error(w, "workload manager not configured", http.StatusServiceUnavailable)
		return
	}

	workloads := wm.ListWorkloads()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workloads)
}

// handleSaveWorkload saves a workload configuration
func (s *Server) handleSaveWorkload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	wm := s.workloadManager
	s.mu.RUnlock()
	if wm == nil {
		http.Error(w, "workload manager not configured", http.StatusServiceUnavailable)
		return
	}

	var workload webui.Workload
	if err := json.NewDecoder(r.Body).Decode(&workload); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if err := wm.SaveWorkload(&workload); err != nil {
		http.Error(w, fmt.Sprintf("Error saving workload: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

// handleLoadWorkload loads a workload configuration
func (s *Server) handleLoadWorkload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	wm := s.workloadManager
	s.mu.RUnlock()
	if wm == nil {
		http.Error(w, "workload manager not configured", http.StatusServiceUnavailable)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Workload name required", http.StatusBadRequest)
		return
	}

	workload, err := wm.LoadWorkload(name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading workload: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workload)
}

// handleDeleteWorkload deletes a workload configuration
func (s *Server) handleDeleteWorkload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	wm := s.workloadManager
	s.mu.RUnlock()
	if wm == nil {
		http.Error(w, "workload manager not configured", http.StatusServiceUnavailable)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Workload name required", http.StatusBadRequest)
		return
	}

	if err := wm.DeleteWorkload(name); err != nil {
		http.Error(w, fmt.Sprintf("Error deleting workload: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// handleTestResults returns latest test results
func (s *Server) handleTestResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	tester := s.snmpTester
	s.mu.RUnlock()
	if tester == nil {
		http.Error(w, "SNMP tester not configured", http.StatusServiceUnavailable)
		return
	}

	results := tester.GetLastResults()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleTestJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	tester := s.snmpTester
	s.mu.RUnlock()
	if tester == nil {
		http.Error(w, "SNMP tester not configured", http.StatusServiceUnavailable)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/test/jobs/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.Error(w, "job id required", http.StatusBadRequest)
		return
	}
	parts := strings.Split(path, "/")
	jobID := parts[0]

	if r.Method == http.MethodPost {
		if len(parts) != 2 || parts[1] != "cancel" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if ok := tester.CancelJob(jobID); !ok {
			http.Error(w, "job not running or not found", http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "canceling"})
		return
	}

	if len(parts) != 1 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	job, ok := tester.GetJob(jobID)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}

func (s *Server) wrapMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if s.apiToken != "" && !authorized(r, s.apiToken) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if s.limiter != nil && !s.limiter.Allow(clientIP(r)) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func authorized(r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	if r.Header.Get("X-API-Token") == token {
		return true
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") && strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")) == token {
		return true
	}
	return false
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if host == "" {
		return "unknown"
	}
	return host
}

type requestLimiter struct {
	mu          sync.Mutex
	perSecond   int
	clientState map[string]*clientRateState
}

type clientRateState struct {
	epochSecond int64
	count       int
}

func newRequestLimiterFromEnv() *requestLimiter {
	limit := 60
	if raw := strings.TrimSpace(os.Getenv("SNMPSIM_UI_RATE_LIMIT_PER_SEC")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return &requestLimiter{perSecond: limit, clientState: make(map[string]*clientRateState)}
}

func (rl *requestLimiter) Allow(ip string) bool {
	if rl == nil {
		return true
	}
	if ip == "" {
		ip = "unknown"
	}
	now := time.Now().Unix()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	state, ok := rl.clientState[ip]
	if !ok || state.epochSecond != now {
		rl.clientState[ip] = &clientRateState{epochSecond: now, count: 1}
		return true
	}
	if state.count >= rl.perSecond {
		return false
	}
	state.count++
	return true
}

func formatUptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02dh %02dm %02ds", h, m, s)
}
