package domain

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
)

type Animetitles struct {
	XMLName xml.Name `xml:"animetitles"`
	Anime   []struct {
		Aid   string `xml:"aid,attr"`
		Title []struct {
			Text string `xml:",chardata"`
			Type string `xml:"type,attr"`
			Lang string `xml:"lang,attr"`
		} `xml:"title"`
	} `xml:"anime"`
}

type TMDBAPIResponse struct {
	Page    int `json:"page"`
	Results []struct {
		Adult            bool    `json:"adult"`
		BackdropPath     string  `json:"backdrop_path"`
		GenreIds         []int   `json:"genre_ids"`
		ID               int     `json:"id"`
		OriginalLanguage string  `json:"original_language"`
		OriginalTitle    string  `json:"original_title"`
		Overview         string  `json:"overview"`
		Popularity       float64 `json:"popularity"`
		PosterPath       string  `json:"poster_path"`
		ReleaseDate      string  `json:"release_date"`
		Title            string  `json:"title"`
		Video            bool    `json:"video"`
		VoteAverage      float64 `json:"vote_average"`
		VoteCount        int     `json:"vote_count"`
	} `json:"results"`
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
}

func GetTmdbIds(cfg *Config) {
	a := GetAnime("./malid-anidbid-tvdbid.json")
	u := buildUrl(cfg.TmdbApiKey)
	noTmdbTotal := 0
	withTmdbTotal := 0
	totalMovies := 0
	for i, anime := range a {
		if anime.Type == "movie" {
			totalMovies++
			target := *u
			query := target.Query()
			if anime.EnglishTitle != "" {
				query.Add("query", anime.EnglishTitle)
			} else {
				query.Add("query", anime.MainTitle)
			}

			if anime.ReleaseDate == "" {
				noTmdbTotal++
				log.Println(anime.MainTitle, "does not have a release date.")
				continue
			}

			year := getYear(anime.ReleaseDate)
			query.Add("year", year)
			target.RawQuery = query.Encode()
			tmdb := TMDBAPIResponse{}
			resp, err := http.Get(target.String())
			checkErr(err)

			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			checkErr(err)

			err = json.Unmarshal(body, &tmdb)
			checkErr(err)

			for _, result := range tmdb.Results {
				if result.ReleaseDate == anime.ReleaseDate || tmdb.TotalResults == 1 {
					a[i].TmdbID = result.ID
					withTmdbTotal++
					//log.Println("TMDBID added for", anime.MainTitle, result.ID)
					break
				} else {
					log.Println("For the following anime", anime.MainTitle)
					fmt.Printf("\t\tTMDB date:%v does not match MAL date:%v and has multiple results!\n\n", result.ReleaseDate, anime.ReleaseDate)
					fmt.Printf("\t\tAnime:\n")
					fmt.Printf("\t\t\t\t%+v\n", anime)
					fmt.Printf("\t\tTMDB:\n")
					fmt.Printf("\t\t\t\t%+v\n", tmdb)
				}
			}

			if a[i].TmdbID == 0 {
				noTmdbTotal++
			}
		}
	}

	StoreAnime(a, "./malid-anidbid-tvdbid-tmdbid.json")
	log.Println("Total number of movies", totalMovies)
	log.Println("Total number of movies with TMDBID", withTmdbTotal)
	log.Println("Total number of movies without TMDBID", noTmdbTotal)
}

func buildUrl(apikey string) *url.URL {
	baseUrl := "https://api.themoviedb.org/3/search/movie"
	u, err := url.Parse(baseUrl)
	checkErr(err)

	query := u.Query()
	query.Add("api_key", apikey)
	query.Add("language", "en-US")
	query.Add("page", "1")
	query.Add("include_adult", "true")
	u.RawQuery = query.Encode()
	return u
}

func getYear(d string) string {
	r := regexp.MustCompile(`^\d{4,4}`)
	return r.FindString(d)
}
