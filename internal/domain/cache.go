package domain

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

var (
	delCount   int
	nilMatches []string
	mutex      sync.Mutex
)

func CleanCache(rootDir string) {
	var wg sync.WaitGroup
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				doc := readFile(path)
				if shouldDelete(doc, path) {
					mutex.Lock()
					delCount++
					mutex.Unlock()
					removeFile(path)
				}
			}(path)
		}
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	wg.Wait()

	fmt.Println("Unable to check the following html pages in cache:")
	for _, v := range nilMatches {
		fmt.Println(v)
	}

	fmt.Println("Total number of files deleted: ", delCount)
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

func shouldDelete(doc, path string) bool {
	return checkTypeAndYear(doc, path) && !checkAnidb(doc)
}

func checkAnidb(doc string) bool {
	return strings.Contains(doc, "external-links-anime-pc-anidb")

}

// returns true if the anime is from the current year and it's type is tv
func checkTypeAndYear(doc, path string) bool {
	exp := `[\S\s]*(?:https://myanimelist\.net/topanime\.php\?type=(\w+)|Type:</span>\n\s{0,}(\w+)\s{0,}</div>)[\S\s\v]*(?:Aired:</span>\n\s+.*(\d{4}))`
	re := regexp.MustCompile(exp)
	match := re.FindStringSubmatch(doc)

	if match == nil {
		nilMatches = append(nilMatches, path)
		return false
	}

	showType := match[1]

	if showType == "" {
		showType = match[2]
	}

	currentYear := strconv.Itoa(time.Now().Year())
	return showType == "tv" && match[3] == currentYear
}
