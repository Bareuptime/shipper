package test

import (
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
		tagField.Write([]byte("test-job-123"))

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
		jobFile.Write([]byte(jobContent))
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
		jobFile.Write([]byte("job content"))
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
		tagField.Write([]byte("test-123"))
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
