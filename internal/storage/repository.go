package storage

import "github.com/victord-ef/deploycraft-site/internal/models"

type Repository interface {

    Save(article models.Article) error

    Exists(link string) (bool, error)

    Recent(limit int) ([]models.Article, error)

    Count() (int, error)

    Close() error
}
