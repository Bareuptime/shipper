package main

import (
	"log"

	"bastion-deployment/internal/config"
	"bastion-deployment/internal/database"
	"bastion-deployment/internal/server"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db := database.InitDB()
	defer db.Close()

	// Create and start server
	srv := server.NewServer(cfg, db)
	if err := srv.Start(); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
