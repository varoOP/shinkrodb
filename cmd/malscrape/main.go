package main

import (
	"fmt"

	"github.com/varoOP/shinkrodb/internal/config"
	"github.com/varoOP/shinkrodb/internal/domain"
)

func main() {
	cfg := config.NewConfig()
	domain.GetMalIds(cfg)
	domain.ScrapeMal()
	domain.GetTvdbIDs()
	domain.GetTmdbIds(cfg)
	a := domain.GetAnime("./malid-anidbid-tvdbid-tmdbid.json")
	fmt.Println("Total number of dupes:", domain.CheckDupes(a))
}
