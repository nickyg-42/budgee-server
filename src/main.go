package main

import (
	"budgee-server/src/api"
	"budgee-server/src/config"
	"budgee-server/src/db"
	sql "budgee-server/src/db"
	plaidclient "budgee-server/src/plaid"
	"log"
	"net/http"
	"os"
)

func main() {
	cfg := config.Load()

	logFile, err := os.OpenFile("/var/log/budgee-api.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: Failed to open log file, defaulting to stdout: %v", err)
	} else {
		defer logFile.Close()
		log.SetOutput(logFile)
		log.Println("Log file attached...")
	}

	// Connect to database
	pool, err := sql.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer pool.Close()

	// Initialize Cache
	db.InitCache()

	// Initialize Plaid Client
	plaidClient := plaidclient.NewPlaidClient(cfg.PlaidClientID, cfg.PlaidSecret, cfg.PlaidEnvironment)

	// Router
	router := api.NewRouter(pool, plaidClient, cfg.PlaidEnvironment)

	log.Println("API server running on port", cfg.Port)
	if err := http.ListenAndServe("127.0.0.1:"+cfg.Port, router); err != nil {
		log.Fatal(err)
	}
}
