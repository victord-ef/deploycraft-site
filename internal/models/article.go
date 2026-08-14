package models

import "time"

type Article struct {
		// Database
	ID int64

	// Original RSS article
	Title       string
	Description string
	Link        string
	Published   time.Time
	Source      string
	Category    string
	Author      string

	// Editorial workflow
	Status string

	Selected bool

	Summarized bool

	PublishedToBlog bool

	// AI-generated content
	BlogTitle string

	Summary string

	Keywords string

	SEODescription string

	CreatedAt time.Time
}
