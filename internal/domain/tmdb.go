package domain

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type AnimeMovie struct {
	MainTitle string `yaml:"mainTitle"`
	TMDBID    int    `yaml:"tmdbid"`
	MALID     int    `yaml:"malid"`
}

type AnimeMovies struct {
	AnimeMovie []AnimeMovie `yaml:"animeMovies"`
}

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

func GetTmdbIds(cfg *Config, rootPath string) {
	a := GetAnime("./malid-anidbid-tvdbid.json")
	u := buildUrl(cfg.TmdbApiKey)
	am := &AnimeMovies{}
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
				am.Add(a[i].MainTitle, a[i].TmdbID, a[i].MalID)
			}
		}
	}

	StoreAnime(a, "./malid-anidbid-tvdbid-tmdbid.json")
	log.Println("Total number of movies", totalMovies)
	log.Println("Total number of movies with TMDBID", withTmdbTotal)
	log.Println("Total number of movies without TMDBID", noTmdbTotal)
	am.Store(path.Join(rootPath, "tmdb-mal-unmapped.yaml"))
	UpdateMaster(&AnimeMovies{}, am, path.Join(rootPath, "tmdb-mal-master.yaml"))
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

func (am *AnimeMovies) Add(title string, tmdbid, malid int) {
	am.AnimeMovie = append(am.AnimeMovie, AnimeMovie{
		MainTitle: title,
		TMDBID:    tmdbid,
		MALID:     malid,
	})
}

func (am *AnimeMovies) Store(path string) {
	b, err := yaml.Marshal(am)
	if err != nil {
		checkErr(err)
	}

	f, err := os.Create(path)
	if err != nil {
		checkErr(err)
	}

	text := string(b)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(line, "malid") {
			lines[i] += "\n"
		}
	}

	modifiedText := strings.Join(lines, "\n")
	defer f.Close()
	_, err = f.Write([]byte(modifiedText))
	if err != nil {
		checkErr(err)
	}
}

func (am *AnimeMovies) Get(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return errors.New("file does not exist")
	}

	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		checkErr(err)
	}

	err = yaml.Unmarshal(b, am)
	if err != nil {
		checkErr(err)
	}

	return err
}

func UpdateMaster(am1 *AnimeMovies, am2 *AnimeMovies, path string) {
	err := am1.Get(path)
	if err.Error() == "file does not exist" {
		am2.Store(path)
		return
	}

	malidToTmdbid := make(map[int]int)
	for i := range am1.AnimeMovie {
		if am1.AnimeMovie[i].TMDBID != 0 {
			malidToTmdbid[am1.AnimeMovie[i].MALID] = am1.AnimeMovie[i].TMDBID
		}
	}

	for ii := range am2.AnimeMovie {
		if tmdbid, found := malidToTmdbid[am2.AnimeMovie[ii].MALID]; found {
			am2.AnimeMovie[ii].TMDBID = tmdbid
		}
	}

	am2.Store(path)
}

func CreateMapping(am *AnimeMovies, path string) {
	amf := &AnimeMovies{}
	for _, movie := range am.AnimeMovie {
		if movie.TMDBID != 0 {
			amf.AnimeMovie = append(amf.AnimeMovie, movie)
		}
	}

	amf.Store(path)
}
