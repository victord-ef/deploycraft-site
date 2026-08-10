package models

import "time"

type Article struct {
	ID          string
	Title       string
	Link        string
	Description string
	Published   time.Time
	Source      string
	Category    string
	Author      string
}
