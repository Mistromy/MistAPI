package projects

import (
	"database/sql"
	"fmt"
	"time"

	"charm.land/log/v2"
	_ "modernc.org/sqlite"
)

var db *sql.DB

// Init sets up the database for the site
func Init() error {
	log.Info("Loading DataBase Module")
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
    source        TEXT    NOT NULL, -- artStation if scraped or manual if uploaded by hand
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

const upsertWork string = `INSERT INTO works (source, source_id, title, description,
                   source_url, published_at, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (source, source_id) DO UPDATE SET 
    title = excluded.title,
    description = excluded.description,
    source_url   = excluded.source_url,
    published_at = excluded.published_at,
    last_seen_at = excluded.last_seen_at
RETURNING id, first_seen_at = last_seen_at;`

func AddArtwork(works []Work) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func(tx *sql.Tx) {
		err := tx.Rollback()
		if err != nil {
			return
		}
	}(tx)

	for _, w := range works {
		if _, err := tx.Exec(upsertWork, w.Source, w.SourceID, w.Title, w.Description, w.SourceURL, w.PublishedAt.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type Work struct {
	Source      string // "artstation" or "manual"
	SourceID    string
	Title       string
	Description string
	SourceURL   string
	PublishedAt time.Time
	Images      []Image
}

type Image struct {
	SourceID string
	URL      string
	Caption  string
}

const (
	SourceTypeArtstation string = "Artstation"
	//SourceTypeManual     string = "manual"
)
