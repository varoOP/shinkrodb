package cache

const schema = `
CREATE TABLE cache_entries (
	mal_id INTEGER PRIMARY KEY,
	anidb_id INTEGER NOT NULL DEFAULT 0,
	url TEXT NOT NULL,
	cached_at TIMESTAMP NOT NULL,
	last_used TIMESTAMP NOT NULL,
	html_hash TEXT,
	had_anidb_id BOOLEAN NOT NULL DEFAULT 0,
	release_date TEXT,
	type TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_cached_at ON cache_entries(cached_at);
CREATE INDEX idx_last_used ON cache_entries(last_used);
CREATE INDEX idx_anidb_id ON cache_entries(anidb_id);
CREATE INDEX idx_had_anidb_id ON cache_entries(had_anidb_id);
CREATE INDEX idx_release_date ON cache_entries(release_date);
CREATE INDEX idx_type ON cache_entries(type);
`

// migrations contains incremental schema changes
// Each migration is applied in order based on the current user_version
// migrations[0] is empty because version 0 uses the base schema
var migrations = []string{
	"", // Version 0 is the base schema, so migrations[0] is empty
	// Future migrations go here, e.g.:
	// `-- Migration 1: Example future migration
	// ALTER TABLE cache_entries ADD COLUMN new_field TEXT;`,
}
