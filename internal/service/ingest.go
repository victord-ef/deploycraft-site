package service

import (
	"log"
	"time"

	"github.com/victord-ef/deploycraft-site/internal/rss"
	"github.com/victord-ef/deploycraft-site/internal/settings"
	"github.com/victord-ef/deploycraft-site/internal/storage"
)

var earliest2026 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func IngestFeeds(db *storage.SQLite, cfg *settings.Configuration) error {

	for _, feed := range cfg.Feeds {

		log.Printf("Fetching %s...", feed.Name)

		articles, err := rss.Fetch(feed.URL)
		if err != nil {
			log.Printf("Failed to fetch %s: %v", feed.Name, err)
			continue
		}

		saved := 0

		for _, article := range articles {

			if article.Published.Before(earliest2026) {
				continue
			}

			article.Source = feed.Name

			exists, err := db.Exists(article.Link)
			if err != nil {
				return err
			}

			if exists {
				continue
			}

			result := Evaluate(article)
			if !result.Newsworthy {
				continue
			}

			if err := db.Save(article); err != nil {
				return err
			}

			saved++
		}

		log.Printf("%s: %d new articles saved", feed.Name, saved)
	}

	return nil
}
