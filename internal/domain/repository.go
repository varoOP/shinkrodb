package domain

import (
	"context"
)

// AnimeRepository defines the interface for anime data storage
type AnimeRepository interface {
	Get(ctx context.Context, path AnimePath) ([]Anime, error)
	Store(ctx context.Context, path AnimePath, anime []Anime) error
}

// MappingRepository defines the interface for mapping data storage
type MappingRepository interface {
	GetTMDBMaster(ctx context.Context, path string) (*AnimeMovies, error)
	StoreTMDBMaster(ctx context.Context, path string, movies *AnimeMovies) error
	GetTVDBMaster(ctx context.Context, path string) (*TVDBMap, error)
	StoreTVDBMaster(ctx context.Context, path string, map_ *TVDBMap) error
}

// TVDBMap represents the TVDB mapping structure
type TVDBMap struct {
	Anime []TVDBAnime `yaml:"AnimeMap"`
}

// TVDBAnime represents a single TVDB anime mapping
type TVDBAnime struct {
	Malid        int            `yaml:"malid"`
	Title        string         `yaml:"title"`
	Type         string         `yaml:"type"`
	Tvdbid       int            `yaml:"tvdbid"`
	TvdbSeason   int            `yaml:"tvdbseason"`
	Start        int            `yaml:"start"`
	UseMapping   bool           `yaml:"useMapping"`
	AnimeMapping []AnimeMapping `yaml:"animeMapping"`
}

// AnimeMapping represents episode mapping configuration
type AnimeMapping struct {
	TvdbSeason       int          `yaml:"tvdbseason"`
	Start            int          `yaml:"start"`
	MappingType      string       `yaml:"mappingType,omitempty"`
	ExplicitEpisodes map[int]int  `yaml:"explicitEpisodes,omitempty"`
	SkipMalEpisodes  []int        `yaml:"skipMalEpisodes,omitempty"`
}

