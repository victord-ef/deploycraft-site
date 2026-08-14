package main

import (
	"log"
	"fmt"
	"github.com/victord-ef/deploycraft-site/internal/service"
	"github.com/victord-ef/deploycraft-site/internal/settings"
	"github.com/victord-ef/deploycraft-site/internal/storage"
)

func main() {

	// Open database
	fmt.Println("Database path: data/news.db")
	db, err := storage.New("data/news.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Run database migrations
	if err := db.Migrate(); err != nil {
		log.Fatal(err)
	}

	// Load configuration
	cfg, err := settings.Load("config/configuration.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Run the ingestion service
	if err := service.IngestFeeds(db, cfg); err != nil {
		log.Fatal(err)
	}

	// Display final statistics
	total, err := db.Count()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Total articles in database: %d", total)
}


