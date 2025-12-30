package animelist

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
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

	// Cache for O(1) lookups
	tvdbMap map[int]int
	tmdbMap map[int]int
}

const (
	animeListURL  = "https://raw.githubusercontent.com/Anime-Lists/anime-lists/master/anime-list.xml"
	cacheFileName = "anime-list.xml"
	cacheMaxAge   = 24 * time.Hour // Refresh cache every 24 hours
)

// NewAnimeList creates a new AnimeList with caching support
// cacheDir: directory to cache the XML file (empty string disables caching)
func NewAnimeList(ctx context.Context, cacheDir string) (*AnimeList, error) {
	al := &AnimeList{
		tvdbMap: make(map[int]int),
		tmdbMap: make(map[int]int),
	}

	var body []byte
	var err error

	// Try to load from cache if cache directory is provided
	if cacheDir != "" {
		cachePath := filepath.Join(cacheDir, cacheFileName)
		if body, err = loadFromCache(ctx, cachePath); err == nil {
			// Successfully loaded from cache
		} else {
			// Cache miss or error, fetch from URL
			body, err = fetchFromURL(ctx, cachePath)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// No caching, fetch directly
		body, err = fetchDirectly(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Parse XML
	err = xml.Unmarshal(body, al)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	// Build lookup map for O(1) access
	al.buildMap()

	return al, nil
}

// loadFromCache attempts to load the XML file from cache
func loadFromCache(ctx context.Context, cachePath string) ([]byte, error) {
	// Check if context is cancelled
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, err // Cache file doesn't exist
	}

	// Check if cache is still fresh
	if time.Since(info.ModTime()) > cacheMaxAge {
		return nil, fmt.Errorf("cache expired")
	}

	body, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	return body, nil
}

// fetchFromURL fetches the XML from URL and saves to cache
func fetchFromURL(ctx context.Context, cachePath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, animeListURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch anime list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Save to cache (best effort, don't fail if cache write fails)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err == nil {
		_ = os.WriteFile(cachePath, body, 0644)
	}

	return body, nil
}

// fetchDirectly fetches the XML from URL without caching
func fetchDirectly(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, animeListURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch anime list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}

// buildMap builds an in-memory map for O(1) lookups
func (a *AnimeList) buildMap() {
	for _, anime := range a.Anime {
		anidbID, err := strconv.Atoi(anime.Anidbid)
		if err != nil {
			continue // Skip invalid AniDB IDs
		}

		// Build TVDB map
		tvdbID, err := strconv.Atoi(anime.Tvdbid)
		if err == nil && tvdbID > 0 {
			a.tvdbMap[anidbID] = tvdbID
		}

		// Build TMDB map
		tmdbID, err := strconv.Atoi(anime.Tmdbid)
		if err == nil && tmdbID > 0 {
			a.tmdbMap[anidbID] = tmdbID
		}
	}
}

// GetTvdbID returns the TVDB ID for a given AniDB ID (O(1) lookup)
func (a *AnimeList) GetTvdbID(aid int) int {
	return a.tvdbMap[aid]
}

// GetTmdbID returns the TMDB ID for a given AniDB ID (O(1) lookup)
func (a *AnimeList) GetTmdbID(aid int) int {
	return a.tmdbMap[aid]
}
