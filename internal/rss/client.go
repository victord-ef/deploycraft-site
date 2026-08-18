package rss

import (
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/victord-ef/deploycraft-site/internal/models"
)

func Fetch(url string) ([]models.Article, error) {

	parser := gofeed.NewParser()
	parser.Client = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &userAgentTransport{
			ua:   "DeployCraft-News/1.0 (https://deploycraft.io)",
			base: http.DefaultTransport,
		},
	}

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

type userAgentTransport struct {
	ua   string
	base http.RoundTripper
}

func (t *userAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("User-Agent", t.ua)
	r.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/xml, text/xml, */*")
	return t.base.RoundTrip(r)
}
