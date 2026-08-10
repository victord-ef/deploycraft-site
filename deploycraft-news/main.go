package main

import (
	"fmt"
	"log"

	"github.com/victord-ef/deploycraft-site/internal/rss"
	"github.com/victord-ef/deploycraft-site/internal/settings"
	"github.com/victord-ef/deploycraft-site/internal/storage"
	
)

const maxArticles = 25

func main() {

	// ----------------------------------------------------
	// Open SQLite database
	// ----------------------------------------------------
	db, err := storage.New("data/news.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// ----------------------------------------------------
	// Create database tables (if they don't already exist)
	// ----------------------------------------------------
	err = db.Migrate()
	if err != nil {
		log.Fatal(err)
	}

	// ----------------------------------------------------
	// Fetch RSS feed
	// ----------------------------------------------------
	cfg, err := settings.Load("config/configuration.yaml")
	if err != nil {
	    log.Fatal(err)
	}

	// ----------------------------------------------------
	// Save new articles into SQLite
	// ----------------------------------------------------
	for _, feed := range cfg.Feeds {

	fmt.Printf("\n=========================================\n")
	fmt.Printf("Fetching: %s\n", feed.Name)
	fmt.Printf("=========================================\n")

	articles, err := rss.Fetch(feed.URL)
	if err != nil {
		log.Printf("Failed to fetch %s: %v\n", feed.Name, err)
		continue
	}

	fmt.Printf("Found %d articles\n", len(articles))

	saved := 0

	for _, article := range articles {

		exists, err := db.Exists(article.Link)
		if err != nil {
			log.Fatal(err)
		}

		if exists {
			continue
		}

		err = db.Save(article)
		if err != nil {
			log.Fatal(err)
		}

		saved++
	}

	fmt.Printf("New articles saved: %d\n", saved)

	limit := maxArticles
	if len(articles) < limit {
		limit = len(articles)
	}

	fmt.Printf("\nDisplaying first %d articles\n\n", limit)

	for i := 0; i < limit; i++ {

		article := articles[i]

		fmt.Printf("%d. %s\n", i+1, article.Title)
		fmt.Printf("   Source: %s\n", article.Source)
		fmt.Printf("   Published: %s\n", article.Published.Format("2006-01-02 15:04"))
		fmt.Printf("   Link: %s\n", article.Link)
		fmt.Println()
	}
}

	// ----------------------------------------------------
	// Print database statistics
	// ----------------------------------------------------
	total, err := db.Count()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=========================================")
	fmt.Printf("Total articles in database: %d\n", total)
	fmt.Println("=========================================")

}
