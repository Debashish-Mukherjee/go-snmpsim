package webui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SNMPTester executes SNMP tests and collects results
type SNMPTester struct {
	mu          sync.RWMutex
	lastResults *TestResults
	running     bool
}

// TestRequest defines parameters for SNMP testing
type TestRequest struct {
	TestType     string   `json:"test_type"` // get, getnext, bulkwalk, walk
	OIDs         []string `json:"oids"`
	PortStart    int      `json:"port_start"`
	PortEnd      int      `json:"port_end"`
	Community    string   `json:"community"`
	Timeout      int      `json:"timeout"`
	MaxRepeaters int      `json:"max_repeaters"`
}

// TestResult holds the result of a single SNMP test
type TestResult struct {
	Port      int       `json:"port"`
	Device    int       `json:"device"`
	OID       string    `json:"oid"`
	Success   bool      `json:"success"`
	Value     string    `json:"value"`
	Type      string    `json:"type"`
	LatencyMs float64   `json:"latency_ms"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// TestResults aggregates results from multiple tests
type TestResults struct {
	TestID       string       `json:"test_id"`
	TestType     string       `json:"test_type"`
	TotalTests   int          `json:"total_tests"`
	SuccessCount int          `json:"success_count"`
	FailureCount int          `json:"failure_count"`
	SuccessRate  float64      `json:"success_rate"`
	AvgLatencyMs float64      `json:"avg_latency_ms"`
	MinLatencyMs float64      `json:"min_latency_ms"`
	MaxLatencyMs float64      `json:"max_latency_ms"`
	Results      []TestResult `json:"results"`
	StartTime    time.Time    `json:"start_time"`
	EndTime      time.Time    `json:"end_time"`
	DurationMs   int64        `json:"duration_ms"`
	ErrorSummary []string     `json:"error_summary"`
}

// NewSNMPTester creates a new SNMP tester
func NewSNMPTester() *SNMPTester {
	return &SNMPTester{
		lastResults: &TestResults{
			Results: []TestResult{},
		},
	}
}

// RunTests executes SNMP tests based on the request
func (st *SNMPTester) RunTests(req interface{}) *TestResults {
	st.mu.Lock()
	if st.running {
		st.mu.Unlock()
		return &TestResults{
			TotalTests:   0,
			Results:      []TestResult{},
			ErrorSummary: []string{"Tests already running"},
		}
	}
	st.running = true
	st.mu.Unlock()

	defer func() {
		st.mu.Lock()
		st.running = false
		st.mu.Unlock()
	}()

	// Convert request to TestRequest
	var testReq TestRequest
	if data, err := json.Marshal(req); err == nil {
		json.Unmarshal(data, &testReq)
	}

	startTime := time.Now()
	results := &TestResults{
		TestID:       fmt.Sprintf("test_%d", startTime.Unix()),
		TestType:     testReq.TestType,
		Results:      []TestResult{},
		StartTime:    startTime,
		ErrorSummary: []string{},
	}

	// Run tests for each port/device
	for port := testReq.PortStart; port <= testReq.PortEnd; port++ {
		deviceNum := port - testReq.PortStart
		testResults := st.runTestForPort(port, deviceNum, &testReq)
		results.Results = append(results.Results, testResults...)
	}

	// Calculate statistics
	st.calculateStats(results)

	results.EndTime = time.Now()
	results.DurationMs = results.EndTime.Sub(results.StartTime).Milliseconds()

	st.mu.Lock()
	st.lastResults = results
	st.mu.Unlock()

	return results
}

// runTestForPort executes SNMP tests on a specific port
func (st *SNMPTester) runTestForPort(port, deviceNum int, req *TestRequest) []TestResult {
	results := []TestResult{}
	target := fmt.Sprintf("localhost:%d", port)

	for _, oid := range req.OIDs {
		start := time.Now()
		value, typeStr, err := st.snmpGet(target, oid, req.Community, req.Timeout)
		latency := time.Since(start).Seconds() * 1000

		result := TestResult{
			Port:      port,
			Device:    deviceNum,
			OID:       oid,
			LatencyMs: latency,
			Timestamp: start,
		}

		if err != nil {
			result.Success = false
			result.Error = err.Error()
			req.TestType = req.TestType
		} else {
			result.Success = true
			result.Value = value
			result.Type = typeStr
		}

		results = append(results, result)
	}

	return results
}

// snmpGet executes a single SNMP GET request
func (st *SNMPTester) snmpGet(target, oid, community string, timeout int) (string, string, error) {
	cmd := exec.Command(
		"snmpget",
		"-v", "2c",
		"-c", community,
		"-t", strconv.Itoa(timeout),
		"-O", "vq", // Value only, quick print
		target,
		oid,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	err := cmd.Run()
	if err != nil {
		return "", "", fmt.Errorf("snmpget failed: %s", errOut.String())
	}

	output := strings.TrimSpace(out.String())
	// Parse output: "STRING: value" or "INTEGER: 123" etc.
	parts := strings.SplitN(output, ":", 2)
	typeStr := "STRING"
	value := output
	if len(parts) == 2 {
		typeStr = strings.TrimSpace(parts[0])
		value = strings.TrimSpace(parts[1])
	}

	return value, typeStr, nil
}

// snmpWalk executes a SNMP WALK request
func (st *SNMPTester) snmpWalk(target, oid, community string, timeout int) ([]TestResult, error) {
	results := []TestResult{}

	cmd := exec.Command(
		"snmpwalk",
		"-v", "2c",
		"-c", community,
		"-t", strconv.Itoa(timeout),
		"-O", "vQn",
		target,
		oid,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("snmpwalk failed: %s", errOut.String())
	}

	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " = ", 2)
		if len(parts) != 2 {
			continue
		}

		oidStr := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		result := TestResult{
			OID:       oidStr,
			Value:     value,
			Success:   true,
			Timestamp: time.Now(),
		}
		results = append(results, result)
	}

	return results, nil
}

// calculateStats computes aggregate statistics for test results
func (st *SNMPTester) calculateStats(results *TestResults) {
	if len(results.Results) == 0 {
		return
	}

	var totalLatency float64
	minLatency := results.Results[0].LatencyMs
	maxLatency := results.Results[0].LatencyMs

	for _, result := range results.Results {
		results.TotalTests++
		if result.Success {
			results.SuccessCount++
		} else {
			results.FailureCount++
			results.ErrorSummary = append(results.ErrorSummary, result.Error)
		}

		totalLatency += result.LatencyMs
		if result.LatencyMs < minLatency {
			minLatency = result.LatencyMs
		}
		if result.LatencyMs > maxLatency {
			maxLatency = result.LatencyMs
		}
	}

	if results.TotalTests > 0 {
		results.AvgLatencyMs = totalLatency / float64(results.TotalTests)
		results.SuccessRate = float64(results.SuccessCount) / float64(results.TotalTests) * 100
	}
	results.MinLatencyMs = minLatency
	results.MaxLatencyMs = maxLatency

	log.Printf("Test Results: %d/%d successful (%.1f%%), avg latency: %.2fms",
		results.SuccessCount, results.TotalTests, results.SuccessRate, results.AvgLatencyMs)
}

// GetLastResults returns the most recent test results
func (st *SNMPTester) GetLastResults() *TestResults {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.lastResults
}

// IsRunning returns whether tests are currently running
func (st *SNMPTester) IsRunning() bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.running
}
