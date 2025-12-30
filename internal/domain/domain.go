package domain

// Anime stores information about an anime
type Anime struct {
	MainTitle     string   `json:"title"`
	EnglishTitle  string   `json:"enTitle,omitempty"`
	JapaneseTitle string   `json:"-"` // Not serialized to JSON, kept in memory only
	Synonyms      []string `json:"-"` // Not serialized to JSON, kept in memory only
	MalID         int      `json:"malid"`
	AnidbID       int      `json:"anidbid,omitempty"`
	TvdbID        int      `json:"tvdbid,omitempty"`
	TmdbID        int      `json:"tmdbid,omitempty"`
	Type          string   `json:"type"`
	ReleaseDate   string   `json:"releaseDate"`
}
