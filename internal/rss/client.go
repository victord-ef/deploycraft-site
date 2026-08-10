package rss

import (
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/victord-ef/deploycraft-site/internal/models"
)

func Fetch(url string) ([]models.Article, error) {

	parser := gofeed.NewParser()

	feed, err := parser.ParseURL(url)
	if err != nil {
		return nil, err
	}

	articles := []models.Article{}

	for _, item := range feed.Items {

		article := models.Article{
			Title:       item.Title,
			Link:        item.Link,
			Description: item.Description,
			Source:      feed.Title,
		}

		if item.PublishedParsed != nil {
			article.Published = *item.PublishedParsed
		} else {
			article.Published = time.Now()
		}

		articles = append(articles, article)
	}

	return articles, nil
}
