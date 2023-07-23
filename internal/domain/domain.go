package domain

import (
	"encoding/json"
	"io"
	"log"
	"os"

	"github.com/gocolly/colly"
)

type AnimeService struct {
	Anime      Anime
	AnimeSlice []Anime
	c          *colly.Collector
}

// Anime stores information about an anime
type Anime struct {
	MainTitle    string `json:"title"`
	EnglishTitle string `json:"enTitle,omitempty"`
	MalID        int    `json:"malid"`
	AnidbID      int    `json:"anidbid,omitempty"`
	TvdbID       int    `json:"tvdbid,omitempty"`
	TmdbID       int    `json:"tmdbid,omitempty"`
	Type         string `json:"type"`
	ReleaseDate  string `json:"releaseDate"`
}

func NewAnimeService(c *colly.Collector) *AnimeService {
	return &AnimeService{
		c: c,
	}
}

func GetAnime(path string) []Anime {
	a := []Anime{}

	f, err := os.Open(path)
	checkErr(err)

	defer f.Close()
	body, err := io.ReadAll(f)
	checkErr(err)

	err = json.Unmarshal(body, &a)
	checkErr(err)

	return a
}

func StoreAnime(a []Anime, path string) {
	j, err := json.MarshalIndent(a, "", "   ")
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()
	_, err = f.Write(j)
	if err != nil {
		log.Fatal(err)
	}
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
