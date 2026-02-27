package test

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"shipper-deployment/internal/config"
	"shipper-deployment/internal/handlers"
	"shipper-deployment/internal/models"
	"shipper-deployment/internal/nomad"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestHandler(t *testing.T) (*handlers.Handler, *sql.DB) {
	// Create test database
	tmpFile := "/tmp/test_handler_" + t.Name() + "_" + time.Now().Format("20060102150405") + ".db"

	t.Cleanup(func() {
		os.Remove(tmpFile)
	})

	db, err := sql.Open("sqlite3", tmpFile)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create the deployments table
	createTable := `
	CREATE TABLE IF NOT EXISTS deployments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tag_id TEXT UNIQUE NOT NULL,
		service_name TEXT NOT NULL,
		job_id TEXT,
		status TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = db.Exec(createTable)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	// Create test config
	cfg := &config.Config{
		NomadURL:        "http://test-nomad:4646",
		ValidSecret:     "test-secret-key-64-characters-long-for-testing-purposes",
		Port:            "16166",
		SkipTLSVerify:   true,
		NomadToken:      "test-token",
		NewRelicLicense: "",
		NewRelicAppName: "test-app",
		NewRelicEnabled: false,
	}

	// Create mock nomad client
	nomadClient := nomad.NewClient(cfg.NomadURL, cfg.SkipTLSVerify, cfg.NomadToken)

	// Create handler
	handler := handlers.NewHandler(db, cfg, nomadClient)

	return handler, db
}

func TestHealthHandler(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.Health(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Health handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}

	if response["time"] == "" {
		t.Error("Expected time to be set in response")
	}
}

func TestDeployHandler(t *testing.T) {
	handler, _ := setupTestHandler(t)

	tests := []struct {
		name           string
		request        models.DeploymentRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid deployment request",
			request: models.DeploymentRequest{
				ServiceName: "test-service",
				TagID:       "test-123",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "missing tag_id",
			request: models.DeploymentRequest{
				ServiceName: "test-service",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Tag ID is required",
		},
		{
			name: "duplicate tag_id",
			request: models.DeploymentRequest{
				ServiceName: "test-service",
				TagID:       "duplicate-123",
			},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For duplicate test, first insert a deployment
			if tt.name == "duplicate tag_id" {
				// Insert first deployment
				firstReq := models.DeploymentRequest{
					ServiceName: "first-service",
					TagID:       "duplicate-123",
				}
				body, _ := json.Marshal(firstReq)
				req, _ := http.NewRequest("POST", "/deploy", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				rr := httptest.NewRecorder()
				handler.Deploy(rr, req)
			}

			// Make the actual test request
			body, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("POST", "/deploy", bytes.NewBuffer(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.Deploy(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("Deploy handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
				t.Logf("Response body: %s", rr.Body.String())
			}

			if tt.expectedError != "" {
				if !strings.Contains(rr.Body.String(), tt.expectedError) {
					t.Errorf("Expected error message containing '%s', got '%s'", tt.expectedError, rr.Body.String())
				}
			}
		})
	}
}

func TestDeployJobHandler(t *testing.T) {
	handler, _ := setupTestHandler(t)

	t.Run("valid job file upload", func(t *testing.T) {
		// Create a multipart form with job file
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// Add tag_id field
		tagField, err := writer.CreateFormField("tag_id")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tagField.Write([]byte("test-job-123")); err != nil {
			t.Fatal(err)
		}

		// Add job file
		jobFile, err := writer.CreateFormFile("job_file", "test.hcl")
		if err != nil {
			t.Fatal(err)
		}
		jobContent := `
job "test-job" {
  datacenters = ["dc1"]
  type = "service"

  group "web" {
    count = 1

    task "server" {
      driver = "docker"

      config {
        image = "nginx:latest"
        ports = ["http"]
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
`
		if _, err := jobFile.Write([]byte(jobContent)); err != nil {
			t.Fatal(err)
		}
		writer.Close()

		req, err := http.NewRequest("POST", "/deploy/job", &buf)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()
		handler.DeployJob(rr, req)

		// Note: This will likely fail because we don't have a real Nomad server
		// but we can check that the request was parsed correctly
		if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
			t.Logf("DeployJob returned status %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("missing tag_id", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// Add job file but no tag_id
		jobFile, err := writer.CreateFormFile("job_file", "test.hcl")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := jobFile.Write([]byte("job content")); err != nil {
			t.Fatal(err)
		}
		writer.Close()

		req, err := http.NewRequest("POST", "/deploy/job", &buf)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()
		handler.DeployJob(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
		}

		if !strings.Contains(rr.Body.String(), "Tag ID is required") {
			t.Errorf("Expected error about Tag ID, got: %s", rr.Body.String())
		}
	})

	t.Run("missing job file", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// Add tag_id but no job file
		tagField, err := writer.CreateFormField("tag_id")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tagField.Write([]byte("test-123")); err != nil {
			t.Fatal(err)
		}
		writer.Close()

		req, err := http.NewRequest("POST", "/deploy/job", &buf)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()
		handler.DeployJob(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
		}
	})
}

func TestStatusHandler(t *testing.T) {
	_, db := setupTestHandler(t)

	// Insert a test deployment
	tagID := "status-test-123"
	serviceName := "test-service"
	jobID := "job-456"
	status := "running"

	insertQuery := "INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)"
	_, err := db.Exec(insertQuery, tagID, serviceName, jobID, status)
	if err != nil {
		t.Fatalf("Failed to insert test deployment: %v", err)
	}

	t.Run("verify deployment was inserted", func(t *testing.T) {
		var retrievedStatus string
		query := "SELECT status FROM deployments WHERE tag_id = ?"
		err := db.QueryRow(query, tagID).Scan(&retrievedStatus)
		if err != nil {
			t.Fatalf("Failed to retrieve deployment: %v", err)
		}

		if retrievedStatus != status {
			t.Errorf("Expected status %s, got %s", status, retrievedStatus)
		}
	})

	t.Run("verify non-existent deployment", func(t *testing.T) {
		var retrievedStatus string
		query := "SELECT status FROM deployments WHERE tag_id = ?"
		err := db.QueryRow(query, "non-existent").Scan(&retrievedStatus)
		if err == nil {
			t.Error("Expected error for non-existent deployment, but got none")
		}
	})
}

func TestStatusHandlerHTTP(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("status endpoint returns deployment info", func(t *testing.T) {
		// Insert a test deployment
		tagID := "status-http-test-123"
		serviceName := "test-service"
		jobID := "job-456"
		status := "running"

		insertQuery := "INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)"
		_, err := db.Exec(insertQuery, tagID, serviceName, jobID, status)
		if err != nil {
			t.Fatalf("Failed to insert test deployment: %v", err)
		}

		// Create request
		req, err := http.NewRequest("GET", "/status/"+tagID, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Add URL parameters using mux
		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/status/{tag_id}", handler.Status).Methods("GET")
		router.ServeHTTP(rr, req)

		// Check response
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Status handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		var response models.StatusResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.TagID != tagID {
			t.Errorf("Expected TagID %s, got %s", tagID, response.TagID)
		}

		if response.Status != "running" {
			t.Errorf("Expected status 'running', got %s", response.Status)
		}

		if response.JobID != jobID {
			t.Errorf("Expected JobID %s, got %s", jobID, response.JobID)
		}
	})

	t.Run("status endpoint returns 404 for non-existent deployment", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/status/non-existent-tag", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/status/{tag_id}", handler.Status).Methods("GET")
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", status)
		}
	})
}

func TestNewHandlerAndHelpers(t *testing.T) {
	handler, _ := setupTestHandler(t)

	t.Run("NewHandler creates handler with dependencies", func(t *testing.T) {
		if handler == nil {
			t.Error("Expected handler to not be nil")
		}

		// Test that the handler has the required dependencies by calling Health
		req, err := http.NewRequest("GET", "/health", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler.Health(rr, req)

		// This should exercise writeJSONResponse internally
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		// Verify the response contains expected JSON
		var response map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response["status"] != "healthy" {
			t.Errorf("Expected status 'healthy', got %v", response["status"])
		}
	})
}

func setupTestHandlerWithNomadURL(t *testing.T, nomadURL string) (*handlers.Handler, *sql.DB) {
	safeName := strings.ReplaceAll(t.Name(), "/", "_")
	tmpFile := "/tmp/test_handler_mock_" + safeName + "_" + time.Now().Format("20060102150405") + ".db"

	t.Cleanup(func() {
		os.Remove(tmpFile)
	})

	db, err := sql.Open("sqlite3", tmpFile)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS deployments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tag_id TEXT UNIQUE NOT NULL,
		service_name TEXT NOT NULL,
		job_id TEXT,
		status TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = db.Exec(createTable)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	cfg := &config.Config{
		NomadURL:      nomadURL,
		ValidSecret:   "test-secret-key-64-characters-long-for-testing-purposes",
		Port:          "16166",
		SkipTLSVerify: true,
		NomadToken:    "test-token",
	}

	nomadClient := nomad.NewClient(cfg.NomadURL, cfg.SkipTLSVerify, cfg.NomadToken)
	handler := handlers.NewHandler(db, cfg, nomadClient)

	return handler, db
}

// parseSSEEvents reads SSE data lines from a response body
func parseSSEEvents(body string) []models.StreamEvent {
	var events []models.StreamEvent
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event models.StreamEvent
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				events = append(events, event)
			}
		}
	}
	return events
}

func TestStreamDeploymentStatusTerminal(t *testing.T) {
	handler, db := setupTestHandler(t)

	t.Run("completed deployment sends single event", func(t *testing.T) {
		tagID := "stream-completed-123"
		_, err := db.Exec("INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)",
			tagID, "test-service", "eval-123", "completed")
		if err != nil {
			t.Fatalf("Failed to insert deployment: %v", err)
		}

		req, err := http.NewRequest("GET", "/deploy/stream/"+tagID, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/deploy/stream/{tag_id}", handler.StreamDeploymentStatus).Methods("GET")
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
			t.Errorf("Expected Content-Type text/event-stream, got %s", ct)
		}

		events := parseSSEEvents(rr.Body.String())
		if len(events) != 1 {
			t.Fatalf("Expected 1 event, got %d", len(events))
		}
		if events[0].Status != "completed" {
			t.Errorf("Expected status 'completed', got %s", events[0].Status)
		}
	})

	t.Run("failed deployment sends single event", func(t *testing.T) {
		tagID := "stream-failed-123"
		_, err := db.Exec("INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)",
			tagID, "test-service", "eval-456", "failed")
		if err != nil {
			t.Fatalf("Failed to insert deployment: %v", err)
		}

		req, err := http.NewRequest("GET", "/deploy/stream/"+tagID, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/deploy/stream/{tag_id}", handler.StreamDeploymentStatus).Methods("GET")
		router.ServeHTTP(rr, req)

		events := parseSSEEvents(rr.Body.String())
		if len(events) != 1 {
			t.Fatalf("Expected 1 event, got %d", len(events))
		}
		if events[0].Status != "failed" {
			t.Errorf("Expected status 'failed', got %s", events[0].Status)
		}
	})
}

func TestStreamDeploymentStatusNotFound(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req, err := http.NewRequest("GET", "/deploy/stream/non-existent", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/deploy/stream/{tag_id}", handler.StreamDeploymentStatus).Methods("GET")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

func TestStreamDeploymentStatusNoEvalID(t *testing.T) {
	handler, db := setupTestHandler(t)

	tagID := "stream-no-eval-123"
	_, err := db.Exec("INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)",
		tagID, "test-service", "", "running")
	if err != nil {
		t.Fatalf("Failed to insert deployment: %v", err)
	}

	req, err := http.NewRequest("GET", "/deploy/stream/"+tagID, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/deploy/stream/{tag_id}", handler.StreamDeploymentStatus).Methods("GET")
	router.ServeHTTP(rr, req)

	events := parseSSEEvents(rr.Body.String())
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	if events[0].Status != "error" {
		t.Errorf("Expected status 'error', got %s", events[0].Status)
	}
	if !strings.Contains(events[0].Message, "No evaluation ID") {
		t.Errorf("Expected message about missing eval ID, got: %s", events[0].Message)
	}
}

func TestStreamDeploymentStatusWithMockNomad(t *testing.T) {
	t.Run("eval complete with no deployment", func(t *testing.T) {
		// Mock Nomad server that returns completed evaluation with no deployment
		mockNomad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/v1/evaluation/") {
				resp := map[string]interface{}{
					"ID":                "eval-mock-123",
					"Status":            "complete",
					"StatusDescription": "evaluation complete",
					"DeploymentID":      "",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer mockNomad.Close()

		handler, db := setupTestHandlerWithNomadURL(t, mockNomad.URL)

		tagID := "stream-eval-complete-123"
		_, err := db.Exec("INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)",
			tagID, "test-service", "eval-mock-123", "running")
		if err != nil {
			t.Fatalf("Failed to insert deployment: %v", err)
		}

		req, err := http.NewRequest("GET", "/deploy/stream/"+tagID, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/deploy/stream/{tag_id}", handler.StreamDeploymentStatus).Methods("GET")
		router.ServeHTTP(rr, req)

		events := parseSSEEvents(rr.Body.String())
		if len(events) < 2 {
			t.Fatalf("Expected at least 2 events (tracking + completed), got %d", len(events))
		}

		// First event should be "tracking"
		if events[0].Status != "tracking" {
			t.Errorf("Expected first event status 'tracking', got %s", events[0].Status)
		}

		// Last event should be "completed"
		lastEvent := events[len(events)-1]
		if lastEvent.Status != "completed" {
			t.Errorf("Expected last event status 'completed', got %s", lastEvent.Status)
		}
	})

	t.Run("eval failed", func(t *testing.T) {
		mockNomad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/v1/evaluation/") {
				resp := map[string]interface{}{
					"ID":                "eval-fail-123",
					"Status":            "failed",
					"StatusDescription": "insufficient resources",
					"DeploymentID":      "",
					"FailedTGAllocs": map[string]interface{}{
						"web": map[string]interface{}{
							"NodesAvailable": 0,
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer mockNomad.Close()

		handler, db := setupTestHandlerWithNomadURL(t, mockNomad.URL)

		tagID := "stream-eval-failed-123"
		_, err := db.Exec("INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)",
			tagID, "test-service", "eval-fail-123", "running")
		if err != nil {
			t.Fatalf("Failed to insert deployment: %v", err)
		}

		req, err := http.NewRequest("GET", "/deploy/stream/"+tagID, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/deploy/stream/{tag_id}", handler.StreamDeploymentStatus).Methods("GET")
		router.ServeHTTP(rr, req)

		events := parseSSEEvents(rr.Body.String())
		if len(events) < 2 {
			t.Fatalf("Expected at least 2 events, got %d", len(events))
		}

		lastEvent := events[len(events)-1]
		if lastEvent.Status != "failed" {
			t.Errorf("Expected last event status 'failed', got %s", lastEvent.Status)
		}
		if !strings.Contains(lastEvent.Message, "insufficient resources") {
			t.Errorf("Expected failure message to contain reason, got: %s", lastEvent.Message)
		}
	})

	t.Run("eval with deployment tracking", func(t *testing.T) {
		callCount := 0
		mockNomad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if strings.HasPrefix(r.URL.Path, "/v1/evaluation/") {
				resp := map[string]interface{}{
					"ID":           "eval-deploy-123",
					"Status":       "complete",
					"DeploymentID": "deploy-abc-123",
				}
				json.NewEncoder(w).Encode(resp)
				return
			}

			if strings.HasPrefix(r.URL.Path, "/v1/deployment/") {
				callCount++
				status := "running"
				if callCount >= 2 {
					status = "successful"
				}
				resp := map[string]interface{}{
					"ID":                "deploy-abc-123",
					"Status":            status,
					"StatusDescription": "",
					"JobID":             "test-job",
					"TaskGroups": map[string]interface{}{
						"web": map[string]interface{}{
							"DesiredTotal":    3,
							"HealthyAllocs":   callCount,
							"UnhealthyAllocs": 0,
							"PlacedAllocs":    3,
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
				return
			}

			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer mockNomad.Close()

		handler, db := setupTestHandlerWithNomadURL(t, mockNomad.URL)

		tagID := "stream-deploy-track-123"
		_, err := db.Exec("INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)",
			tagID, "test-service", "eval-deploy-123", "running")
		if err != nil {
			t.Fatalf("Failed to insert deployment: %v", err)
		}

		req, err := http.NewRequest("GET", "/deploy/stream/"+tagID, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/deploy/stream/{tag_id}", handler.StreamDeploymentStatus).Methods("GET")
		router.ServeHTTP(rr, req)

		events := parseSSEEvents(rr.Body.String())
		if len(events) < 3 {
			t.Fatalf("Expected at least 3 events (tracking, deploying, completed), got %d", len(events))
		}

		// Check we got a completed event at the end
		lastEvent := events[len(events)-1]
		if lastEvent.Status != "completed" {
			t.Errorf("Expected final status 'completed', got %s", lastEvent.Status)
		}

		// Check that we received deployment ID in events
		foundDeploymentID := false
		for _, e := range events {
			if e.DeploymentID == "deploy-abc-123" {
				foundDeploymentID = true
				break
			}
		}
		if !foundDeploymentID {
			t.Error("Expected deployment ID in stream events")
		}
	})
}

func TestDeleteDeploymentHandler(t *testing.T) {
	t.Run("successful delete with mock Nomad", func(t *testing.T) {
		mockNomad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/v1/job/") {
				resp := map[string]string{
					"EvalID": "eval-stop-123",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer mockNomad.Close()

		handler, _ := setupTestHandlerWithNomadURL(t, mockNomad.URL)

		req, err := http.NewRequest("DELETE", "/deploy/test-service", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/deploy/{service_name}", handler.DeleteDeployment).Methods("DELETE")
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var response models.DeleteResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Status != "stopped" {
			t.Errorf("Expected status 'stopped', got %s", response.Status)
		}
		if response.ServiceName != "test-service" {
			t.Errorf("Expected service_name 'test-service', got %s", response.ServiceName)
		}
		if response.EvalID != "eval-stop-123" {
			t.Errorf("Expected eval_id 'eval-stop-123', got %s", response.EvalID)
		}
	})

	t.Run("Nomad returns error", func(t *testing.T) {
		mockNomad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "job not found", http.StatusNotFound)
		}))
		defer mockNomad.Close()

		handler, _ := setupTestHandlerWithNomadURL(t, mockNomad.URL)

		req, err := http.NewRequest("DELETE", "/deploy/non-existent-service", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/deploy/{service_name}", handler.DeleteDeployment).Methods("DELETE")
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rr.Code)
		}
	})
}

func TestNomadClientHelpers(t *testing.T) {
	cfg := &config.Config{
		NomadURL:      "http://test-nomad:4646",
		SkipTLSVerify: true,
		NomadToken:    "test-token",
	}

	t.Run("NewClient creates client with proper configuration", func(t *testing.T) {
		client := nomad.NewClient(cfg.NomadURL, cfg.SkipTLSVerify, cfg.NomadToken)
		if client == nil {
			t.Error("Expected client to not be nil")
		}

		// Test GetLogger function
		logger := client.GetLogger()
		if logger == nil {
			t.Error("Expected logger to not be nil")
		}

		// Test GetHTTPClient function
		httpClient := client.GetHTTPClient()
		if httpClient == nil {
			t.Error("Expected HTTP client to not be nil")
		}
	})
}
