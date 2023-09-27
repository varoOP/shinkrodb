package main

import (
	"fmt"
	"log"
	"path"

	"github.com/spf13/pflag"
	"github.com/varoOP/shinkrodb/internal/config"
	"github.com/varoOP/shinkrodb/internal/domain"
	"github.com/varoOP/shinkrodb/internal/format"
	"github.com/varoOP/shinkrodb/internal/tvdbmap"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	var rootPath string
	pflag.StringVar(&rootPath, "rootPath", ".", "the path where output is saved")
	pflag.Parse()

	switch cmd := pflag.Arg(0); cmd {
	case "run":
		cfg := config.NewConfig()
		domain.GetMalIds(cfg)
		domain.ScrapeMal()
		domain.GetTvdbIDs()
		domain.GetTmdbIds(cfg, rootPath)
		a := domain.GetAnime("./malid-anidbid-tvdbid-tmdbid.json")
		fmt.Println("Total number of dupes:", domain.CheckDupes(a))
		unmapped := tvdbmap.CreateAnimeTVDBMap(rootPath)
		err := tvdbmap.UpdateMaster(unmapped, rootPath)
		if err != nil {
			log.Fatal(err)
		}

	case "genmap":
		am := &domain.AnimeMovies{}
		err := am.Get(path.Join(rootPath, "tmdb-mal-master.yaml"))
		if err != nil {
			log.Fatal(err)
		}

		domain.CreateMapping(am, path.Join(rootPath, "tmdb-mal.yaml"))
		err = tvdbmap.GenerateAnimeTVDBMap(rootPath)
		if err != nil {
			log.Fatal(err)
		}

	case "format":
		format.FormatTMDB()
		format.FormatTVDB()

	case "version":
		fmt.Printf("shinkrodb: %v\n", version)
		fmt.Printf("Commit: %v\n", commit)
		fmt.Printf("Build Date: %v\n", date)

	default:
		fmt.Println("ERROR: no command specified")
	}
}
