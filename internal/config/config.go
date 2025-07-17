package config

import (
	"os"
	"strings"
)

type Config struct {
	NomadURL      string
	ValidSecret   string
	Port          string
	ValidServices []string
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

	return &Config{
		NomadURL:      getEnv("NOMAD_URL", "https://10.10.85.1:4646"),
		ValidSecret:   getEnv("VALID_SECRET", "your-64-character-secret-key-here-please-change-this-in-production"),
		Port:          getEnv("PORT", "16166"),
		ValidServices: validServices,
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
