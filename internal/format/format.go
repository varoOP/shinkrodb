package format

import (
	"log"

	"github.com/varoOP/shinkrodb/internal/domain"
	"github.com/varoOP/shinkrodb/internal/tvdbmap"
)

const tmdbPath string = "./tmdb-mal-master.yaml"
const tvdbPath string = "./tvdb-mal-master.yaml"

func FormatTMDB() {
	tmdb := &domain.AnimeMovies{}
	err := tmdb.Get(tmdbPath)
	if err != nil {
		log.Fatal(err)
	}

	tmdb.Store(tmdbPath)
}

func FormatTVDB() {
	tvdb, err := tvdbmap.GetAnimeTVDBMap(tvdbPath)
	if err != nil {
		log.Fatal(err)
	}

	err = tvdb.Store(tvdbPath)
	if err != nil {
		log.Fatal(err)
	}
}
