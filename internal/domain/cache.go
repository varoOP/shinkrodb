package domain

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/net/html"
)

var (
	delCount   []int
	nilMatches []string
)

var anime []Anime
var typeDateMap map[int]Anime

func CleanCache(rootDir string) {
	anime = GetAnime(AniDBIDPath)
	typeDateMap = *loadMalid(anime)

	fmt.Println("Cleaning mal_cache..")
	exp := `<link\s*rel="canonical"\s*\n*href="https://myanimelist\.net/anime/(\d+)/`
	re := regexp.MustCompile(exp)

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			func(path string) {
				doc := readFile(path)
				if delete, id := shouldDelete(doc, path, re); delete {
					delCount = append(delCount, id)
					removeFile(path)
				}
			}(path)
		}
		return nil
	})

	if errors.Is(err, os.ErrNotExist) {
		log.Println("mal_cache does not exist and will be created")
		return
	}

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Unable to check the following html pages in cache: %v\n", len(nilMatches))
	for _, v := range nilMatches {
		fmt.Println(v)
	}

	ld := len(delCount)
	fmt.Println("Total number of files deleted: ", ld)
	if ld > 0 {
		fmt.Println("Following MAL pages were deleted from cache:")
		for _, v := range delCount {
			fmt.Printf("https://myanimelist.net/anime/%d\n", v)
		}
	}
}

func loadMalid(anime []Anime) *map[int]Anime {
	idDate := make(map[int]Anime, len(anime))
	for _, a := range anime {
		idDate[a.MalID] = a
	}
	return &idDate
}

func removeFile(path string) {
	err := os.Remove(path)
	if err != nil {
		log.Fatalf("Failed to delete file %s: %v", path, err)
	}
}

func readFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	d, err := html.Parse(f)
	if err != nil {
		log.Fatal(err)
	}

	var b bytes.Buffer
	err = html.Render(&b, d)
	if err != nil {
		log.Fatal(err)
	}

	doc := b.String()
	return doc
}

// returns true if the anime is from the current year, it's type is tv and doesn't have an anidbID
func shouldDelete(doc, path string, re *regexp.Regexp) (bool, int) {
	var animeDate time.Time
	match := re.FindStringSubmatch(doc)

	if match == nil {
		nilMatches = append(nilMatches, path)
		return false, 0
	}

	malID, err := strconv.Atoi(match[1])
	if err != nil {
		log.Printf("Error converting malID from string to int for path: %s\n", path)
		return false, 0
	}

	animeInfo, exists := typeDateMap[malID]
	if !exists {
		log.Printf("Warning: malID %d not found in typeDateMap for path: %s\n", malID, path)
		return true, malID
	}

	showType := animeInfo.Type
	dateStr := animeInfo.ReleaseDate

	if dateStr != "" {
		// Attempt to parse date with various formats
		animeDate, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			animeDate, err = time.Parse("2006-01", dateStr)
			if err != nil {
				animeDate, err = time.Parse("2006", dateStr)
				if err != nil {
					log.Printf("Warning: Unable to parse date for malID %d (%s) in path: %s\n", malID, dateStr, path)
					return false, 0
				}
			}
		}
	}

	currentYear := time.Now().Year()
	return showType == "tv" && animeDate.Year() == currentYear && animeInfo.AnidbID <= 0, malID
}
