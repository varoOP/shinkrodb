package domain

// FetchMode defines the fetching behavior for IDs
type FetchMode string

const (
	// FetchModeDefault - Default behavior (AniDB: only scrape for MAL IDs without AniDB ID, released in past 1 year, type = "tv"; TMDB: only fetch for movies without TMDB ID)
	FetchModeDefault FetchMode = "default"
	// FetchModeMissing - Fetch all entries without ID (no filters)
	FetchModeMissing FetchMode = "missing"
	// FetchModeAll - Fetch everything, even if already has ID in cache
	FetchModeAll FetchMode = "all"
	// FetchModeSkip - Skip fetching entirely
	FetchModeSkip FetchMode = "skip"
)

type Config struct {
	MalClientID string     `json:"mal-client-id"`
	TmdbApiKey  string     `json:"tmdb-api-key"`
	AniDBMode   FetchMode  `json:"anidb-mode"`
	TMDBMode    FetchMode  `json:"tmdb-mode"`
}
