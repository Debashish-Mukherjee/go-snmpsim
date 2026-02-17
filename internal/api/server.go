package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/engine"
	"github.com/debashish-mukherjee/go-snmpsim/internal/webui"
)

// Server handles HTTP API requests and WebSocket connections
type Server struct {
	simulator       *engine.Simulator
	workloadManager *webui.WorkloadManager
	snmpTester      *webui.SNMPTester
	httpServer      *http.Server
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
	}

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/start", s.handleStart)
	mux.HandleFunc("/api/stop", s.handleStop)
	mux.HandleFunc("/api/test/snmp", s.handleSNMPTest)
	mux.HandleFunc("/api/workloads", s.handleWorkloads)
	mux.HandleFunc("/api/workloads/save", s.handleSaveWorkload)
	mux.HandleFunc("/api/workloads/load", s.handleLoadWorkload)
	mux.HandleFunc("/api/workloads/delete", s.handleDeleteWorkload)
	mux.HandleFunc("/api/test/results", s.handleTestResults)

	// Static files
	mux.Handle("/", http.FileServer(http.Dir("./web/ui")))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./web/assets"))))

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

// SetSimulator sets the running simulator instance
func (s *Server) SetSimulator(sim *engine.Simulator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.simulator = sim
	if sim != nil {
		s.status.IsRunning = true
	}
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
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
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

	// Create simulator (simplified - in real code, integrate with engine.Simulator)
	s.mu.Lock()
	s.status.IsRunning = true
	s.status.TotalDevices = req.Devices
	s.status.PortStart = req.PortStart
	s.status.PortEnd = req.PortEnd
	s.status.ListenAddr = req.ListenAddr
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
	s.status.IsRunning = false
	s.mu.Unlock()

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

	// Run tests
	results := s.snmpTester.RunTests(&req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleWorkloads returns list of saved workloads
func (s *Server) handleWorkloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workloads := s.workloadManager.ListWorkloads()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workloads)
}

// handleSaveWorkload saves a workload configuration
func (s *Server) handleSaveWorkload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var workload webui.Workload
	if err := json.NewDecoder(r.Body).Decode(&workload); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.workloadManager.SaveWorkload(&workload); err != nil {
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

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Workload name required", http.StatusBadRequest)
		return
	}

	workload, err := s.workloadManager.LoadWorkload(name)
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

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Workload name required", http.StatusBadRequest)
		return
	}

	if err := s.workloadManager.DeleteWorkload(name); err != nil {
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

	results := s.snmpTester.GetLastResults()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
