package service

import (
	"github.com/victord-ef/deploycraft-site/internal/models"
	"github.com/victord-ef/deploycraft-site/internal/storage"
)

func Latest(
	db *storage.SQLite,
	limit int,
) ([]models.Article, error) {

	return db.Recent(limit)
}
