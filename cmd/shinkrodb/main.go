package main

import (
	"fmt"
	"path"

	"github.com/spf13/pflag"
	"github.com/varoOP/shinkrodb/internal/config"
	"github.com/varoOP/shinkrodb/internal/domain"
)

func main() {
	cfg := config.NewConfig()
	var rootPath string
	pflag.StringVar(&rootPath, "rootPath", ".", "the path where output is saved")
	pflag.Parse()

	switch cmd := pflag.Arg(0); cmd {
	case "run":
		domain.GetMalIds(cfg)
		domain.ScrapeMal()
		domain.GetTvdbIDs()
		domain.GetTmdbIds(cfg, rootPath)
		a := domain.GetAnime("./malid-anidbid-tvdbid-tmdbid.json")
		fmt.Println("Total number of dupes:", domain.CheckDupes(a))

	case "genmap":
		am := &domain.AnimeMovies{}
		am.Get(path.Join(rootPath, "tmdb-mal-master.yaml"))
		domain.CreateMapping(am, path.Join(rootPath, "tmdb-mal.yaml"))

	default:
		fmt.Println("ERROR: no command specified")
	}
}
