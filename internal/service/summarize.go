package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/victord-ef/deploycraft-site/internal/models"
	"github.com/victord-ef/deploycraft-site/internal/storage"
)

type articleSummary struct {
	BlogTitle      string `json:"blog_title"`
	Summary        string `json:"summary"`
	Keywords       string `json:"keywords"`
	SEODescription string `json:"seo_description"`
}

func SummarizeSelected(db *storage.SQLite) error {
	articles, err := db.ListByStatus(models.StatusSelected)
	if err != nil {
		return fmt.Errorf("listing selected articles: %w", err)
	}

	if len(articles) == 0 {
		log.Println("No selected articles to summarize.")
		return nil
	}

	client := anthropic.NewClient()

	for _, article := range articles {
		log.Printf("Summarizing: %s", article.Title)

		result, err := summarize(&client, article)
		if err != nil {
			log.Printf("Failed to summarize article %d: %v", article.ID, err)
			continue
		}

		if err := db.UpdateSummary(article.ID, result.BlogTitle, result.Summary, result.Keywords, result.SEODescription); err != nil {
			return fmt.Errorf("saving summary for article %d: %w", article.ID, err)
		}

		log.Printf("Summarized: %s", result.BlogTitle)
	}

	return nil
}

func summarize(client *anthropic.Client, article models.Article) (*articleSummary, error) {
	prompt := fmt.Sprintf(`You are an editorial assistant for DeployCraft.io, a technical platform for DevOps and Kubernetes engineers.

Given the following article, produce a JSON object with these fields:
- blog_title: An SEO-optimized title for DeployCraft's audience (max 70 chars)
- summary: A 200–300 word rewrite of the article suitable for publication. Write in clear, professional technical prose. Focus on what matters to DevOps/Kubernetes/cloud-native engineers. Do not use first person.
- keywords: 5–8 comma-separated lowercase keywords relevant to the article
- seo_description: A compelling meta description for search engines (max 155 chars)

Respond with valid JSON only. No markdown, no code fences, no explanation.

Article title: %s
Article source: %s
Article description: %s`, article.Title, article.Source, article.Description)

	message, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5_20251001,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API: %w", err)
	}

	if len(message.Content) == 0 {
		return nil, fmt.Errorf("empty response from claude")
	}

	raw := message.Content[0].Text

	var result articleSummary
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parsing claude response: %w\nraw: %s", err, raw)
	}

	return &result, nil
}
