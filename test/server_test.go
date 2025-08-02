package test

import (
	"database/sql"
	"testing"

	"shipper-deployment/internal/config"
	"shipper-deployment/internal/server"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func TestNewServer(t *testing.T) {
	// Create a test config
	cfg := &config.Config{
		NomadURL:      "http://localhost:4646",
		ValidSecret:   "test-secret",
		Port:          "8080",
		SkipTLSVerify: true,
		NomadToken:    "",
	}

	// Create an in-memory database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a test New Relic app (can be nil)
	var nrApp *newrelic.Application = nil

	s := server.NewServer(cfg, db, nrApp)

	if s == nil {
		t.Error("Expected server to be created, got nil")
	}
}

func TestServerWithNewRelic(t *testing.T) {
	// Create a test config
	cfg := &config.Config{
		NomadURL:      "http://localhost:4646",
		ValidSecret:   "test-secret",
		Port:          "8080",
		SkipTLSVerify: true,
		NomadToken:    "",
	}

	// Create an in-memory database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Try to create a New Relic app for testing (may fail in test environment)
	nrApp, err := newrelic.NewApplication(
		newrelic.ConfigAppName("test-app"),
		newrelic.ConfigLicense("invalid-license-for-testing"),
		newrelic.ConfigEnabled(false), // Disable for testing
	)
	if err != nil {
		t.Logf("Could not create New Relic app for testing: %v", err)
		nrApp = nil
	}

	s := server.NewServer(cfg, db, nrApp)

	if s == nil {
		t.Error("Expected server to be created, got nil")
	}
}
