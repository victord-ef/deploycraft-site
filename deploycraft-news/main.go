package main

import (
	"log"

	"github.com/victord-ef/deploycraft-site/internal/service"
	"github.com/victord-ef/deploycraft-site/internal/settings"
	"github.com/victord-ef/deploycraft-site/internal/storage"
)

func main() {
	db, err := storage.New("deploycraft-news/news.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Fatal(err)
	}

	cfg, err := settings.Load("config/configuration.yaml")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("--- Ingesting feeds ---")
	if err := service.IngestFeeds(db, cfg); err != nil {
		log.Fatal(err)
	}

	log.Println("--- Summarizing selected articles ---")
	if err := service.SummarizeSelected(db); err != nil {
		log.Fatal(err)
	}

	total, err := db.Count()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Done. Total articles in database: %d", total)
}
