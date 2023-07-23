package domain

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
)

type MalResponse struct {
	Data []struct {
		Node struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			MainPicture struct {
				Medium string `json:"medium"`
				Large  string `json:"large"`
			} `json:"main_picture"`
			MediaType         string `json:"media_type"`
			AlternativeTitles struct {
				Synonyms []string `json:"synonyms"`
				English  string   `json:"en"`
				Japanese string   `json:"ja"`
			} `json:"alternative_titles"`
			StartDate string `json:"start_date"`
		} `json:"node"`
		Ranking struct {
			Rank int `json:"rank"`
		} `json:"ranking"`
	} `json:"data"`
	Paging struct {
		Next string `json:"next"`
	} `json:"paging"`
}

type clientIDTransport struct {
	Transport http.RoundTripper
	ClientID  string
}

func (c *clientIDTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if c.Transport == nil {
		c.Transport = http.DefaultTransport
	}
	req.Header.Add("X-MAL-CLIENT-ID", c.ClientID)
	return c.Transport.RoundTrip(req)
}

func GetMalIds(cfg *Config) {
	c := &http.Client{
		Transport: &clientIDTransport{ClientID: cfg.MalClientID},
	}

	a := []Anime{}
	next := storeAnimeID(c, "https://api.myanimelist.net/v2/anime/ranking?ranking_type=all&limit=500&fields={media_type,start_date,alternative_titles{en}}", &a)

	for {
		if next != "" {
			next = storeAnimeID(c, next, &a)
		} else {
			break
		}
	}

	sort.SliceStable(a, func(i, j int) bool {
		return a[i].MalID < a[j].MalID
	})

	StoreAnime(a, "./malid.json")
}

func storeAnimeID(c *http.Client, url string, a *[]Anime) string {
	mal := &MalResponse{}

	resp, err := c.Get(url)
	if err != nil {
		log.Println(err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}

	err = json.Unmarshal(body, mal)
	if err != nil {
		log.Println(err)
	}

	for _, v := range mal.Data {
		*a = append(*a, Anime{
			MainTitle:    v.Node.Title,
			EnglishTitle: v.Node.AlternativeTitles.English,
			MalID:        v.Node.ID,
			Type:         v.Node.MediaType,
			ReleaseDate:  v.Node.StartDate,
		})
	}

	return mal.Paging.Next
}

func ScrapeMal() {
	var gc int
	cc := colly.NewCollector(
		// Visit only domains: myanimelist.net
		colly.AllowedDomains("myanimelist.net"),

		// Cache responses to prevent multiple download of pages
		// even if the collector is restarted
		colly.CacheDir("./mal_cache"),
	)

	extensions.RandomUserAgent(cc)
	//extensions.Referer(cc)

	as := NewAnimeService(cc)
	a := GetAnime("./malid.json")
	as.AnimeSlice = a
	r := regexp.MustCompile(`aid=(\d+)`)
	as.c.OnHTML("a[href]", func(e *colly.HTMLElement) {

		if e.Attr("data-ga-click-type") == "external-links-anime-pc-anidb" {
			url := e.Attr("href")
			m := r.FindStringSubmatch(url)

			anidbid, err := strconv.Atoi(m[1])
			if err != nil {
				log.Println(err)
			}

			log.Println("Parsed AniDB ID:", anidbid)
			as.AnimeSlice[gc].AnidbID = anidbid
		}
	})

	as.c.Limit(&colly.LimitRule{
		RandomDelay: 5 * time.Second,
		Delay:       5 * time.Second,
		Parallelism: 10,
		DomainGlob:  "*myanimelist*",
	})

	as.c.OnRequest(func(r *colly.Request) {
		log.Println("visiting", r.URL.String())
	})

	for i, v := range as.AnimeSlice {
		gc = i
		as.c.Visit(fmt.Sprintf("https://myanimelist.net/anime/%d", v.MalID))
	}

	StoreAnime(as.AnimeSlice, "./malid-anidbid.json")
}
