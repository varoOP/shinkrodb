package domain

// ScrapeMode defines the scraping behavior for AniDB IDs
type ScrapeMode string

const (
	// ScrapeModeDefault - Only scrape for MAL IDs without AniDB ID, released in past 1 year, type = "tv"
	ScrapeModeDefault ScrapeMode = "default"
	// ScrapeModeMissing - Scrape all MAL IDs without AniDB ID (no year/type filter)
	ScrapeModeMissing ScrapeMode = "missing"
	// ScrapeModeAll - Scrape everything, even if already has AniDB ID in cache
	ScrapeModeAll ScrapeMode = "all"
)

type Config struct {
	MalClientID string     `json:"mal-client-id"`
	TmdbApiKey  string     `json:"tmdb-api-key"`
	ScrapeMode  ScrapeMode `json:"scrape-mode"`
}
