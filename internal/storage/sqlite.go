package storage

import (
	"database/sql"

	_ "modernc.org/sqlite"

	"github.com/victord-ef/deploycraft-site/internal/models"
)

type SQLite struct {
	db *sql.DB
}

func New(path string) (*SQLite, error) {

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	return &SQLite{
		db: db,
	}, nil
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

func (s *SQLite) Exists(link string) (bool, error) {

	var exists int

	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM articles WHERE link = ?",
		link,
	).Scan(&exists)

	if err != nil {
		return false, err
	}

	return exists > 0, nil
}


func (s *SQLite) Save(article models.Article) error {

	_, err := s.db.Exec(

		`
INSERT INTO articles(

title,

link,

description,

published,

source,

category,

author

)

VALUES(?,?,?,?,?,?,?)

`,

		article.Title,

		article.Link,

		article.Description,

		article.Published,

		article.Source,

		article.Category,

		article.Author,
	)

	return err
}

func (s *SQLite) Count() (int, error) {

	var count int

	err := s.db.QueryRow(

		"SELECT COUNT(*) FROM articles",

	).Scan(&count)

	return count, err
}


