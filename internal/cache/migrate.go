package cache

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/varoOP/shinkrodb/internal/database"
	"github.com/varoOP/shinkrodb/internal/domain"
	"golang.org/x/net/html"
)

// MigrateCache migrates existing Colly HTML cache to SQLite database
// This is a temporary function that can be removed after migration is complete
// animeRepo and malIDPath are needed to get release date and type from MAL API data
func MigrateCache(ctx context.Context, cacheDir, dbPath string, animeRepo domain.AnimeRepository, malIDPath domain.AnimePath, log zerolog.Logger) error {
	log.Info().Str("cache_dir", cacheDir).Str("db_path", dbPath).Msg("Starting cache migration")

	// Get anime data from MAL API results (malid.json) which includes release dates and types
	// This is more reliable than AniDB path since all entries have MAL data
	animeList, err := animeRepo.Get(ctx, malIDPath)
	if err != nil {
		// If anime data doesn't exist, we can still migrate but without release date/type
		log.Warn().Err(err).Msg("Failed to get anime data, migrating without release date/type")
		animeList = []domain.Anime{}
	}

	// Build map for quick lookup
	animeMap := make(map[int]domain.Anime)
	for _, a := range animeList {
		animeMap[a.MalID] = a
	}

	// Open database (schema migration happens automatically)
	dbDir := filepath.Dir(dbPath)
	db, err := database.NewDB(dbDir, log)
	if err != nil {
		return errors.Wrap(err, "failed to open database")
	}
	defer db.Close()

	// Create repository for database operations
	cacheRepo := database.NewCacheRepo(log, db)

	// Prepare regex patterns
	malIDRegex := regexp.MustCompile(`<link\s*rel="canonical"\s*\n*href="https://myanimelist\.net/anime/(\d+)/`)
	anidbIDRegex := regexp.MustCompile(`https://anidb\.net/perl-bin/animedb\.pl\?show=anime&amp.aid=(\d+)`)

	// Repository is ready for database operations

	// Walk cache directory
	var migrated, skipped, errorCount int
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("error accessing file")
			errorCount++
			return nil // Continue processing
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Read and parse HTML file (normalizes HTML structure)
		file, err := os.Open(path)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("failed to open file")
			errorCount++
			return nil
		}

		// Parse HTML to normalize structure
		doc, err := html.Parse(file)
		file.Close()
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("failed to parse HTML")
			errorCount++
			return nil
		}

		// Render parsed HTML back to string (normalized)
		var b bytes.Buffer
		if err := html.Render(&b, doc); err != nil {
			log.Warn().Err(err).Str("path", path).Msg("failed to render HTML")
			errorCount++
			return nil
		}

		htmlContent := b.String()

		// Extract MAL ID from HTML
		// log.Debug().Str("htmlContent", htmlContent).Msg("htmlContent")
		malIDMatch := malIDRegex.FindStringSubmatch(htmlContent)
		log.Debug().Strs("malIDMatch", malIDMatch).Msg("malIDMatch")
		if len(malIDMatch) < 2 {
			skipped++
			return nil // Skip files without MAL ID
		}

		malID, err := strconv.Atoi(malIDMatch[1])
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("failed to parse MAL ID")
			skipped++
			return nil
		}

		// Extract AniDB ID from HTML
		// Look for AniDB link in the HTML - should be in format: href="...aid=12345..."
		anidbID := 0
		hadAniDBID := false
		anidbMatch := anidbIDRegex.FindStringSubmatch(htmlContent)
		log.Debug().Strs("anidbMatch", anidbMatch).Msg("anidbMatch")
		matchedString := "no match"
		if len(anidbMatch) >= 2 {
			matchedString = anidbMatch[0]
			parsedAnidbID, parseErr := strconv.Atoi(anidbMatch[1])
			if parseErr == nil && parsedAnidbID > 0 {
				anidbID = parsedAnidbID
				hadAniDBID = true
			}
		}

		// Debug: Log if mal_id equals anidb_id (should not happen)
		if malID == anidbID && anidbID > 0 {
			log.Warn().
				Int("mal_id", malID).
				Int("anidb_id", anidbID).
				Str("path", path).
				Str("matched_string", matchedString).
				Msg("WARNING: mal_id equals anidb_id during migration - this should not happen!")
		}

		// Get file modification time as cached_at
		cachedAt := info.ModTime()
		if cachedAt.IsZero() {
			cachedAt = time.Now()
		}

		// Construct URL
		url := fmt.Sprintf("https://myanimelist.net/anime/%d", malID)

		// Get release date and type from anime data (for invalidation logic)
		releaseDate := ""
		animeType := ""
		if anime, exists := animeMap[malID]; exists {
			releaseDate = anime.ReleaseDate
			animeType = anime.Type
		}

		// Insert into database using repository
		entry := &domain.CacheEntry{
			MalID:       malID,
			AnidbID:     anidbID,
			TmdbID:      0, // TMDB IDs not available in old HTML cache
			URL:         url,
			CachedAt:    cachedAt.Format(time.RFC3339),
			LastUsed:    cachedAt.Format(time.RFC3339),
			HadAniDBID:  hadAniDBID,
			ReleaseDate: releaseDate,
			Type:        animeType,
		}

		if err := cacheRepo.InsertEntry(ctx, entry); err != nil {
			log.Warn().Err(err).Int("mal_id", malID).Str("path", path).Msg("failed to insert entry")
			errorCount++
			return nil
		}

		migrated++
		if migrated%100 == 0 {
			log.Info().Int("migrated", migrated).Msg("Migration progress")
		}

		return nil
	})

	if err != nil {
		return errors.Wrap(err, "failed to walk cache directory")
	}

	log.Info().
		Int("migrated", migrated).
		Int("skipped", skipped).
		Int("errors", errorCount).
		Msg("Cache migration complete")

	return nil
}
