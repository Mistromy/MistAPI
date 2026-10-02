package projects

import (
	"database/sql"
	"fmt"

	"charm.land/log/v2"
	_ "modernc.org/sqlite"
)

var db *sql.DB

// Init sets up the database for the site
func Init() error {
	var err error
	db, err = sql.Open("sqlite", "site.db")
	if err != nil {
		return fmt.Errorf("database open: %w", err)
	}
	err = db.Ping()
	if err != nil {
		return fmt.Errorf("DB check: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS works
(
    id            INTEGER PRIMARY KEY,
    source        TEXT    NOT NULL, -- artstation if scraped or manual if uploaded by hand
    title         TEXT    NOT NULL,
    source_id     TEXT    NOT NULL,
    description   TEXT    NOT NULL DEFAULT '',
    source_url    TEXT    NOT NULL DEFAULT '',
    published_at  TEXT    NOT NULL,
    first_seen_at TEXT    NOT NULL, -- when the importer first saw it
    last_seen_at  TEXT    NOT NULL, -- last time it was in the feed
    hidden        INTEGER NOT NULL DEFAULT 0,
    UNIQUE (source, source_id)
) STRICT;

CREATE TABLE IF NOT EXISTS images
(
    id         INTEGER PRIMARY KEY,
    work_id    INTEGER NOT NULL REFERENCES works (id),
    source_id  TEXT    NOT NULL, -- ArtStation image id, '102099512'
    position   INTEGER NOT NULL, -- display order
    source_url TEXT    NOT NULL,
    caption    TEXT    NOT NULL DEFAULT '',
    sha256     TEXT,             -- NULL until downloaded
    width      INTEGER,
    height     INTEGER,
    UNIQUE (work_id, source_id)
) STRICT;`)
	if err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

func Test() {
	err := db.Ping()
	if err != nil {
		log.Debug("Database Ping", "error", err)
		return
	}
	log.Debug("ok")
}
