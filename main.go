package main

import (
	"shipper-deployment/internal/config"
	"shipper-deployment/internal/database"
	"shipper-deployment/internal/logger"
	"shipper-deployment/internal/newrelic"
	"shipper-deployment/internal/server"
)

func main() {
	// Initialize global logger
	appLogger := logger.Initialize()
	appLogger.Info("shipper Deployment Service starting")

	// Load configuration
	cfg := config.Load()

	// Initialize New Relic monitoring
	nrApp, err := newrelic.Initialize(cfg)
	if err != nil {
		appLogger.WithError(err).Warn("Failed to initialize New Relic, continuing without monitoring")
	}

	// Initialize database
	db := database.InitDB()
	defer db.Close()

	// Create and start server with New Relic app
	srv := server.NewServer(cfg, db, nrApp)
	if err := srv.Start(); err != nil {
		appLogger.Fatal("Server failed to start:", err)
	}
}
