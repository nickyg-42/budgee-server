package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	DatabaseURL      string
	PlaidClientID    string
	PlaidSecret      string
	PlaidEnvironment string
	IsDemo           bool
}

func Load() Config {
	// Load .env file if present
	_ = godotenv.Load()

	cfg := Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		PlaidClientID:    getEnv("PLAID_CLIENT_ID", ""),
		PlaidSecret:      getEnv("PLAID_SECRET", ""),
		PlaidEnvironment: getEnv("PLAID_ENVIRONMENT", "sandbox"),
		IsDemo:           getEnv("IS_DEMO", "false") == "true",
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if cfg.PlaidClientID == "" {
		log.Fatal("PLAID_CLIENT_ID is required")
	}
	if cfg.PlaidSecret == "" {
		log.Fatal("PLAID_SECRET is required")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
