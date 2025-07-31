package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	NomadURL         string
	ValidSecret      string
	Port             string
	ValidServices    []string
	SkipTLSVerify    bool
	NomadToken       string
	NewRelicLicense  string
	NewRelicAppName  string
	NewRelicEnabled  bool
}

func Load() *Config {
	validServicesStr := getEnv("VALID_SERVICES", "")
	var validServices []string
	if validServicesStr != "" {
		validServices = strings.Split(validServicesStr, ",")
		// Trim whitespace from each service name
		for i, service := range validServices {
			validServices[i] = strings.TrimSpace(service)
		}
	}

	skipTLSVerifyStr := getEnv("SKIP_TLS_VERIFY", "true")
	skipTLSVerify, err := strconv.ParseBool(skipTLSVerifyStr)
	if err != nil {
		skipTLSVerify = false
	}

	newRelicEnabledStr := getEnv("NEW_RELIC_ENABLED", "false")
	newRelicEnabled, err := strconv.ParseBool(newRelicEnabledStr)
	if err != nil {
		newRelicEnabled = false
	}

	return &Config{
		NomadURL:         getEnv("NOMAD_URL", "https://10.10.85.1:4646"),
		ValidSecret:      getEnv("RPC_SECRET", "your-64-character-secret-key-here-please-change-this-in-production"),
		Port:             getEnv("PORT", "16166"),
		ValidServices:    validServices,
		SkipTLSVerify:    skipTLSVerify,
		NomadToken:       getEnv("NOMAD_TOKEN", "e26c5903-27f8-4c10-7d91-1d5b5b022c89"),
		NewRelicLicense:  getEnv("NEW_RELIC_LICENSE_KEY", ""),
		NewRelicAppName:  getEnv("NEW_RELIC_APP_NAME", "shipper-deployment"),
		NewRelicEnabled:  newRelicEnabled,
	}
}

// IsValidService checks if a service name is in the list of valid services
func (c *Config) IsValidService(serviceName string) bool {
	// If no valid services are configured, allow all services
	if len(c.ValidServices) == 0 {
		return true
	}

	for _, validService := range c.ValidServices {
		if validService == serviceName {
			return true
		}
	}
	return false
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
