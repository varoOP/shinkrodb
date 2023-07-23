package animelist

import (
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
)

type AnimeList struct {
	XMLName xml.Name `xml:"anime-list"`
	Text    string   `xml:",chardata"`
	Anime   []struct {
		Text              string `xml:",chardata"`
		Anidbid           string `xml:"anidbid,attr"`
		Tvdbid            string `xml:"tvdbid,attr"`
		Tmdbid            string `xml:"tmdbid,attr"`
		Defaulttvdbseason string `xml:"defaulttvdbseason,attr"`
		Name              string `xml:"name"`
		SupplementalInfo  struct {
			Text   string `xml:",chardata"`
			Studio string `xml:"studio"`
		} `xml:"supplemental-info"`
	} `xml:"anime"`
}

func NewAnimeList() (*AnimeList, error) {
	al := &AnimeList{}
	resp, err := http.Get("https://raw.githubusercontent.com/Anime-Lists/anime-lists/master/anime-list.xml")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = xml.Unmarshal(body, al)
	if err != nil {
		return nil, err
	}

	return al, nil
}

func (a *AnimeList) GetTvdbID(aid int) int {
	var tvdbid string
	for _, anime := range a.Anime {
		if strconv.Itoa(aid) == anime.Anidbid {
			tvdbid = anime.Tvdbid
			break
		}
	}

	id, err := strconv.Atoi(tvdbid)
	if err != nil {
		return 0
	}

	return id
}
