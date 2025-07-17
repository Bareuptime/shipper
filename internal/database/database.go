package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB() *sql.DB {
	log.Println("Initializing database connection")
	dbPath := "./bastion.db"

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	log.Printf("Successfully connected to database at %s", dbPath)

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
	log.Println("Database tables initialized")

	return db
}

func InsertDeployment(db *sql.DB, tagID, serviceName, jobID, status string) error {
	log.Printf("Inserting deployment: tag_id=%s, service=%s, status=%s", tagID, serviceName, status)
	stmt, err := db.Prepare("INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)")
	if err != nil {
		log.Printf("ERROR preparing insert statement: %v", err)
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(tagID, serviceName, jobID, status)
	if err != nil {
		log.Printf("ERROR executing insert statement: %v", err)
		return fmt.Errorf("failed to insert deployment: %w", err)
	}
	log.Printf("Successfully inserted deployment with tag_id=%s", tagID)
	return nil
}

func UpdateDeploymentStatus(db *sql.DB, tagID, status string) error {
	stmt, err := db.Prepare("UPDATE deployments SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE tag_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(status, tagID)
	return err
}

func UpdateDeploymentJobID(db *sql.DB, tagID, jobID, status string) error {
	stmt, err := db.Prepare("UPDATE deployments SET job_id = ?, status = ?, updated_at = CURRENT_TIMESTAMP WHERE tag_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(jobID, status, tagID)
	return err
}

func GetDeployment(db *sql.DB, tagID string) (string, string, string, error) {
	var serviceName, jobID, status string
	err := db.QueryRow("SELECT service_name, job_id, status FROM deployments WHERE tag_id = ?", tagID).
		Scan(&serviceName, &jobID, &status)
	return serviceName, jobID, status, err
}
