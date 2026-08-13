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

	// ----------------------------------------------------
	// Initialize editorial workflow
	// ----------------------------------------------------
	article.Status = "NEW"
	article.Selected = false
	article.Summarized = false
	article.PublishedToBlog = false

	// ----------------------------------------------------
	// Save article
	// ---------------------------------------------------
	_, err := s.db.Exec(
		`
INSERT INTO articles (

	title,
	description,
	link,
	published,
	source,
	category,
	author,
	status,
	selected,
	summarized,
	published_to_blog,
	blog_title,
	summary,
	keywords,
	seo_description

)

VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		article.Title,
		article.Description,
		article.Link,
		article.Published,
		article.Source,
		article.Category,
		article.Author,

		// Editorial workflow
		article.Status,
		article.Selected,
		article.Summarized,
		article.PublishedToBlog,

		// AI fields
		article.BlogTitle,
		article.Summary,
		article.Keywords,
		article.SEODescription,
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

func (s *SQLite) Recent(limit int) ([]models.Article, error) {

	rows, err := s.db.Query(
		`
		SELECT
			title,
			link,
			description,
			published,
			source,
			category,
			author
		FROM articles
		ORDER BY published DESC
		LIMIT ?
		`,
		limit,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article

	for rows.Next() {

		var article models.Article

		err := rows.Scan(
			&article.Title,
			&article.Link,
			&article.Description,
			&article.Published,
			&article.Source,
			&article.Category,
			&article.Author,
		)

		if err != nil {
			return nil, err
		}

		articles = append(articles, article)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return articles, nil

CREATE TABLE IF NOT EXISTS articles(

id INTEGER PRIMARY KEY AUTOINCREMENT,

title TEXT NOT NULL,

description TEXT,

link TEXT NOT NULL UNIQUE,

published DATETIME,

source TEXT,

category TEXT,

author TEXT,

status TEXT DEFAULT 'NEW',

selected INTEGER DEFAULT 0,

summarized INTEGER DEFAULT 0,

published_to_blog INTEGER DEFAULT 0,

blog_title TEXT,

summary TEXT,

keywords TEXT,

seo_description TEXT,

created_at DATETIME DEFAULT CURRENT_TIMESTAMP

);
}
