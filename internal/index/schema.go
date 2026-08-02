package index

// The index is derived, so migrations only ever need to bring an empty database
// up to date — there is no data to preserve. A schema change bumps the version,
// which discards the old index and rebuilds it from YAML.
const schemaVersion = 2

var sqliteSchema = []string{
	`CREATE TABLE IF NOT EXISTS meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS repositories (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		root          TEXT NOT NULL,
		indexed_at    INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS files (
		repository_id TEXT NOT NULL,
		path          TEXT NOT NULL,
		mtime_unix    INTEGER NOT NULL,
		size          INTEGER NOT NULL,
		present       INTEGER NOT NULL,
		PRIMARY KEY (repository_id, path)
	)`,
	`CREATE TABLE IF NOT EXISTS annotations (
		id            TEXT NOT NULL,
		repository_id TEXT NOT NULL,
		path          TEXT NOT NULL,
		kind          TEXT NOT NULL,
		scope         TEXT NOT NULL,
		body          TEXT NOT NULL,
		excerpt       TEXT NOT NULL DEFAULT '',
		created       TEXT NOT NULL,
		author_name   TEXT NOT NULL DEFAULT '',
		author_kind   TEXT NOT NULL DEFAULT '',
		git_commit    TEXT NOT NULL DEFAULT '',
		status        TEXT NOT NULL DEFAULT '',
		line          INTEGER NOT NULL DEFAULT 0,
		excerpt_sha256  TEXT NOT NULL DEFAULT '',
		last_seen_line  INTEGER NOT NULL DEFAULT 0,
		git_path        TEXT NOT NULL DEFAULT '',
		git_line        INTEGER NOT NULL DEFAULT 0,
		git_end_line    INTEGER NOT NULL DEFAULT 0,
		author_email    TEXT NOT NULL DEFAULT '',
		author_source   TEXT NOT NULL DEFAULT '',
		author_account  TEXT NOT NULL DEFAULT '',
		author_verified TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (repository_id, id)
	)`,
	`CREATE INDEX IF NOT EXISTS annotations_by_path ON annotations (repository_id, path)`,
	`CREATE INDEX IF NOT EXISTS annotations_by_status ON annotations (repository_id, status)`,
	`CREATE INDEX IF NOT EXISTS annotations_by_kind ON annotations (repository_id, kind)`,
	// FTS5 is what makes koment_search a query rather than a scan over every
	// body. Verified present in modernc.org/sqlite (ADR 0022).
	`CREATE VIRTUAL TABLE IF NOT EXISTS annotation_search USING fts5(
		body,
		annotation_id UNINDEXED,
		repository_id UNINDEXED,
		tokenize = 'porter unicode61'
	)`,
}

var postgresSchema = []string{
	`CREATE TABLE IF NOT EXISTS meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS repositories (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		root       TEXT NOT NULL,
		indexed_at BIGINT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS files (
		repository_id TEXT NOT NULL,
		path          TEXT NOT NULL,
		mtime_unix    BIGINT NOT NULL,
		size          BIGINT NOT NULL,
		present       BOOLEAN NOT NULL,
		PRIMARY KEY (repository_id, path)
	)`,
	`CREATE TABLE IF NOT EXISTS annotations (
		id            TEXT NOT NULL,
		repository_id TEXT NOT NULL,
		path          TEXT NOT NULL,
		kind          TEXT NOT NULL,
		scope         TEXT NOT NULL,
		body          TEXT NOT NULL,
		excerpt       TEXT NOT NULL DEFAULT '',
		created       TEXT NOT NULL,
		author_name   TEXT NOT NULL DEFAULT '',
		author_kind   TEXT NOT NULL DEFAULT '',
		git_commit    TEXT NOT NULL DEFAULT '',
		status        TEXT NOT NULL DEFAULT '',
		line          INTEGER NOT NULL DEFAULT 0,
		excerpt_sha256  TEXT NOT NULL DEFAULT '',
		last_seen_line  INTEGER NOT NULL DEFAULT 0,
		git_path        TEXT NOT NULL DEFAULT '',
		git_line        INTEGER NOT NULL DEFAULT 0,
		git_end_line    INTEGER NOT NULL DEFAULT 0,
		author_email    TEXT NOT NULL DEFAULT '',
		author_source   TEXT NOT NULL DEFAULT '',
		author_account  TEXT NOT NULL DEFAULT '',
		author_verified TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (repository_id, id)
	)`,
	`CREATE INDEX IF NOT EXISTS annotations_by_path ON annotations (repository_id, path)`,
	`CREATE INDEX IF NOT EXISTS annotations_by_status ON annotations (repository_id, status)`,
	`CREATE INDEX IF NOT EXISTS annotations_by_kind ON annotations (repository_id, kind)`,
	// Postgres has no FTS5; a generated tsvector with a GIN index is the
	// equivalent, and it means Search has one shape per driver rather than one
	// query that is mediocre on both.
	`ALTER TABLE annotations ADD COLUMN IF NOT EXISTS body_search tsvector
		GENERATED ALWAYS AS (to_tsvector('english', body)) STORED`,
	`CREATE INDEX IF NOT EXISTS annotations_body_search ON annotations USING GIN (body_search)`,
}
