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
	
	articles, err := service.Latest(db, 10)
	if err != nil {
	    log.Fatal(err)
}

	fmt.Println()
	fmt.Println("Latest Articles")
	fmt.Println("==============================")

	for i, article := range articles {


	     fmt.Printf("ID: %d\n", article.ID)
	     fmt.Printf("Title: %s\n", article.Title)

	     fmt.Printf("   Source: %s\n", article.Source)
	     fmt.Printf("   Published: %s\n", article.Published.Format("2006-01-02 15:04"))
	     fmt.Printf("   Status: %s\n", article.Status)
	     fmt.Printf("   Link: %s\n", article.Link)
	     fmt.Println()
}


	log.Printf("Total articles in database: %d", total)
}


