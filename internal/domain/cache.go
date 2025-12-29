package domain

import "context"

// CacheRepo defines the interface for cache database operations
type CacheRepo interface {
	GetAniDBIDs(ctx context.Context) (map[int]int, error)
	UpsertEntry(ctx context.Context, entry *CacheEntry) error
	InsertEntry(ctx context.Context, entry *CacheEntry) error
	GetEntriesByReleaseYear(ctx context.Context, year int) ([]*CacheEntry, error)
	DeleteEntry(ctx context.Context, malID int) error
}

// CacheEntry represents a cache entry in the database
type CacheEntry struct {
	MalID       int
	AnidbID     int
	URL         string
	CachedAt    string
	LastUsed    string
	HadAniDBID  bool
	ReleaseDate string
	Type        string
}
