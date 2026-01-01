package database

const cacheSchema = `
-- Base table for MAL IDs with metadata
CREATE TABLE mal_cache (
	mal_id INTEGER PRIMARY KEY,
	url TEXT NOT NULL,
	release_date TEXT,
	type TEXT,
	cached_at TIMESTAMP NOT NULL,
	last_used TIMESTAMP NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_mal_cached_at ON mal_cache(cached_at);
CREATE INDEX idx_mal_last_used ON mal_cache(last_used);
CREATE INDEX idx_mal_release_date ON mal_cache(release_date);
CREATE INDEX idx_mal_type ON mal_cache(type);

-- AniDB cache table
CREATE TABLE anidb_cache (
	mal_id INTEGER PRIMARY KEY,
	anidb_id INTEGER NOT NULL,
	had_anidb_id BOOLEAN NOT NULL DEFAULT 1,
	cached_at TIMESTAMP NOT NULL,
	last_used TIMESTAMP NOT NULL,
	FOREIGN KEY (mal_id) REFERENCES mal_cache(mal_id) ON DELETE CASCADE
);

CREATE INDEX idx_anidb_id ON anidb_cache(anidb_id);
CREATE INDEX idx_anidb_cached_at ON anidb_cache(cached_at);

-- TMDB cache table
CREATE TABLE tmdb_cache (
	mal_id INTEGER PRIMARY KEY,
	tmdb_id INTEGER NOT NULL,
	has_tmdb_id BOOLEAN NOT NULL DEFAULT 1,
	cached_at TIMESTAMP NOT NULL,
	last_used TIMESTAMP NOT NULL,
	FOREIGN KEY (mal_id) REFERENCES mal_cache(mal_id) ON DELETE CASCADE
);

CREATE INDEX idx_tmdb_id ON tmdb_cache(tmdb_id);
CREATE INDEX idx_tmdb_cached_at ON tmdb_cache(cached_at);
`

// cacheMigrations contains incremental schema changes
// Each migration is applied in order based on the current user_version
// cacheMigrations[0] is empty because version 0 uses the base schema
var cacheMigrations = []string{
	"",
}
