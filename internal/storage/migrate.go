package storage

func (s *SQLite) Migrate() error {

	query := `

CREATE TABLE IF NOT EXISTS articles(

id INTEGER PRIMARY KEY AUTOINCREMENT,

title TEXT NOT NULL,

link TEXT NOT NULL UNIQUE,

description TEXT,

published DATETIME,

source TEXT,

category TEXT,

author TEXT,

summary TEXT,

created_at DATETIME DEFAULT CURRENT_TIMESTAMP

);

`

	_, err := s.db.Exec(query)

	return err
}
