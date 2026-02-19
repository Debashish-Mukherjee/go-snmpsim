package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/engine"
	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	apiAddr := flag.String("api-addr", "127.0.0.1:8080", "API server address")
	metricsAddr := flag.String("metrics-addr", "127.0.0.1:9090", "Prometheus metrics address")
	flag.Parse()

	// Initialize metrics FIRST
	initMetrics()

	// Create resource manager
	rm := NewResourceManager()

	// Create HTTP mux
	mux := http.NewServeMux()

	// Use a custom router wrapper
	router := NewRouter(mux, rm)
	router.Register()

	// Health and metrics
	mux.HandleFunc("/health", healthHandler)
	mux.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))

	// Start API server
	apiServer := &http.Server{
		Addr:    *apiAddr,
		Handler: mux,
	}

	// Start metrics server
	metricsServer := &http.Server{
		Addr:    *metricsAddr,
		Handler: promhttp.Handler(),
	}

	go func() {
		log.Printf("Starting API server on %s\n", *apiAddr)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API server error: %v", err)
		}
	}()

	go func() {
		log.Printf("Starting metrics server on %s\n", *metricsAddr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Metrics server error: %v", err)
		}
	}()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := apiServer.Shutdown(ctx); err != nil {
		log.Printf("API server shutdown error: %v", err)
	}

	if err := metricsServer.Shutdown(ctx); err != nil {
		log.Printf("Metrics server shutdown error: %v", err)
	}

	rm.Shutdown()
	log.Println("Shutdown complete")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ResourceManager manages all CRUD resources and lab lifecycle
type ResourceManager struct {
	mu        sync.RWMutex
	labs      map[string]*Lab
	engines   map[string]*Engine
	endpoints map[string]*Endpoint
	users     map[string]*User
	datasets  map[string]*Dataset

	labSimulators map[string]*engine.Simulator // labID -> running simulator
	nextID        int
}

// Resource models
type Lab struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	EngineID  string    `json:"engine_id"`
	Status    string    `json:"status"` // "stopped", "running"
	CreatedAt time.Time `json:"created_at"`
}

type Engine struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	EngineID   string `json:"engine_id"`   // SNMPv3 engine ID (hex)
	ListenAddr string `json:"listen_addr"` // IPv4 address
	ListenAddr6 string `json:"listen_addr6"` // IPv6 address (optional)
	PortStart  int    `json:"port_start"`
	PortEnd    int    `json:"port_end"`
	NumDevices int    `json:"num_devices"`
	CreatedAt  time.Time `json:"created_at"`
}

type Endpoint struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"` // IP address
	Port     int    `json:"port"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Dataset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	EngineID string `json:"engine_id"`
	FilePath string `json:"file_path"` // path to SNMP record file
	CreatedAt time.Time `json:"created_at"`
}

// NewResourceManager creates a new resource manager
func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		labs:          make(map[string]*Lab),
		engines:       make(map[string]*Engine),
		endpoints:     make(map[string]*Endpoint),
		users:         make(map[string]*User),
		datasets:      make(map[string]*Dataset),
		labSimulators: make(map[string]*engine.Simulator),
	}
}

// Lab endpoints
func (rm *ResourceManager) CreateLab(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		RecordLatency("POST", "labs", time.Since(startTime).Seconds())
	}()

	var req struct {
		Name     string `json:"name"`
		EngineID string `json:"engine_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RecordFailure("invalid_lab_payload", "labs")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	id := fmt.Sprintf("lab-%d", rm.nextID)
	rm.nextID++

	lab := &Lab{
		ID:        id,
		Name:      req.Name,
		EngineID:  req.EngineID,
		Status:    "stopped",
		CreatedAt: time.Now(),
	}
	rm.labs[id] = lab

	RecordLabCreated()
	RecordPacket("POST", id)
	UpdateActiveAgents(id, 0)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(lab)
}

func (rm *ResourceManager) ListLabs(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	defer func() {
		RecordLatency("GET", "labs", time.Since(startTime).Seconds())
	}()

	rm.mu.RLock()
	labs := make([]*Lab, 0, len(rm.labs))
	for _, lab := range rm.labs {
		labs = append(labs, lab)
	}
	rm.mu.RUnlock()

	// Record metric for API activity
	RecordPacket("GET", "labs")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(labs)
}

func (rm *ResourceManager) GetLab(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	startTime := time.Now()
	defer func() {
		RecordLatency("GET", id, time.Since(startTime).Seconds())
	}()

	rm.mu.RLock()
	lab, ok := rm.labs[id]
	rm.mu.RUnlock()

	if !ok {
		RecordFailure("lab_not_found", id)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	RecordPacket("GET", id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lab)
}

func (rm *ResourceManager) DeleteLab(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rm.mu.Lock()
	lab, ok := rm.labs[id]
	if !ok {
		rm.mu.Unlock()
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if lab.Status == "running" {
		rm.mu.Unlock()
		http.Error(w, "cannot delete running lab", http.StatusConflict)
		return
	}

	delete(rm.labs, id)
	rm.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func (rm *ResourceManager) StartLab(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	startTime := time.Now()
	defer func() {
		RecordLatency("POST", id, time.Since(startTime).Seconds())
	}()

	rm.mu.Lock()
	lab, ok := rm.labs[id]
	if !ok {
		rm.mu.Unlock()
		RecordFailure("lab_not_found", id)
		http.Error(w, "lab not found", http.StatusNotFound)
		return
	}

	if lab.Status == "running" {
		rm.mu.Unlock()
		RecordFailure("lab_already_running", id)
		http.Error(w, "lab already running", http.StatusConflict)
		return
	}

	eng, ok := rm.engines[lab.EngineID]
	if !ok {
		rm.mu.Unlock()
		RecordFailure("engine_not_found", id)
		http.Error(w, "engine not found", http.StatusBadRequest)
		return
	}
	rm.mu.Unlock()

	// Create and start simulator
	v3cfg := v3.Config{
		Enabled: false, // minimal config
	}
	sim, err := engine.NewSimulator(eng.ListenAddr, eng.PortStart, eng.PortEnd, eng.NumDevices, "", "", "", v3cfg)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create simulator: %v", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sim.Start(ctx); err != nil {
		RecordFailure("simulator_start_failed", id)
		http.Error(w, fmt.Sprintf("failed to start simulator: %v", err), http.StatusInternalServerError)
		return
	}

	rm.mu.Lock()
	lab.Status = "running"
	rm.labSimulators[id] = sim
	rm.mu.Unlock()

	RecordLabStart()
	RecordPacket("START", id)
	UpdateActiveAgents(id, eng.NumDevices)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lab)
}

func (rm *ResourceManager) StopLab(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	startTime := time.Now()
	defer func() {
		RecordLatency("POST", id, time.Since(startTime).Seconds())
	}()

	rm.mu.Lock()
	lab, ok := rm.labs[id]
	if !ok {
		rm.mu.Unlock()
		RecordFailure("lab_not_found", id)
		http.Error(w, "lab not found", http.StatusNotFound)
		return
	}

	if lab.Status != "running" {
		rm.mu.Unlock()
		RecordFailure("lab_not_running", id)
		http.Error(w, "lab not running", http.StatusConflict)
		return
	}

	sim, ok := rm.labSimulators[id]
	if !ok {
		rm.mu.Unlock()
		RecordFailure("simulator_not_found", id)
		http.Error(w, "simulator not found", http.StatusInternalServerError)
		return
	}

	sim.Stop()
	delete(rm.labSimulators, id)

	lab.Status = "stopped"
	rm.mu.Unlock()

	RecordLabStop()
	RecordPacket("STOP", id)
	UpdateActiveAgents(id, 0)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lab)
}

// Engine endpoints
func (rm *ResourceManager) CreateEngine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		EngineID    string `json:"engine_id"`
		ListenAddr  string `json:"listen_addr"`
		ListenAddr6 string `json:"listen_addr6"`
		PortStart   int    `json:"port_start"`
		PortEnd     int    `json:"port_end"`
		NumDevices  int    `json:"num_devices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	id := fmt.Sprintf("engine-%d", rm.nextID)
	rm.nextID++

	engine := &Engine{
		ID:          id,
		Name:        req.Name,
		EngineID:    req.EngineID,
		ListenAddr:  req.ListenAddr,
		ListenAddr6: req.ListenAddr6,
		PortStart:   req.PortStart,
		PortEnd:     req.PortEnd,
		NumDevices:  req.NumDevices,
		CreatedAt:   time.Now(),
	}
	rm.engines[id] = engine

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(engine)
}

func (rm *ResourceManager) ListEngines(w http.ResponseWriter, r *http.Request) {
	rm.mu.RLock()
	engines := make([]*Engine, 0, len(rm.engines))
	for _, e := range rm.engines {
		engines = append(engines, e)
	}
	rm.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(engines)
}

func (rm *ResourceManager) GetEngine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rm.mu.RLock()
	engine, ok := rm.engines[id]
	rm.mu.RUnlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(engine)
}

func (rm *ResourceManager) DeleteEngine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.engines[id]; !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Check if engine is in use by any lab
	for _, lab := range rm.labs {
		if lab.EngineID == id && lab.Status == "running" {
			http.Error(w, "engine in use by running lab", http.StatusConflict)
			return
		}
	}

	delete(rm.engines, id)
	w.WriteHeader(http.StatusNoContent)
}

// Endpoint (network address) handlers
func (rm *ResourceManager) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		Port    int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	id := fmt.Sprintf("endpoint-%d", rm.nextID)
	rm.nextID++

	endpoint := &Endpoint{
		ID:        id,
		Name:      req.Name,
		Address:   req.Address,
		Port:      req.Port,
		CreatedAt: time.Now(),
	}
	rm.endpoints[id] = endpoint

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(endpoint)
}

func (rm *ResourceManager) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	rm.mu.RLock()
	endpoints := make([]*Endpoint, 0, len(rm.endpoints))
	for _, e := range rm.endpoints {
		endpoints = append(endpoints, e)
	}
	rm.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}

func (rm *ResourceManager) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rm.mu.RLock()
	endpoint, ok := rm.endpoints[id]
	rm.mu.RUnlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoint)
}

func (rm *ResourceManager) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.endpoints[id]; !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	delete(rm.endpoints, id)
	w.WriteHeader(http.StatusNoContent)
}

// User handlers
func (rm *ResourceManager) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	id := fmt.Sprintf("user-%d", rm.nextID)
	rm.nextID++

	user := &User{
		ID:        id,
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: time.Now(),
	}
	rm.users[id] = user

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func (rm *ResourceManager) ListUsers(w http.ResponseWriter, r *http.Request) {
	rm.mu.RLock()
	users := make([]*User, 0, len(rm.users))
	for _, u := range rm.users {
		users = append(users, u)
	}
	rm.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (rm *ResourceManager) GetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rm.mu.RLock()
	user, ok := rm.users[id]
	rm.mu.RUnlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (rm *ResourceManager) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.users[id]; !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	delete(rm.users, id)
	w.WriteHeader(http.StatusNoContent)
}

// Dataset handlers
func (rm *ResourceManager) CreateDataset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		EngineID string `json:"engine_id"`
		FilePath string `json:"file_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	id := fmt.Sprintf("dataset-%d", rm.nextID)
	rm.nextID++

	dataset := &Dataset{
		ID:        id,
		Name:      req.Name,
		EngineID:  req.EngineID,
		FilePath:  req.FilePath,
		CreatedAt: time.Now(),
	}
	rm.datasets[id] = dataset

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(dataset)
}

func (rm *ResourceManager) ListDatasets(w http.ResponseWriter, r *http.Request) {
	rm.mu.RLock()
	datasets := make([]*Dataset, 0, len(rm.datasets))
	for _, d := range rm.datasets {
		datasets = append(datasets, d)
	}
	rm.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(datasets)
}

func (rm *ResourceManager) GetDataset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rm.mu.RLock()
	dataset, ok := rm.datasets[id]
	rm.mu.RUnlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dataset)
}

func (rm *ResourceManager) DeleteDataset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.datasets[id]; !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	delete(rm.datasets, id)
	w.WriteHeader(http.StatusNoContent)
}

// Shutdown cleanly stops all running labs
func (rm *ResourceManager) Shutdown() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for labID, sim := range rm.labSimulators {
		sim.Stop()
		if lab, ok := rm.labs[labID]; ok {
			lab.Status = "stopped"
		}
	}
}

// Router registers HTTP handlers with proper method routing
type Router struct {
	mux *http.ServeMux
	rm  *ResourceManager
}

// NewRouter creates a new router
func NewRouter(mux *http.ServeMux, rm *ResourceManager) *Router {
	return &Router{mux: mux, rm: rm}
}

// Register registers all API endpoints
func (r *Router) Register() {
	// Labs
	r.mux.HandleFunc("/labs", r.handleLabs)
	r.mux.HandleFunc("/labs/", r.handleLabsDetail)

	// Engines
	r.mux.HandleFunc("/engines", r.handleEngines)
	r.mux.HandleFunc("/engines/", r.handleEnginesDetail)

	// Endpoints
	r.mux.HandleFunc("/endpoints", r.handleEndpoints)
	r.mux.HandleFunc("/endpoints/", r.handleEndpointsDetail)

	// Users
	r.mux.HandleFunc("/users", r.handleUsers)
	r.mux.HandleFunc("/users/", r.handleUsersDetail)

	// Datasets
	r.mux.HandleFunc("/datasets", r.handleDatasets)
	r.mux.HandleFunc("/datasets/", r.handleDatasetsDetail)
}

func (r *Router) handleLabs(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		r.rm.CreateLab(w, req)
	} else if req.Method == http.MethodGet {
		r.rm.ListLabs(w, req)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleLabsDetail(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if !contains(path[6:], "/") {
		// /labs/{id}
		id := path[6:]
		req.SetPathValue("id", id)

		if req.Method == http.MethodGet {
			r.rm.GetLab(w, req)
		} else if req.Method == http.MethodDelete {
			r.rm.DeleteLab(w, req)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	} else {
		// /labs/{id}/{action}
		parts := splitPath(path[6:])
		if len(parts) >= 2 {
			id := parts[0]
			action := parts[1]
			req.SetPathValue("id", id)

			if req.Method == http.MethodPost && action == "start" {
				r.rm.StartLab(w, req)
			} else if req.Method == http.MethodPost && action == "stop" {
				r.rm.StopLab(w, req)
			} else {
				http.Error(w, "not found", http.StatusNotFound)
			}
		} else {
			http.Error(w, "not found", http.StatusNotFound)
		}
	}
}

func (r *Router) handleEngines(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		r.rm.CreateEngine(w, req)
	} else if req.Method == http.MethodGet {
		r.rm.ListEngines(w, req)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleEnginesDetail(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Path[9:] // len("/engines/") = 9
	req.SetPathValue("id", id)

	if req.Method == http.MethodGet {
		r.rm.GetEngine(w, req)
	} else if req.Method == http.MethodDelete {
		r.rm.DeleteEngine(w, req)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleEndpoints(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		r.rm.CreateEndpoint(w, req)
	} else if req.Method == http.MethodGet {
		r.rm.ListEndpoints(w, req)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleEndpointsDetail(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Path[11:] // len("/endpoints/") = 11
	req.SetPathValue("id", id)

	if req.Method == http.MethodGet {
		r.rm.GetEndpoint(w, req)
	} else if req.Method == http.MethodDelete {
		r.rm.DeleteEndpoint(w, req)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleUsers(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		r.rm.CreateUser(w, req)
	} else if req.Method == http.MethodGet {
		r.rm.ListUsers(w, req)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleUsersDetail(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Path[7:] // len("/users/") = 7
	req.SetPathValue("id", id)

	if req.Method == http.MethodGet {
		r.rm.GetUser(w, req)
	} else if req.Method == http.MethodDelete {
		r.rm.DeleteUser(w, req)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleDatasets(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		r.rm.CreateDataset(w, req)
	} else if req.Method == http.MethodGet {
		r.rm.ListDatasets(w, req)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleDatasetsDetail(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Path[10:] // len("/datasets/") = 10
	req.SetPathValue("id", id)

	if req.Method == http.MethodGet {
		r.rm.GetDataset(w, req)
	} else if req.Method == http.MethodDelete {
		r.rm.DeleteDataset(w, req)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Helper functions
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func splitPath(path string) []string {
	var parts []string
	var current string
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(path[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
