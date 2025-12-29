package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
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
	db, err := OpenDB(ctx, dbPath, log)
	if err != nil {
		return errors.Wrap(err, "failed to open database")
	}
	defer db.Close()

	// Prepare regex patterns
	malIDRegex := regexp.MustCompile(`<link\s*rel="canonical"\s*\n*href="https://myanimelist\.net/anime/(\d+)/`)
	// Match AniDB URL format: https://anidb.net/perl-bin/animedb.pl?show=anime&aid=12345
	// HTML may have &amp; instead of &, so match both &aid= and &amp;aid=
	// Using pattern: https://anidb.net/perl-bin/animedb.pl?show=anime&amp.aid=(\d+)
	// The period (.) matches any character (will match &amp;aid= with semicolon)
	anidbIDRegex := regexp.MustCompile(`https://anidb\.net/perl-bin/animedb\.pl\?show=anime&amp.aid=(\d+)`)

	// Prepare insert statement (schema always includes release_date and type now)
	insertStmt, err := db.PrepareContext(ctx, `
		INSERT OR REPLACE INTO cache_entries 
		(mal_id, anidb_id, url, cached_at, last_used, html_hash, had_anidb_id, release_date, type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return errors.Wrap(err, "failed to prepare insert statement")
	}
	defer insertStmt.Close()

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

		// Also read raw content for hash calculation
		file, err = os.Open(path)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("failed to reopen file for hash")
			errorCount++
			return nil
		}
		content, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("failed to read file for hash")
			errorCount++
			return nil
		}

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

		// Calculate HTML hash for change detection
		hash := sha256.Sum256(content)
		htmlHash := hex.EncodeToString(hash[:])

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

		// Insert into database
		// Parameter order must match: mal_id, anidb_id, url, cached_at, last_used, html_hash, had_anidb_id, release_date, type
		_, err = insertStmt.ExecContext(ctx,
			malID,                         // mal_id
			anidbID,                       // anidb_id
			url,                           // url
			cachedAt.Format(time.RFC3339), // cached_at
			cachedAt.Format(time.RFC3339), // last_used
			htmlHash,                      // html_hash
			hadAniDBID,                    // had_anidb_id
			releaseDate,                   // release_date
			animeType,                     // type
		)

		if err != nil {
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
