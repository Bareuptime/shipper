package main

import (
	"bastion-deployment/internal/config"
	"bastion-deployment/internal/database"
	"bastion-deployment/internal/logger"
	"bastion-deployment/internal/server"
)

func main() {
	// Initialize global logger
	appLogger := logger.Initialize()
	appLogger.Info("Bastion Deployment Service starting")

	// Load configuration
	cfg := config.Load()

	// Initialize database
	db := database.InitDB()
	defer db.Close()

	// Create and start server
	srv := server.NewServer(cfg, db)
	if err := srv.Start(); err != nil {
		appLogger.Fatal("Server failed to start:", err)
	}
}
