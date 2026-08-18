package service

import (
	"github.com/victord-ef/deploycraft-site/internal/models"
	"github.com/victord-ef/deploycraft-site/internal/storage"
)

// List all articles waiting for editorial review.
func ReviewQueue(db *storage.SQLite) ([]models.Article, error) {
	return db.ListByStatus(models.StatusNew)
}

// Mark an article as selected for publication.
func SelectArticle(db *storage.SQLite, id int64) error {
	return db.UpdateStatus(id, models.StatusSelected)
}

// Reject an article.
func RejectArticle(db *storage.SQLite, id int64) error {
	return db.UpdateStatus(id, models.StatusRejected)
}

// View a single article.
func ReviewArticle(db *storage.SQLite, id int64) (*models.Article, error) {
	return db.FindByID(id)
}
