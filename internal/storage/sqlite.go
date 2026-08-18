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

	if article.Status == "" {
	article.Status = models.StatusNew
}

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
		        id,
			title,
			link,
			description,
			published,
			source,
			category,
			author,
			status
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
			&article.ID,
			&article.Title,
			&article.Link,
			&article.Description,
			&article.Published,
			&article.Source,
			&article.Category,
			&article.Author,
			&article.Status,
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
}

func (s *SQLite) UpdateStatus(id int64, status string) error {

	_, err := s.db.Exec(
		`
		UPDATE articles
		SET status = ?
		WHERE id = ?
		`,
		status,
		id,
	)

	return err
}

func (s *SQLite) FindByID(id int64) (*models.Article, error) {

	var article models.Article

	err := s.db.QueryRow(
		`
		SELECT
			id,
			title,
			description,
			link,
			published,
			source,
			category,
			author,
			status
		FROM articles
		WHERE id = ?
		`,
		id,
	).Scan(
		&article.ID,
		&article.Title,
		&article.Description,
		&article.Link,
		&article.Published,
		&article.Source,
		&article.Category,
		&article.Author,
		&article.Status,
	)

	if err != nil {
		return nil, err
	}

	return &article, nil
}

func (s *SQLite) UpdateSummary(id int64, blogTitle, summary, keywords, seoDescription string) error {
	_, err := s.db.Exec(
		`UPDATE articles
		 SET blog_title = ?, summary = ?, keywords = ?, seo_description = ?,
		     status = ?, summarized = 1
		 WHERE id = ?`,
		blogTitle, summary, keywords, seoDescription, models.StatusSummarized, id,
	)
	return err
}

func (s *SQLite) ListByStatus(status string) ([]models.Article, error) {

	rows, err := s.db.Query(
		`
		SELECT
			id,
			title,
			description,
			link,
			published,
			source,
			category,
			author,
			status
		FROM articles
		WHERE status = ?
		ORDER BY published DESC
		`,
		status,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article

	for rows.Next() {

		var article models.Article

		err := rows.Scan(
			&article.ID,
			&article.Title,
			&article.Description,
			&article.Link,
			&article.Published,
			&article.Source,
			&article.Category,
			&article.Author,
			&article.Status,
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
}
