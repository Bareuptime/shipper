package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"NOMAD_URL", "RPC_SECRET", "PORT", "SKIP_TLS_VERIFY",
		"NOMAD_TOKEN", "NEW_RELIC_LICENSE_KEY", "NEW_RELIC_APP_NAME", "NEW_RELIC_ENABLED",
	}
	
	for _, key := range envVars {
		originalEnv[key] = os.Getenv(key)
	}

	// Cleanup function to restore environment
	cleanup := func() {
		for _, key := range envVars {
			if original, exists := originalEnv[key]; exists {
				os.Setenv(key, original)
			} else {
				os.Unsetenv(key)
			}
		}
	}
	defer cleanup()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name: "default values",
			envVars: map[string]string{
				// Clear all env vars
				"NOMAD_URL":               "",
				"RPC_SECRET":              "",
				"PORT":                    "",
				"SKIP_TLS_VERIFY":         "",
				"NOMAD_TOKEN":             "",
				"NEW_RELIC_LICENSE_KEY":   "",
				"NEW_RELIC_APP_NAME":      "",
				"NEW_RELIC_ENABLED":       "",
			},
			expected: &Config{
				NomadURL:        "https://10.10.85.1:4646",
				ValidSecret:     "your-64-character-secret-key-here-please-change-this-in-production",
				Port:            "16166",
				SkipTLSVerify:   true,
				NomadToken:      "e26c5903-27f8-4c10-7d91-1d5b5b022c89",
				NewRelicLicense: "",
				NewRelicAppName: "shipper-deployment",
				NewRelicEnabled: false,
			},
		},
		{
			name: "custom values",
			envVars: map[string]string{
				"NOMAD_URL":               "https://custom-nomad:4646",
				"RPC_SECRET":              "custom-secret-key-64-chars-long-production-ready-secret",
				"PORT":                    "8080",
				"SKIP_TLS_VERIFY":         "false",
				"NOMAD_TOKEN":             "custom-token-123",
				"NEW_RELIC_LICENSE_KEY":   "custom-license",
				"NEW_RELIC_APP_NAME":      "custom-app",
				"NEW_RELIC_ENABLED":       "true",
			},
			expected: &Config{
				NomadURL:        "https://custom-nomad:4646",
				ValidSecret:     "custom-secret-key-64-chars-long-production-ready-secret",
				Port:            "8080",
				SkipTLSVerify:   false,
				NomadToken:      "custom-token-123",
				NewRelicLicense: "custom-license",
				NewRelicAppName: "custom-app",
				NewRelicEnabled: true,
			},
		},
		{
			name: "invalid boolean values",
			envVars: map[string]string{
				"SKIP_TLS_VERIFY":   "invalid",
				"NEW_RELIC_ENABLED": "not-a-bool",
			},
			expected: &Config{
				NomadURL:        "https://10.10.85.1:4646",
				ValidSecret:     "your-64-character-secret-key-here-please-change-this-in-production",
				Port:            "16166",
				SkipTLSVerify:   false, // Should default to false on parse error
				NomadToken:      "e26c5903-27f8-4c10-7d91-1d5b5b022c89",
				NewRelicLicense: "",
				NewRelicAppName: "shipper-deployment",
				NewRelicEnabled: false, // Should default to false on parse error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables for this test
			for key, value := range tt.envVars {
				if value == "" {
					os.Unsetenv(key)
				} else {
					os.Setenv(key, value)
				}
			}

			config := Load()

			if config.NomadURL != tt.expected.NomadURL {
				t.Errorf("NomadURL = %v, want %v", config.NomadURL, tt.expected.NomadURL)
			}

			if config.ValidSecret != tt.expected.ValidSecret {
				t.Errorf("ValidSecret = %v, want %v", config.ValidSecret, tt.expected.ValidSecret)
			}

			if config.Port != tt.expected.Port {
				t.Errorf("Port = %v, want %v", config.Port, tt.expected.Port)
			}

			if config.SkipTLSVerify != tt.expected.SkipTLSVerify {
				t.Errorf("SkipTLSVerify = %v, want %v", config.SkipTLSVerify, tt.expected.SkipTLSVerify)
			}

			if config.NomadToken != tt.expected.NomadToken {
				t.Errorf("NomadToken = %v, want %v", config.NomadToken, tt.expected.NomadToken)
			}

			if config.NewRelicLicense != tt.expected.NewRelicLicense {
				t.Errorf("NewRelicLicense = %v, want %v", config.NewRelicLicense, tt.expected.NewRelicLicense)
			}

			if config.NewRelicAppName != tt.expected.NewRelicAppName {
				t.Errorf("NewRelicAppName = %v, want %v", config.NewRelicAppName, tt.expected.NewRelicAppName)
			}

			if config.NewRelicEnabled != tt.expected.NewRelicEnabled {
				t.Errorf("NewRelicEnabled = %v, want %v", config.NewRelicEnabled, tt.expected.NewRelicEnabled)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "environment variable exists",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "environment variable does not exist",
			key:          "NONEXISTENT_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "empty environment variable",
			key:          "EMPTY_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			original := os.Getenv(tt.key)
			defer func() {
				if original == "" {
					os.Unsetenv(tt.key)
				} else {
					os.Setenv(tt.key, original)
				}
			}()

			// Set test value
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("getEnv(%q, %q) = %v, want %v", tt.key, tt.defaultValue, result, tt.expected)
			}
		})
	}
}
