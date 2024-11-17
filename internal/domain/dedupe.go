package domain

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

var AidTitleMap = map[int]string{}

func CheckDupes(a []Anime) int {
	dupeanidb := []Anime{}
	deduped := []Anime{}
	indexes := []int{}
	for i, v := range a {
		count := 0
		if v.AnidbID == 0 {
			continue
		}

		for _, vv := range a {
			if v.AnidbID == vv.AnidbID && v.Type == vv.Type && v.Type == "tv" {
				count++
			}
		}

		if count > 1 {
			dupeanidb = append(dupeanidb, v)
			indexes = append(indexes, i)
			count = 0
			_, ok := AidTitleMap[v.AnidbID]
			if !ok {
				AidTitleMap[v.AnidbID] = ""
			}
		}
	}

	for _, val := range AidTitleMap {
		if val != "" {
			break
		} else {
			fillAidTitleMap()
		}
	}

	sort.SliceStable(dupeanidb, func(i, j int) bool {
		return dupeanidb[i].AnidbID < dupeanidb[j].AnidbID
	})

	if len(indexes) > 0 {
		j, err := json.MarshalIndent(dupeanidb, "", "   ")
		checkErr(err)

		fmt.Println(string(j))
		fmt.Println("Check dupe reporting:", len(dupeanidb))
		deduped = checkTitle(a, indexes)
	}

	StoreAnime(deduped, shinkroPath)
	return len(dupeanidb)
}

func checkTitle(a []Anime, indexes []int) []Anime {
	deduped := []Anime{}
	for _, index := range indexes {
		mainTitle := AidTitleMap[a[index].AnidbID]
		if !strings.EqualFold(a[index].MainTitle, mainTitle) {
			fmt.Println("Deleting from slice", a[index])
			deduped = RemoveIndex(a, index)
			break
		}
	}

	CheckDupes(deduped)
	return deduped
}

func RemoveIndex(a []Anime, index int) []Anime {
	return append(a[:index], a[index+1:]...)
}

func fillAidTitleMap() {
	for key := range AidTitleMap {
		anidb := &Animetitles{}
		resp, err := http.Get("https://github.com/Anime-Lists/anime-lists/raw/master/animetitles.xml")
		checkErr(err)

		defer resp.Body.Close()
		xr := xml.NewDecoder(resp.Body)
		err = xr.Decode(anidb)
		if err != nil {
			log.Fatal(err)
		}

		for _, anime := range anidb.Anime {
			if anime.Aid == strconv.Itoa(key) {
				for _, title := range anime.Title {
					if title.Type == "main" {
						AidTitleMap[key] = title.Text
						break
					}
				}
			}
		}
	}
}
