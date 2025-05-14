package main

import (
	"distributed-db/shared"
	"log"
)

func main() {
	log.Printf("Initializing database connection...")
	config := shared.NewDBConfig("Kaido440", "5277859MoKaido!", "127.0.0.1", "3307")
	dbHandler, err := shared.NewDBHandler(config)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	log.Printf("Database connection initialized successfully")

	StartWebServer(dbHandler)
}
