package webui

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Workload represents a SNMP test workload configuration
type Workload struct {
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	TestType      string    `json:"test_type"` // get, getnext, bulkwalk, walk
	OIDs          []string  `json:"oids"`
	PortStart     int       `json:"port_start"`
	PortEnd       int       `json:"port_end"`
	DeviceCount   int       `json:"device_count"`
	Community     string    `json:"community"`
	Timeout       int       `json:"timeout"`
	MaxRepeaters  int       `json:"max_repeaters"`
	Concurrency   int       `json:"concurrency"`
	IntervalSec   int       `json:"interval_seconds"`
	DurationSec   int       `json:"duration_seconds"`
	SNMPrecFile   string    `json:"snmprec_file"`
	SimulatorPath int       `json:"simulator_path"` // Port where simulator listens
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// WorkloadManager handles saving and loading workload configurations
type WorkloadManager struct {
	mu          sync.RWMutex
	workloadDir string
	workloads   map[string]*Workload
}

// NewWorkloadManager creates a new workload manager
func NewWorkloadManager(dir ...string) *WorkloadManager {
	workloadDir := "config/workloads"
	if len(dir) > 0 {
		workloadDir = dir[0]
	}

	wm := &WorkloadManager{
		workloadDir: workloadDir,
		workloads:   make(map[string]*Workload),
	}

	// Create workload directory if it doesn't exist
	if err := os.MkdirAll(wm.workloadDir, 0755); err != nil {
		log.Printf("Warning: Failed to create workload directory: %v", err)
	}

	// Load existing workloads from disk
	wm.loadFromDisk()

	return wm
}

// SaveWorkload saves a workload configuration to disk and memory
func (wm *WorkloadManager) SaveWorkload(workload *Workload) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if workload.Name == "" {
		return fmt.Errorf("workload name is required")
	}

	// Validate workload
	if len(workload.OIDs) == 0 {
		return fmt.Errorf("at least one OID is required")
	}

	now := time.Now()
	if workload.CreatedAt.IsZero() {
		workload.CreatedAt = now
	}
	workload.UpdatedAt = now

	// Save to disk
	filePath := filepath.Join(wm.workloadDir, workload.Name+".json")
	data, err := json.MarshalIndent(workload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workload: %v", err)
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workload file: %v", err)
	}

	// Save to memory
	wm.workloads[workload.Name] = workload

	log.Printf("Workload saved: %s", workload.Name)
	return nil
}

// LoadWorkload loads a workload configuration by name
func (wm *WorkloadManager) LoadWorkload(name string) (*Workload, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workload, exists := wm.workloads[name]
	if !exists {
		return nil, fmt.Errorf("workload not found: %s", name)
	}

	return workload, nil
}

// DeleteWorkload deletes a workload configuration
func (wm *WorkloadManager) DeleteWorkload(name string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	delete(wm.workloads, name)

	filePath := filepath.Join(wm.workloadDir, name+".json")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete workload file: %v", err)
	}

	log.Printf("Workload deleted: %s", name)
	return nil
}

// ListWorkloads returns a list of all available workloads
func (wm *WorkloadManager) ListWorkloads() []Workload {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workloads := make([]Workload, 0, len(wm.workloads))
	for _, w := range wm.workloads {
		workloads = append(workloads, *w)
	}

	return workloads
}

// loadFromDisk loads all workload files from disk
func (wm *WorkloadManager) loadFromDisk() {
	files, err := ioutil.ReadDir(wm.workloadDir)
	if err != nil {
		log.Printf("Warning: Failed to read workload directory: %v", err)
		return
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		name := file.Name()[:len(file.Name())-5] // Remove .json

		filePath := filepath.Join(wm.workloadDir, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Printf("Warning: Failed to read workload file %s: %v", file.Name(), err)
			continue
		}

		var workload Workload
		if err := json.Unmarshal(data, &workload); err != nil {
			log.Printf("Warning: Failed to parse workload file %s: %v", file.Name(), err)
			continue
		}

		wm.workloads[name] = &workload
		log.Printf("Loaded workload: %s", name)
	}
}

// DefaultWorkloads returns a set of default workload templates
func DefaultWorkloads() []Workload {
	return []Workload{
		{
			Name:        "Basic System OIDs",
			Description: "Basic system information (sysDescr, sysUptime, sysServices)",
			TestType:    "get",
			OIDs: []string{
				"1.3.6.1.2.1.1.1.0", // sysDescr
				"1.3.6.1.2.1.1.3.0", // sysUptime
				"1.3.6.1.2.1.1.7.0", // sysServices
			},
			PortStart:   20000,
			PortEnd:     20009,
			DeviceCount: 10,
			Community:   "public",
			Timeout:     5,
			Concurrency: 20,
			IntervalSec: 5,
			DurationSec: 60,
		},
		{
			Name:        "Interface Metrics",
			Description: "Interface statistics (ifDescr, ifInOctets, ifOutOctets)",
			TestType:    "bulkwalk",
			OIDs: []string{
				"1.3.6.1.2.1.2.2.1.2",  // ifDescr (all)
				"1.3.6.1.2.1.2.2.1.10", // ifInOctets (all)
				"1.3.6.1.2.1.2.2.1.16", // ifOutOctets (all)
			},
			PortStart:    20000,
			PortEnd:      20000,
			DeviceCount:  1,
			Community:    "public",
			Timeout:      5,
			MaxRepeaters: 10,
			Concurrency:  5,
			IntervalSec:  5,
			DurationSec:  30,
		},
		{
			Name:        "Full System Walk",
			Description: "Walk entire system subtree (1.3.6.1.2.1.1)",
			TestType:    "walk",
			OIDs: []string{
				"1.3.6.1.2.1.1", // system subtree
			},
			PortStart:   20000,
			PortEnd:     20000,
			DeviceCount: 1,
			Community:   "public",
			Timeout:     10,
			Concurrency: 1,
			IntervalSec: 5,
			DurationSec: 30,
		},
		{
			Name:        "48-Port Switch Test",
			Description: "Test all 48 interface ports from single device",
			TestType:    "bulkwalk",
			OIDs: []string{
				"1.3.6.1.2.1.2.2.1", // Interface table
			},
			PortStart:    20000,
			PortEnd:      20000,
			DeviceCount:  1,
			Community:    "public",
			Timeout:      15,
			MaxRepeaters: 20,
		},
	}
}
