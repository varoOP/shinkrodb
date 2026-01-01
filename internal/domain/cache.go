package domain

import "context"

// CacheRepo defines the interface for cache database operations
type CacheRepo interface {
	// MAL cache operations
	UpsertMAL(ctx context.Context, malID int, url, releaseDate, animeType string) error
	
	// AniDB cache operations
	GetAniDBIDs(ctx context.Context) (map[int]int, error)
	UpsertAniDB(ctx context.Context, malID, anidbID int) error
	
	// TMDB cache operations
	GetTMDBIDs(ctx context.Context) (map[int]int, error)
	UpsertTMDB(ctx context.Context, malID, tmdbID int) error
	
	// Query operations
	GetEntriesByReleaseYear(ctx context.Context, year int) ([]*MALCacheEntry, error)
	DeleteMAL(ctx context.Context, malID int) error
}

// MALCacheEntry represents a MAL cache entry
type MALCacheEntry struct {
	MalID       int
	URL         string
	ReleaseDate string
	Type        string
	CachedAt    string
	LastUsed    string
}
