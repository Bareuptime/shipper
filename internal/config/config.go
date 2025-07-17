package config

import "os"

type Config struct {
	NomadURL    string
	ValidSecret string
	Port        string
}

func Load() *Config {
	return &Config{
		NomadURL:    getEnv("NOMAD_URL", "http://10.10.85.1:4646"),
		ValidSecret: getEnv("VALID_SECRET", "your-64-character-secret-key-here-please-change-this-in-production"),
		Port:        getEnv("PORT", "8080"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
