package main

import (
	"budgee-server/src/api"
	"budgee-server/src/config"
	"budgee-server/src/db"
	"log"
	"net/http"
)

func main() {
	cfg := config.Load()

	// Connect to database
	pool, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer pool.Close()

	// Router
	router := api.NewRouter(pool)

	log.Println("API server running on port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal(err)
	}
}
