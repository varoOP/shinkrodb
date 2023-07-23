package domain

import (
	"log"

	"github.com/varoOP/shinkrodb/pkg/animelist"
)

func GetTvdbIDs() {
	al, err := animelist.NewAnimeList()
	if err != nil {
		log.Fatal(err)
	}

	a := GetAnime("./malid-anidbid.json")
	for i, anime := range a {
		if anime.Type == "tv" && anime.AnidbID > 0 {
			if tvdbid := al.GetTvdbID(anime.AnidbID); tvdbid > 0 {
				a[i].TvdbID = tvdbid
			}
		}
	}

	StoreAnime(a, "./malid-anidbid-tvdbid.json")
}
