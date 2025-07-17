package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB() *sql.DB {
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

func InsertDeployment(db *sql.DB, tagID, serviceName, jobID, status string) error {
	stmt, err := db.Prepare("INSERT INTO deployments (tag_id, service_name, job_id, status) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(tagID, serviceName, jobID, status)
	return err
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
