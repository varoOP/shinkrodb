package domain

import "context"

// CacheRepo defines the interface for cache database operations
type CacheRepo interface {
	GetAniDBIDs(ctx context.Context) (map[int]int, error)
	GetTMDBIDs(ctx context.Context) (map[int]int, error)
	UpsertEntry(ctx context.Context, entry *CacheEntry) error
	InsertEntry(ctx context.Context, entry *CacheEntry) error
	UpdateTMDBID(ctx context.Context, malID, tmdbID int, releaseDate, animeType string) error
	GetEntriesByReleaseYear(ctx context.Context, year int) ([]*CacheEntry, error)
	DeleteEntry(ctx context.Context, malID int) error
}

// CacheEntry represents a cache entry in the database
type CacheEntry struct {
	MalID       int
	AnidbID     int
	TmdbID      int
	URL         string
	CachedAt    string
	LastUsed    string
	HadAniDBID  bool
	ReleaseDate string
	Type        string
}
