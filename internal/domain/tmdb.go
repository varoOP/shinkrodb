package domain

// Domain types for TMDB integration
// All TMDB functionality has been moved to internal/tmdb/service.go

type AnimeMovie struct {
	MainTitle string `yaml:"mainTitle"`
	TMDBID    int    `yaml:"tmdbid"`
	MALID     int    `yaml:"malid"`
}

type AnimeMovies struct {
	AnimeMovie []AnimeMovie `yaml:"animeMovies"`
}

// Add adds an anime movie to the collection
func (am *AnimeMovies) Add(title string, tmdbid, malid int) {
	am.AnimeMovie = append(am.AnimeMovie, AnimeMovie{
		MainTitle: title,
		TMDBID:    tmdbid,
		MALID:     malid,
	})
}
