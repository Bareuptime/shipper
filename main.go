package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

type DeploymentRequest struct {
	ServiceName string `json:"service_name"`
	SecretKey   string `json:"secret_key"`
}

type DeploymentResponse struct {
	Status  string `json:"status"`
	TagID   string `json:"tag_id"`
	JobID   string `json:"job_id,omitempty"`
	Message string `json:"message,omitempty"`
}

type StatusResponse struct {
	Status  string `json:"status"`
	TagID   string `json:"tag_id"`
	JobID   string `json:"job_id"`
	Message string `json:"message,omitempty"`
}

type NomadJobResponse struct {
	EvalID string `json:"EvalID"`
	JobID  string `json:"JobID"`
}

type NomadEvalResponse struct {
	Status string `json:"Status"`
}

type Config struct {
	NomadURL    string
	ValidSecret string
	Port        string
}

func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", "./bastion.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	// Create deployments table
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
		log.Fatal("Failed to create table:", err)
	}

	return db
}

func insertDeployment(db *sql.DB, tagID, serviceName, jobID, status string) error {
	stmt, err := db.Prepare("INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(tagID, serviceName, jobID, status)
	return err
}

func updateDeploymentStatus(db *sql.DB, tagID, status string) error {
	stmt, err := db.Prepare("UPDATE deployments SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE tag_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(status, tagID)
	return err
}

func getDeployment(db *sql.DB, tagID string) (string, string, string, error) {
	var serviceName, jobID, status string
	err := db.QueryRow("SELECT service_name, job_id, status FROM deployments WHERE tag_id = ?", tagID).
		Scan(&serviceName, &jobID, &status)
	return serviceName, jobID, status, err
}

func main() {
	config := Config{
		NomadURL:    getEnv("NOMAD_URL", "http://10.10.85.1:4646"),
		ValidSecret: getEnv("VALID_SECRET", "your-64-character-secret-key-here-please-change-this-in-production"),
		Port:        getEnv("PORT", "8080"),
	}

	db := initDB()
	defer db.Close()

	r := mux.NewRouter()

	// Health endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}).Methods("GET")

	// Deploy endpoint
	r.HandleFunc("/deploy", func(w http.ResponseWriter, r *http.Request) {
		handleDeploy(w, r, db, config)
	}).Methods("POST")

	// Status endpoint
	r.HandleFunc("/status/{tag_id}", func(w http.ResponseWriter, r *http.Request) {
		handleStatus(w, r, db, config)
	}).Methods("GET")

	log.Printf("Server starting on port %s", config.Port)
	log.Fatal(http.ListenAndServe(":"+config.Port, r))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func handleDeploy(w http.ResponseWriter, r *http.Request, db *sql.DB, config Config) {
	var req DeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate secret key
	if req.SecretKey != config.ValidSecret {
		http.Error(w, "Invalid secret key", http.StatusUnauthorized)
		return
	}

	// Generate unique tag ID
	tagID := uuid.New().String()

	// Store initial deployment record
	if err := insertDeployment(db, tagID, req.ServiceName, "", "pending"); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Trigger Nomad deployment
	jobID, err := triggerNomadDeployment(req.ServiceName, tagID, config.NomadURL)
	if err != nil {
		updateDeploymentStatus(db, tagID, "failed")
		response := DeploymentResponse{
			Status:  "failed",
			TagID:   tagID,
			Message: err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Update with job ID
	stmt, err := db.Prepare("UPDATE deployments SET job_id = ?, status = ? WHERE tag_id = ?")
	if err == nil {
		stmt.Exec(jobID, "running", tagID)
		stmt.Close()
	}

	response := DeploymentResponse{
		Status: "running",
		TagID:  tagID,
		JobID:  jobID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleStatus(w http.ResponseWriter, r *http.Request, db *sql.DB, config Config) {
	vars := mux.Vars(r)
	tagID := vars["tag_id"]

	_, jobID, status, err := getDeployment(db, tagID)
	if err != nil {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	// Check current status from Nomad if job is running
	if status == "running" && jobID != "" {
		nomadStatus, err := getNomadJobStatus(jobID, config.NomadURL)
		if err == nil && nomadStatus != status {
			updateDeploymentStatus(db, tagID, nomadStatus)
			status = nomadStatus
		}
	}

	response := StatusResponse{
		Status: status,
		TagID:  tagID,
		JobID:  jobID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func triggerNomadDeployment(serviceName, tagID, nomadURL string) (string, error) {
	// Create Nomad job payload
	jobPayload := map[string]interface{}{
		"Job": map[string]interface{}{
			"ID":   fmt.Sprintf("%s-%s", serviceName, tagID[:8]),
			"Name": serviceName,
			"Type": "service",
			"Meta": map[string]string{
				"tag_id": tagID,
			},
			"TaskGroups": []map[string]interface{}{
				{
					"Name": serviceName,
					"Tasks": []map[string]interface{}{
						{
							"Name":   serviceName,
							"Driver": "docker",
							"Config": map[string]interface{}{
								"image": fmt.Sprintf("%s:latest", serviceName),
							},
						},
					},
				},
			},
		},
	}

	// Convert to JSON
	payloadBytes, err := json.Marshal(jobPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal job payload: %v", err)
	}

	// Make HTTP request to Nomad
	client := &http.Client{Timeout: 30 * time.Second}
	url := fmt.Sprintf("%s/v1/jobs", nomadURL)

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to submit job to Nomad: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Nomad returned status: %d", resp.StatusCode)
	}

	var nomadResp NomadJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&nomadResp); err != nil {
		return "", fmt.Errorf("failed to decode Nomad response: %v", err)
	}

	return nomadResp.EvalID, nil
}

func getNomadJobStatus(evalID, nomadURL string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/v1/evaluation/%s", nomadURL, evalID)

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get job status from Nomad: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Nomad returned status: %d", resp.StatusCode)
	}

	var evalResp NomadEvalResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		return "", fmt.Errorf("failed to decode Nomad response: %v", err)
	}

	// Map Nomad status to our status
	switch evalResp.Status {
	case "complete":
		return "completed", nil
	case "failed":
		return "failed", nil
	case "pending":
		return "running", nil
	default:
		return "running", nil
	}
}
