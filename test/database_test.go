package test

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"shipper-deployment/internal/database"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	// Create a temporary database file
	tmpFile := "/tmp/test_" + t.Name() + "_" + time.Now().Format("20060102150405") + ".db"

	// Ensure cleanup
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

	return db
}

func TestInsertDeployment(t *testing.T) {
	db := setupTestDB(t)

	tagID := "test-123"
	serviceName := "test-service"
	jobID := "job-456"
	status := "pending"

	err := database.InsertDeployment(db, tagID, serviceName, jobID, status)
	if err != nil {
		t.Fatalf("InsertDeployment failed: %v", err)
	}

	// Verify the deployment was inserted
	retrievedServiceName, retrievedJobID, retrievedStatus, err := database.GetDeployment(db, tagID)
	if err != nil {
		t.Fatalf("GetDeployment failed: %v", err)
	}

	if retrievedServiceName != serviceName {
		t.Errorf("ServiceName = %v, want %v", retrievedServiceName, serviceName)
	}

	if retrievedJobID != jobID {
		t.Errorf("JobID = %v, want %v", retrievedJobID, jobID)
	}

	if retrievedStatus != status {
		t.Errorf("Status = %v, want %v", retrievedStatus, status)
	}
}

func TestGetDeployment(t *testing.T) {
	db := setupTestDB(t)

	// Test getting non-existent deployment
	_, _, _, err := database.GetDeployment(db, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent deployment, but got none")
	}

	// Insert a deployment
	tagID := "test-123"
	serviceName := "test-service"
	jobID := "job-456"
	status := "running"

	err = database.InsertDeployment(db, tagID, serviceName, jobID, status)
	if err != nil {
		t.Fatalf("InsertDeployment failed: %v", err)
	}

	// Test getting existing deployment
	retrievedServiceName, retrievedJobID, retrievedStatus, err := database.GetDeployment(db, tagID)
	if err != nil {
		t.Fatalf("GetDeployment failed: %v", err)
	}

	if retrievedServiceName != serviceName {
		t.Errorf("ServiceName = %v, want %v", retrievedServiceName, serviceName)
	}

	if retrievedJobID != jobID {
		t.Errorf("JobID = %v, want %v", retrievedJobID, jobID)
	}

	if retrievedStatus != status {
		t.Errorf("Status = %v, want %v", retrievedStatus, status)
	}
}

func TestUpdateDeploymentStatus(t *testing.T) {
	db := setupTestDB(t)

	tagID := "test-123"
	serviceName := "test-service"
	jobID := "job-456"
	initialStatus := "pending"
	updatedStatus := "running"

	// Insert deployment
	err := database.InsertDeployment(db, tagID, serviceName, jobID, initialStatus)
	if err != nil {
		t.Fatalf("InsertDeployment failed: %v", err)
	}

	// Update status
	err = database.UpdateDeploymentStatus(db, tagID, updatedStatus)
	if err != nil {
		t.Fatalf("UpdateDeploymentStatus failed: %v", err)
	}

	// Verify status was updated
	var status string
	query := "SELECT status FROM deployments WHERE tag_id = ?"
	err = db.QueryRow(query, tagID).Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query deployment status: %v", err)
	}

	if status != updatedStatus {
		t.Errorf("Status = %v, want %v", status, updatedStatus)
	}
}

func TestUpdateDeploymentJobID(t *testing.T) {
	db := setupTestDB(t)

	tagID := "test-123"
	serviceName := "test-service"
	initialJobID := ""
	updatedJobID := "job-789"
	status := "pending"

	// Insert deployment without job ID
	err := database.InsertDeployment(db, tagID, serviceName, initialJobID, status)
	if err != nil {
		t.Fatalf("InsertDeployment failed: %v", err)
	}

	// Update job ID
	err = database.UpdateDeploymentJobID(db, tagID, updatedJobID, "running")
	if err != nil {
		t.Fatalf("UpdateDeploymentJobID failed: %v", err)
	}

	// Verify job ID was updated
	_, retrievedJobID, retrievedStatus, err := database.GetDeployment(db, tagID)
	if err != nil {
		t.Fatalf("GetDeployment failed: %v", err)
	}

	if retrievedJobID != updatedJobID {
		t.Errorf("JobID = %v, want %v", retrievedJobID, updatedJobID)
	}

	if retrievedStatus != "running" {
		t.Errorf("Status = %v, want %v", retrievedStatus, "running")
	}
}

func TestDuplicateDeployment(t *testing.T) {
	db := setupTestDB(t)

	tagID := "test-123"
	serviceName := "test-service"
	jobID := "job-456"
	status := "pending"

	// Insert first deployment
	err := database.InsertDeployment(db, tagID, serviceName, jobID, status)
	if err != nil {
		t.Fatalf("First InsertDeployment failed: %v", err)
	}

	// Try to insert duplicate deployment
	err = database.InsertDeployment(db, tagID, "another-service", "another-job", "another-status")
	if err == nil {
		t.Error("Expected error for duplicate deployment, but got none")
	}
}
