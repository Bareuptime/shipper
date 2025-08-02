package test

import (
	"os"
	"testing"

	"shipper-deployment/internal/config"
)

func TestConfigLoad(t *testing.T) {
	// Save original environment
	originalValues := map[string]string{
		"NOMAD_URL":             os.Getenv("NOMAD_URL"),
		"RPC_SECRET":            os.Getenv("RPC_SECRET"),
		"PORT":                  os.Getenv("PORT"),
		"NOMAD_TOKEN":           os.Getenv("NOMAD_TOKEN"),
		"SKIP_TLS_VERIFY":       os.Getenv("SKIP_TLS_VERIFY"),
		"NEW_RELIC_ENABLED":     os.Getenv("NEW_RELIC_ENABLED"),
		"NEW_RELIC_LICENSE_KEY": os.Getenv("NEW_RELIC_LICENSE_KEY"),
		"NEW_RELIC_APP_NAME":    os.Getenv("NEW_RELIC_APP_NAME"),
	}

	// Clean up function to restore original environment
	cleanup := func() {
		for key, value := range originalValues {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}
	defer cleanup()

	t.Run("default values", func(t *testing.T) {
		// Clear all environment variables
		for key := range originalValues {
			os.Unsetenv(key)
		}

		cfg := config.Load()

		if cfg.NomadURL != "https://10.10.85.1:4646" {
			t.Errorf("NomadURL = %v, want %v", cfg.NomadURL, "https://10.10.85.1:4646")
		}

		if cfg.Port != "16166" {
			t.Errorf("Port = %v, want %v", cfg.Port, "16166")
		}

		if cfg.SkipTLSVerify != true {
			t.Errorf("SkipTLSVerify = %v, want %v", cfg.SkipTLSVerify, true)
		}

		if cfg.NewRelicEnabled != false {
			t.Errorf("NewRelicEnabled = %v, want %v", cfg.NewRelicEnabled, false)
		}

		if cfg.NewRelicAppName != "shipper-deployment" {
			t.Errorf("NewRelicAppName = %v, want %v", cfg.NewRelicAppName, "shipper-deployment")
		}
	})

	t.Run("custom values", func(t *testing.T) {
		// Set custom environment variables
		os.Setenv("NOMAD_URL", "https://custom-nomad:4646")
		os.Setenv("RPC_SECRET", "custom-secret-key-64-characters-long-for-testing-purposes")
		os.Setenv("PORT", "8080")
		os.Setenv("NOMAD_TOKEN", "test-token")
		os.Setenv("SKIP_TLS_VERIFY", "false")
		os.Setenv("NEW_RELIC_ENABLED", "true")
		os.Setenv("NEW_RELIC_LICENSE_KEY", "test-license-key")
		os.Setenv("NEW_RELIC_APP_NAME", "test-app")

		cfg := config.Load()

		if cfg.NomadURL != "https://custom-nomad:4646" {
			t.Errorf("NomadURL = %v, want %v", cfg.NomadURL, "https://custom-nomad:4646")
		}

		if cfg.ValidSecret != "custom-secret-key-64-characters-long-for-testing-purposes" {
			t.Errorf("ValidSecret = %v, want %v", cfg.ValidSecret, "custom-secret-key-64-characters-long-for-testing-purposes")
		}

		if cfg.Port != "8080" {
			t.Errorf("Port = %v, want %v", cfg.Port, "8080")
		}

		if cfg.NomadToken != "test-token" {
			t.Errorf("NomadToken = %v, want %v", cfg.NomadToken, "test-token")
		}

		if cfg.SkipTLSVerify != false {
			t.Errorf("SkipTLSVerify = %v, want %v", cfg.SkipTLSVerify, false)
		}

		if cfg.NewRelicEnabled != true {
			t.Errorf("NewRelicEnabled = %v, want %v", cfg.NewRelicEnabled, true)
		}

		if cfg.NewRelicLicense != "test-license-key" {
			t.Errorf("NewRelicLicense = %v, want %v", cfg.NewRelicLicense, "test-license-key")
		}

		if cfg.NewRelicAppName != "test-app" {
			t.Errorf("NewRelicAppName = %v, want %v", cfg.NewRelicAppName, "test-app")
		}
	})

	t.Run("invalid boolean values", func(t *testing.T) {
		os.Setenv("SKIP_TLS_VERIFY", "invalid")
		os.Setenv("NEW_RELIC_ENABLED", "invalid")

		cfg := config.Load()

		// Should default to false for invalid boolean values
		if cfg.SkipTLSVerify != false {
			t.Errorf("SkipTLSVerify = %v, want %v", cfg.SkipTLSVerify, false)
		}

		if cfg.NewRelicEnabled != false {
			t.Errorf("NewRelicEnabled = %v, want %v", cfg.NewRelicEnabled, false)
		}
	})
}
