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
	MalClientID      string    `toml:"mal_client_id" mapstructure:"mal_client_id"`
	TmdbApiKey       string    `toml:"tmdb_api_key" mapstructure:"tmdb_api_key"`
	AniDBMode        FetchMode `toml:"anidb_mode" mapstructure:"anidb_mode"`
	TMDBMode         FetchMode `toml:"tmdb_mode" mapstructure:"tmdb_mode"`
	DiscordWebhookURL string   `toml:"discord_webhook_url" mapstructure:"discord_webhook_url"`
}
