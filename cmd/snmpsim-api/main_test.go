package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupTestServer(t *testing.T) (*httptest.Server, *ResourceManager) {
	rm := NewResourceManager()

	mux := http.NewServeMux()

	// Register routes using the Router
	router := NewRouter(mux, rm)
	router.Register()

	mux.HandleFunc("/health", healthHandler)

	return httptest.NewServer(mux), rm
}

func validateMethod(method string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}
}

// Test CRUD for Labs
func TestLabsCRUD(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Create lab
	createPayload := []byte(`{"name":"test-lab","engine_id":"engine-1"}`)
	resp, err := client.Post(
		fmt.Sprintf("%s/labs", server.URL),
		"application/json",
		bytes.NewBuffer(createPayload),
	)
	if err != nil {
		t.Fatalf("Failed to create lab: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Errorf("Expected status 201, got %d. Response: %s", resp.StatusCode, string(body))
	}

	var createdLab Lab
	if err := json.NewDecoder(resp.Body).Decode(&createdLab); err != nil {
		t.Fatalf("Failed to decode lab: %v", err)
	}
	resp.Body.Close()

	if createdLab.Name != "test-lab" || createdLab.Status != "stopped" {
		t.Errorf("Lab created with unexpected values: %+v", createdLab)
	}

	// Get lab
	resp, err = client.Get(fmt.Sprintf("%s/labs/%s", server.URL, createdLab.ID))
	if err != nil {
		t.Fatalf("Failed to get lab: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var retrievedLab Lab
	if err := json.NewDecoder(resp.Body).Decode(&retrievedLab); err != nil {
		t.Fatalf("Failed to decode lab: %v", err)
	}
	resp.Body.Close()

	if retrievedLab.ID != createdLab.ID {
		t.Errorf("Retrieved lab ID mismatch")
	}

	// List labs
	resp, err = client.Get(fmt.Sprintf("%s/labs", server.URL))
	if err != nil {
		t.Fatalf("Failed to list labs: %v", err)
	}

	var labs []Lab
	if err := json.NewDecoder(resp.Body).Decode(&labs); err != nil {
		t.Fatalf("Failed to decode labs: %v", err)
	}
	resp.Body.Close()

	if len(labs) < 1 {
		t.Errorf("Expected at least 1 lab, got %d", len(labs))
	}

	// Delete lab
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/labs/%s", server.URL, createdLab.ID), nil)
	if err != nil {
		t.Fatalf("Failed to create delete request: %v", err)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete lab: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify deletion
	resp, err = client.Get(fmt.Sprintf("%s/labs/%s", server.URL, createdLab.ID))
	if err != nil {
		t.Fatalf("Failed to check deleted lab: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for deleted lab, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// Test CRUD for Engines
func TestEnginesCRUD(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Create engine
	createPayload := []byte(`{
		"name":"test-engine",
		"engine_id":"800007e5",
		"listen_addr":"127.0.0.1",
		"port_start":10000,
		"port_end":10010,
		"num_devices":5
	}`)
	resp, err := client.Post(
		fmt.Sprintf("%s/engines", server.URL),
		"application/json",
		bytes.NewBuffer(createPayload),
	)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var createdEngine Engine
	if err := json.NewDecoder(resp.Body).Decode(&createdEngine); err != nil {
		t.Fatalf("Failed to decode engine: %v", err)
	}
	resp.Body.Close()

	if createdEngine.Name != "test-engine" {
		t.Errorf("Engine created with unexpected name: %s", createdEngine.Name)
	}

	// Get engine
	resp, err = client.Get(fmt.Sprintf("%s/engines/%s", server.URL, createdEngine.ID))
	if err != nil {
		t.Fatalf("Failed to get engine: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// List engines
	resp, err = client.Get(fmt.Sprintf("%s/engines", server.URL))
	if err != nil {
		t.Fatalf("Failed to list engines: %v", err)
	}

	var engines []Engine
	if err := json.NewDecoder(resp.Body).Decode(&engines); err != nil {
		t.Fatalf("Failed to decode engines: %v", err)
	}
	resp.Body.Close()

	if len(engines) < 1 {
		t.Errorf("Expected at least 1 engine, got %d", len(engines))
	}

	// Delete engine
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/engines/%s", server.URL, createdEngine.ID), nil)
	if err != nil {
		t.Fatalf("Failed to create delete request: %v", err)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete engine: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// Test CRUD for Endpoints
func TestEndpointsCRUD(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Create endpoint
	createPayload := []byte(`{"name":"test-endpoint","address":"127.0.0.1","port":10000}`)
	resp, err := client.Post(
		fmt.Sprintf("%s/endpoints", server.URL),
		"application/json",
		bytes.NewBuffer(createPayload),
	)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var createdEndpoint Endpoint
	if err := json.NewDecoder(resp.Body).Decode(&createdEndpoint); err != nil {
		t.Fatalf("Failed to decode endpoint: %v", err)
	}
	resp.Body.Close()

	if createdEndpoint.Address != "127.0.0.1" {
		t.Errorf("Endpoint address mismatch: %s", createdEndpoint.Address)
	}

	// Delete endpoint
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/endpoints/%s", server.URL, createdEndpoint.ID), nil)
	if err != nil {
		t.Fatalf("Failed to create delete request: %v", err)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete endpoint: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// Test CRUD for Users
func TestUsersCRUD(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Create user
	createPayload := []byte(`{"name":"test-user","email":"test@example.com"}`)
	resp, err := client.Post(
		fmt.Sprintf("%s/users", server.URL),
		"application/json",
		bytes.NewBuffer(createPayload),
	)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var createdUser User
	if err := json.NewDecoder(resp.Body).Decode(&createdUser); err != nil {
		t.Fatalf("Failed to decode user: %v", err)
	}
	resp.Body.Close()

	if createdUser.Email != "test@example.com" {
		t.Errorf("User email mismatch: %s", createdUser.Email)
	}

	// Delete user
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/users/%s", server.URL, createdUser.ID), nil)
	if err != nil {
		t.Fatalf("Failed to create delete request: %v", err)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// Test CRUD for Datasets
func TestDatasetsCRUD(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Create dataset
	createPayload := []byte(`{
		"name":"test-dataset",
		"engine_id":"engine-1",
		"file_path":"/tmp/test.snmprec"
	}`)
	resp, err := client.Post(
		fmt.Sprintf("%s/datasets", server.URL),
		"application/json",
		bytes.NewBuffer(createPayload),
	)
	if err != nil {
		t.Fatalf("Failed to create dataset: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var createdDataset Dataset
	if err := json.NewDecoder(resp.Body).Decode(&createdDataset); err != nil {
		t.Fatalf("Failed to decode dataset: %v", err)
	}
	resp.Body.Close()

	if createdDataset.FilePath != "/tmp/test.snmprec" {
		t.Errorf("Dataset file path mismatch: %s", createdDataset.FilePath)
	}

	// Delete dataset
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/datasets/%s", server.URL, createdDataset.ID), nil)
	if err != nil {
		t.Fatalf("Failed to create delete request: %v", err)
	}

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete dataset: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// Test health endpoint
func TestHealth(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(fmt.Sprintf("%s/health", server.URL))
	if err != nil {
		t.Fatalf("Failed to get health: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var healthResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}
	resp.Body.Close()

	if healthResp["status"] != "ok" {
		t.Errorf("Expected health status 'ok', got %s", healthResp["status"])
	}
}

// Test Lab lifecycle control
func TestLabLifecycle(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	// Create engine first
	engPayload := []byte(`{
		"name":"test-engine",
		"engine_id":"800007e5",
		"listen_addr":"127.0.0.1",
		"port_start":11000,
		"port_end":11010,
		"num_devices":2
	}`)
	resp, err := client.Post(
		fmt.Sprintf("%s/engines", server.URL),
		"application/json",
		bytes.NewBuffer(engPayload),
	)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	var engine Engine
	if err := json.NewDecoder(resp.Body).Decode(&engine); err != nil {
		t.Fatalf("Failed to decode engine: %v", err)
	}
	resp.Body.Close()

	// Create lab with engine
	labPayload := []byte(fmt.Sprintf(`{"name":"lifecycle-lab","engine_id":"%s"}`, engine.ID))
	resp, err = client.Post(
		fmt.Sprintf("%s/labs", server.URL),
		"application/json",
		bytes.NewBuffer(labPayload),
	)
	if err != nil {
		t.Fatalf("Failed to create lab: %v", err)
	}

	var lab Lab
	if err := json.NewDecoder(resp.Body).Decode(&lab); err != nil {
		t.Fatalf("Failed to decode lab: %v", err)
	}
	resp.Body.Close()

	if lab.Status != "stopped" {
		t.Errorf("Expected initial lab status 'stopped', got %s", lab.Status)
	}

	// Note: Actual start/stop would require valid simulator setup which requires filesystem resources
	// This test demonstrates the endpoint structure; full integration would need proper fixtures

	// Cleanup
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/labs/%s", server.URL, lab.ID), nil)
	client.Do(req)
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("%s/engines/%s", server.URL, engine.ID), nil)
	client.Do(req)
}

// Test error cases
func TestErrorCases(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Get non-existent lab
	resp, err := client.Get(fmt.Sprintf("%s/labs/nonexistent", server.URL))
	if err != nil {
		t.Fatalf("Failed to get non-existent lab: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent lab, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Delete non-existent lab
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/labs/nonexistent", server.URL), nil)
	if err != nil {
		t.Fatalf("Failed to create delete request: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete non-existent lab: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent lab deletion, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create with invalid JSON
	resp, err = client.Post(
		fmt.Sprintf("%s/labs", server.URL),
		"application/json",
		bytes.NewBuffer([]byte("invalid json")),
	)
	if err != nil {
		t.Fatalf("Failed to post invalid JSON: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// Test concurrent operations
func TestConcurrentCRUD(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	// Create 10 labs concurrently
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			payload := []byte(fmt.Sprintf(`{"name":"lab-%d","engine_id":"engine-%d"}`, index, index))
			resp, err := client.Post(
				fmt.Sprintf("%s/labs", server.URL),
				"application/json",
				bytes.NewBuffer(payload),
			)
			if err != nil {
				done <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				done <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
				return
			}

			// Consume response body to prevent connection leak
			io.ReadAll(resp.Body)
			done <- nil
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}
}
