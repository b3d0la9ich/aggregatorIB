package main

import (
	"log"
	"net/http"
	"os"

	"aggregatorIB/internal/db"
	apphttp "aggregatorIB/internal/http"
)

func main() {
	database, err := db.Open()
	if err != nil {
		log.Fatalf("database error: %v", err)
	}

	app := apphttp.New(database)
	addr := envOrDefault("APP_PORT", "8080")

	log.Printf("server started on :%s", addr)
	if err := http.ListenAndServe(":"+addr, app.Routes()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
