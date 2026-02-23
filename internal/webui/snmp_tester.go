package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SNMPTester executes SNMP tests and collects results.
type SNMPTester struct {
	mu          sync.RWMutex
	lastResults *TestResults
	running     bool
	activeJobID string
	jobs        map[string]*TestJob
}

// TestRequest defines parameters for SNMP testing.
type TestRequest struct {
	TestType     string   `json:"test_type"` // get, getnext, bulkwalk, walk
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

// TestResult holds the result of a single SNMP test.
type TestResult struct {
	Port      int       `json:"port"`
	Device    int       `json:"device"`
	OID       string    `json:"oid"`
	Iteration int       `json:"iteration"`
	Success   bool      `json:"success"`
	Value     string    `json:"value"`
	Type      string    `json:"type"`
	LatencyMs float64   `json:"latency_ms"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// TestResults aggregates results from multiple tests.
type TestResults struct {
	TestID       string       `json:"test_id"`
	TestType     string       `json:"test_type"`
	Iterations   int          `json:"iterations"`
	IntervalSec  int          `json:"interval_seconds"`
	Concurrency  int          `json:"concurrency"`
	DurationSec  int          `json:"duration_seconds"`
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

// TestJob tracks asynchronous test execution.
type TestJob struct {
	ID        string       `json:"id"`
	Status    string       `json:"status"` // queued, running, completed, failed, canceled
	Request   *TestRequest `json:"request"`
	Progress  TestProgress `json:"progress"`
	Results   *TestResults `json:"results,omitempty"`
	Error     string       `json:"error,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	StartedAt *time.Time   `json:"started_at,omitempty"`
	EndedAt   *time.Time   `json:"ended_at,omitempty"`

	cancel context.CancelFunc `json:"-"`
}

// TestProgress captures execution progress for async jobs.
type TestProgress struct {
	TotalIterations  int     `json:"total_iterations"`
	CurrentIteration int     `json:"current_iteration"`
	TotalJobs        int     `json:"total_jobs"`
	CompletedJobs    int     `json:"completed_jobs"`
	SuccessCount     int     `json:"success_count"`
	FailureCount     int     `json:"failure_count"`
	RatePerSecond    float64 `json:"rate_per_second"`
	ElapsedSeconds   int     `json:"elapsed_seconds"`
	RemainingSeconds int     `json:"remaining_seconds"`
}

// NewSNMPTester creates a new SNMP tester.
func NewSNMPTester() *SNMPTester {
	return &SNMPTester{
		lastResults: &TestResults{Results: []TestResult{}},
		jobs:        make(map[string]*TestJob),
	}
}

// StartTests starts an asynchronous test job.
func (st *SNMPTester) StartTests(req interface{}) (*TestJob, error) {
	testReq := normalizeTestRequest(req)
	if err := validateTestRequest(testReq); err != nil {
		return nil, err
	}

	st.mu.Lock()
	if st.running {
		active := st.activeJobID
		st.mu.Unlock()
		return nil, fmt.Errorf("tests already running (job %s)", active)
	}
	st.running = true
	jobID := fmt.Sprintf("job_%d", time.Now().UnixNano())
	now := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	totalJobs := (testReq.PortEnd - testReq.PortStart + 1) * len(testReq.OIDs) * testReq.Iterations
	job := &TestJob{
		ID:      jobID,
		Status:  "running",
		Request: testReq,
		Progress: TestProgress{
			TotalIterations:  testReq.Iterations,
			CurrentIteration: 0,
			TotalJobs:        totalJobs,
		},
		CreatedAt: now,
		StartedAt: &now,
		cancel:    cancel,
	}
	st.jobs[jobID] = job
	st.activeJobID = jobID
	st.mu.Unlock()

	go st.runJob(ctx, jobID, testReq)

	return copyJob(job), nil
}

// CancelJob cancels a running test job.
func (st *SNMPTester) CancelJob(id string) bool {
	st.mu.RLock()
	job, ok := st.jobs[id]
	st.mu.RUnlock()
	if !ok || job.cancel == nil || job.Status != "running" {
		return false
	}
	job.cancel()
	return true
}

// GetJob returns a snapshot of a test job.
func (st *SNMPTester) GetJob(id string) (*TestJob, bool) {
	st.mu.RLock()
	job, ok := st.jobs[id]
	st.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return copyJob(job), true
}

// RunTests executes SNMP tests synchronously (legacy behavior).
func (st *SNMPTester) RunTests(req interface{}) *TestResults {
	testReq := normalizeTestRequest(req)
	if err := validateTestRequest(testReq); err != nil {
		return &TestResults{Results: []TestResult{}, ErrorSummary: []string{err.Error()}}
	}

	results := st.executeTests(context.Background(), testReq, func(_ TestProgress) {})
	st.mu.Lock()
	st.lastResults = results
	st.mu.Unlock()
	return results
}

func (st *SNMPTester) runJob(ctx context.Context, jobID string, req *TestRequest) {
	start := time.Now()
	results := st.executeTests(ctx, req, func(progress TestProgress) {
		st.mu.Lock()
		if job, ok := st.jobs[jobID]; ok {
			job.Progress = progress
		}
		st.mu.Unlock()
	})

	st.mu.Lock()
	defer st.mu.Unlock()
	now := time.Now()
	job := st.jobs[jobID]
	job.EndedAt = &now
	job.Results = results
	if ctx.Err() == context.Canceled {
		job.Status = "canceled"
		job.Error = "job canceled"
	} else if len(results.ErrorSummary) > 0 && results.SuccessCount == 0 {
		job.Status = "failed"
		job.Error = strings.Join(results.ErrorSummary, "; ")
	} else {
		job.Status = "completed"
	}
	job.Progress.ElapsedSeconds = int(time.Since(start).Seconds())
	job.Progress.RemainingSeconds = 0
	st.lastResults = results
	st.running = false
	st.activeJobID = ""
}

func (st *SNMPTester) executeTests(ctx context.Context, testReq *TestRequest, progressCb func(TestProgress)) *TestResults {
	startTime := time.Now()
	results := &TestResults{
		TestID:       fmt.Sprintf("test_%d", startTime.UnixNano()),
		TestType:     testReq.TestType,
		Iterations:   testReq.Iterations,
		IntervalSec:  testReq.IntervalSec,
		Concurrency:  testReq.Concurrency,
		DurationSec:  testReq.DurationSec,
		Results:      []TestResult{},
		StartTime:    startTime,
		ErrorSummary: []string{},
	}

	totalJobs := (testReq.PortEnd - testReq.PortStart + 1) * len(testReq.OIDs) * testReq.Iterations
	completed := 0
	success := 0
	failure := 0

	for iter := 1; iter <= testReq.Iterations; iter++ {
		if ctx.Err() != nil {
			break
		}

		iterResults := st.runIteration(ctx, iter, testReq.Concurrency, testReq)
		for _, r := range iterResults {
			results.Results = append(results.Results, r)
			completed++
			if r.Success {
				success++
			} else {
				failure++
			}
			elapsed := int(time.Since(startTime).Seconds())
			rate := 0.0
			if elapsed > 0 {
				rate = float64(completed) / float64(elapsed)
			}
			remaining := 0
			if rate > 0 && totalJobs > completed {
				remaining = int(float64(totalJobs-completed) / rate)
			}
			progressCb(TestProgress{
				TotalIterations:  testReq.Iterations,
				CurrentIteration: iter,
				TotalJobs:        totalJobs,
				CompletedJobs:    completed,
				SuccessCount:     success,
				FailureCount:     failure,
				RatePerSecond:    rate,
				ElapsedSeconds:   elapsed,
				RemainingSeconds: remaining,
			})
		}

		if iter < testReq.Iterations {
			select {
			case <-ctx.Done():
				break
			case <-time.After(time.Duration(testReq.IntervalSec) * time.Second):
			}
		}
	}

	st.calculateStats(results)
	results.EndTime = time.Now()
	results.DurationMs = results.EndTime.Sub(results.StartTime).Milliseconds()
	return results
}

type testJob struct {
	port      int
	deviceNum int
	oid       string
	iteration int
}

func (st *SNMPTester) runIteration(ctx context.Context, iteration, concurrency int, req *TestRequest) []TestResult {
	jobs := make(chan testJob)
	results := make(chan TestResult, max(concurrency, 1))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case job, ok := <-jobs:
				if !ok {
					return
				}
				result := st.executeJob(job, req)
				select {
				case <-ctx.Done():
					return
				case results <- result:
				}
			}
		}
	}

	for i := 0; i < max(concurrency, 1); i++ {
		wg.Add(1)
		go worker()
	}

	go func() {
		defer close(jobs)
		for port := req.PortStart; port <= req.PortEnd; port++ {
			deviceNum := port - req.PortStart
			for _, oid := range req.OIDs {
				select {
				case <-ctx.Done():
					return
				case jobs <- testJob{port: port, deviceNum: deviceNum, oid: oid, iteration: iteration}:
				}
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	iterResults := make([]TestResult, 0)
	for result := range results {
		iterResults = append(iterResults, result)
	}
	return iterResults
}

func (st *SNMPTester) executeJob(job testJob, req *TestRequest) TestResult {
	start := time.Now()
	var value, typeStr string
	var err error
	target := fmt.Sprintf("localhost:%d", job.port)

	switch req.TestType {
	case "getnext":
		value, typeStr, err = st.snmpGetNext(target, job.oid, req.Community, req.Timeout)
	case "walk":
		value, typeStr, err = st.snmpWalkSingle(target, job.oid, req.Community, req.Timeout)
	case "bulkwalk":
		value, typeStr, err = st.snmpBulkwalk(target, job.oid, req.Community, req.Timeout, req.MaxRepeaters)
	default:
		value, typeStr, err = st.snmpGet(target, job.oid, req.Community, req.Timeout)
	}

	result := TestResult{
		Port:      job.port,
		Device:    job.deviceNum,
		OID:       job.oid,
		Iteration: job.iteration,
		LatencyMs: time.Since(start).Seconds() * 1000,
		Timestamp: start,
	}
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}
	result.Success = true
	result.Value = value
	result.Type = typeStr
	return result
}

// snmpGet executes a single SNMP GET request.
func (st *SNMPTester) snmpGet(target, oid, community string, timeout int) (string, string, error) {
	cmd := exec.Command("snmpget", "-v", "2c", "-c", community, "-t", strconv.Itoa(timeout), "-O", "vq", target, oid)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("snmpget failed: %s", strings.TrimSpace(errOut.String()))
	}
	return parseCLIValue(out.String())
}

// snmpGetNext executes a SNMP GETNEXT request.
func (st *SNMPTester) snmpGetNext(target, oid, community string, timeout int) (string, string, error) {
	cmd := exec.Command("snmpgetnext", "-v", "2c", "-c", community, "-t", strconv.Itoa(timeout), "-O", "vq", target, oid)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("snmpgetnext failed: %s", strings.TrimSpace(errOut.String()))
	}
	return parseCLIValue(out.String())
}

// snmpBulkwalk executes a SNMP BULKWALK request.
func (st *SNMPTester) snmpBulkwalk(target, oid, community string, timeout int, maxRepeaters int) (string, string, error) {
	if maxRepeaters <= 0 {
		maxRepeaters = 10
	}
	cmd := exec.Command("snmptable", "-v", "2c", "-c", community, "-t", strconv.Itoa(timeout), "-Cb", "-Cc", target, oid)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return st.snmpGet(target, oid, community, timeout)
	}
	output := strings.TrimSpace(out.String())
	if output == "" {
		return "(empty table)", "TABLE", nil
	}
	lineCount := strings.Count(output, "\n") + 1
	return fmt.Sprintf("[%d rows]", lineCount), "TABLE", nil
}

// snmpWalkSingle performs a WALK operation but returns summarized value.
func (st *SNMPTester) snmpWalkSingle(target, oid, community string, timeout int) (string, string, error) {
	cmd := exec.Command("snmpwalk", "-v", "2c", "-c", community, "-t", strconv.Itoa(timeout), "-O", "vQn", "-m", "ALL", target, oid)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("snmpwalk failed: %s", strings.TrimSpace(errOut.String()))
	}
	output := strings.TrimSpace(out.String())
	if output == "" {
		return "(no values)", "WALK", nil
	}
	lines := strings.Split(output, "\n")
	if len(lines) > 1 {
		return fmt.Sprintf("[%d entries]", len(lines)), "WALK", nil
	}
	parts := strings.SplitN(lines[0], " = ", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1]), "WALK", nil
	}
	return output, "WALK", nil
}

func parseCLIValue(output string) (string, string, error) {
	out := strings.TrimSpace(output)
	parts := strings.SplitN(out, ":", 2)
	typeStr := "STRING"
	value := out
	if len(parts) == 2 {
		typeStr = strings.TrimSpace(parts[0])
		value = strings.TrimSpace(parts[1])
	}
	return value, typeStr, nil
}

// calculateStats computes aggregate statistics for test results.
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

	log.Printf("Test Results: %d/%d successful (%.1f%%), avg latency: %.2fms", results.SuccessCount, results.TotalTests, results.SuccessRate, results.AvgLatencyMs)
}

// GetLastResults returns the most recent test results.
func (st *SNMPTester) GetLastResults() *TestResults {
	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.lastResults == nil {
		return &TestResults{Results: []TestResult{}}
	}
	cloned := *st.lastResults
	cloned.Results = append([]TestResult(nil), st.lastResults.Results...)
	cloned.ErrorSummary = append([]string(nil), st.lastResults.ErrorSummary...)
	return &cloned
}

// IsRunning returns whether tests are currently running.
func (st *SNMPTester) IsRunning() bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.running
}

func normalizeTestRequest(req interface{}) *TestRequest {
	var testReq TestRequest
	if data, err := json.Marshal(req); err == nil {
		_ = json.Unmarshal(data, &testReq)
	}
	if testReq.TestType == "" {
		testReq.TestType = "get"
	}
	if testReq.Community == "" {
		testReq.Community = "public"
	}
	if testReq.Timeout <= 0 {
		testReq.Timeout = 5
	}
	if testReq.IntervalSec <= 0 {
		testReq.IntervalSec = 5
	}
	if testReq.DurationSec > 0 {
		testReq.Iterations = testReq.DurationSec / testReq.IntervalSec
		if testReq.DurationSec%testReq.IntervalSec != 0 {
			testReq.Iterations++
		}
	}
	if testReq.Iterations <= 0 {
		testReq.Iterations = 1
	}
	if testReq.Concurrency <= 0 {
		testReq.Concurrency = 20
	}
	if testReq.Concurrency > 1000 {
		testReq.Concurrency = 1000
	}
	if testReq.MaxRepeaters <= 0 {
		testReq.MaxRepeaters = 10
	}
	return &testReq
}

func validateTestRequest(req *TestRequest) error {
	if req == nil {
		return fmt.Errorf("test request is required")
	}
	if req.PortEnd < req.PortStart {
		return fmt.Errorf("port_end must be greater than or equal to port_start")
	}
	if len(req.OIDs) == 0 {
		return fmt.Errorf("at least one OID is required")
	}
	return nil
}

func copyJob(job *TestJob) *TestJob {
	if job == nil {
		return nil
	}
	copied := *job
	copied.cancel = nil
	if job.Request != nil {
		reqCopy := *job.Request
		reqCopy.OIDs = append([]string(nil), job.Request.OIDs...)
		copied.Request = &reqCopy
	}
	if job.Results != nil {
		resCopy := *job.Results
		resCopy.Results = append([]TestResult(nil), job.Results.Results...)
		resCopy.ErrorSummary = append([]string(nil), job.Results.ErrorSummary...)
		copied.Results = &resCopy
	}
	return &copied
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
