package storage

func (s *SQLite) Migrate() error {

	query := `
CREATE TABLE IF NOT EXISTS articles (

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
`

	_, err := s.db.Exec(query)

	return err
}
