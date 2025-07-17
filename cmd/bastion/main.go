package main

import (
	"log"
	"os"

	"bastion-deployment/internal/config"
	"bastion-deployment/internal/database"
	"bastion-deployment/internal/server"
)

func init() {
	// Configure logging to output to stdout with timestamp and file information
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	log.Println("Starting Bastion Deployment Service")
	
	// Load configuration
	cfg := config.Load()
	log.Println("Configuration loaded successfully")

	// Initialize database
	db := database.InitDB()
	defer db.Close()
	log.Println("Database initialized successfully")

	// Create and start server
	srv := server.NewServer(cfg, db)
	log.Printf("Server starting on port %s", cfg.Port)
	if err := srv.Start(); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
