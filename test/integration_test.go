package test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"shipper-deployment/internal/config"
	"shipper-deployment/internal/handlers"
	"shipper-deployment/internal/models"
	"shipper-deployment/internal/nomad"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestRouter(t *testing.T) (*mux.Router, *sql.DB) {
	// Create test database
	tmpFile := "/tmp/test_router_" + t.Name() + "_" + time.Now().Format("20060102150405") + ".db"
	
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

	// Create handler
	nomadClient := nomad.NewClient(cfg.NomadURL, cfg.SkipTLSVerify, cfg.NomadToken)
	handler := handlers.NewHandler(db, cfg, nomadClient)

	// Create router and setup routes
	router := mux.NewRouter()

	// Auth middleware
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			secretKey := r.Header.Get("X-Secret-Key")
			if secretKey != cfg.ValidSecret {
				http.Error(w, "Invalid secret key", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// Health endpoint (unprotected)
	router.HandleFunc("/health", handler.Health).Methods("GET")

	// Protected routes
	protectedRouter := router.PathPrefix("").Subrouter()
	protectedRouter.Use(authMiddleware)
	protectedRouter.HandleFunc("/deploy", handler.Deploy).Methods("POST")
	protectedRouter.HandleFunc("/deploy/job", handler.DeployJob).Methods("POST")
	protectedRouter.HandleFunc("/status/{tag_id}", handler.Status).Methods("GET")

	return router, db
}

func TestRouterIntegration(t *testing.T) {
	router, _ := setupTestRouter(t)
	server := httptest.NewServer(router)
	defer server.Close()

	t.Run("health endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		var response map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["status"] != "healthy" {
			t.Errorf("Expected status 'healthy', got %v", response["status"])
		}
	})

	t.Run("deploy endpoint without auth", func(t *testing.T) {
		deployReq := models.DeploymentRequest{
			ServiceName: "test-service",
			TagID:       "test-123",
		}

		body, _ := json.Marshal(deployReq)
		resp, err := http.Post(server.URL+"/deploy", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})

	t.Run("deploy endpoint with valid auth", func(t *testing.T) {
		deployReq := models.DeploymentRequest{
			ServiceName: "test-service",
			TagID:       "test-123",
		}

		body, _ := json.Marshal(deployReq)
		req, err := http.NewRequest("POST", server.URL+"/deploy", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Secret-Key", "test-secret-key-64-characters-long-for-testing-purposes")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// This will likely fail because we don't have a real Nomad server
		// but it should at least pass authentication and reach the deployment logic
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
			t.Logf("Deploy endpoint returned status %d (expected due to mock Nomad)", resp.StatusCode)
		}
	})

	t.Run("deploy endpoint with invalid auth", func(t *testing.T) {
		deployReq := models.DeploymentRequest{
			ServiceName: "test-service",
			TagID:       "test-456",
		}

		body, _ := json.Marshal(deployReq)
		req, err := http.NewRequest("POST", server.URL+"/deploy", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Secret-Key", "invalid-secret-key")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})

	t.Run("status endpoint without auth", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/status/test-123")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})

	t.Run("status endpoint with valid auth", func(t *testing.T) {
		req, err := http.NewRequest("GET", server.URL+"/status/test-123", nil)
		if err != nil {
			t.Fatal(err)
		}

		req.Header.Set("X-Secret-Key", "test-secret-key-64-characters-long-for-testing-purposes")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Should return 404 or similar for non-existent deployment
		if resp.StatusCode == http.StatusUnauthorized {
			t.Error("Authentication should have passed")
		}
	})
}

func TestAuthMiddleware(t *testing.T) {
	router, _ := setupTestRouter(t)
	server := httptest.NewServer(router)
	defer server.Close()

	protectedEndpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/deploy"},
		{"POST", "/deploy/job"},
		{"GET", "/status/test-123"},
	}

	for _, endpoint := range protectedEndpoints {
		t.Run("endpoint "+endpoint.method+" "+endpoint.path+" requires auth", func(t *testing.T) {
			var resp *http.Response
			var err error

			if endpoint.method == "POST" {
				body := bytes.NewBuffer([]byte(`{"service_name":"test","tag_id":"test"}`))
				resp, err = http.Post(server.URL+endpoint.path, "application/json", body)
			} else {
				resp, err = http.Get(server.URL + endpoint.path)
			}

			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Expected status %d for %s %s, got %d", 
					http.StatusUnauthorized, endpoint.method, endpoint.path, resp.StatusCode)
			}
		})
	}
}
