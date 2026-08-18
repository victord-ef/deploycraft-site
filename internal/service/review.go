package service

import (
	"github.com/victord-ef/deploycraft-site/internal/models"
	"github.com/victord-ef/deploycraft-site/internal/storage"
)

// List all articles waiting for editorial review.
func ReviewQueue(db *storage.SQLite) ([]models.Article, error) {
	return db.ListByStatus(models.StatusNew)
}

// List articles waiting for review filtered by source name (partial match).
func FilteredQueue(db *storage.SQLite, source string) ([]models.Article, error) {
	return db.ListByStatusAndSource(models.StatusNew, source)
}

// Delete all NEW and REJECTED articles so their links are freed for re-ingestion.
// SELECTED and SUMMARIZED articles are preserved.
func PurgeQueue(db *storage.SQLite) (int64, error) {
	return db.DeleteUnreviewed()
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
