package cache

// import (
// 	"context"
// 	"os"
// 	"strings"
// 	"time"

// 	"github.com/pkg/errors"
// 	"github.com/rs/zerolog"
// 	"github.com/varoOP/shinkrodb/internal/domain"
// )

// type Service interface {
// 	CleanCache(ctx context.Context, dbPath string, animeRepo domain.AnimeRepository, anidbPath domain.AnimePath) error
// }

// type service struct {
// 	log zerolog.Logger
// }

// func NewService(log zerolog.Logger) Service {
// 	return &service{
// 		log: log.With().Str("module", "cache").Logger(),
// 	}
// }

// func (s *service) CleanCache(ctx context.Context, dbPath string, animeRepo domain.AnimeRepository, anidbPath domain.AnimePath) error {
// 	s.log.Info().Msg("Cleaning cache..")

// 	// Check if database exists
// 	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
// 		s.log.Info().Msg("Cache database does not exist yet, skipping cache cleanup (first run)")
// 		return nil
// 	}

// 	// Open database (schema migration happens automatically)
// 	db, err := OpenDB(ctx, dbPath, s.log)
// 	if err != nil {
// 		return errors.Wrap(err, "failed to open database")
// 	}
// 	defer db.Close()

// 	// Get anime data to check current AniDB IDs
// 	anime, err := animeRepo.Get(ctx, anidbPath)
// 	if err != nil {
// 		// If the file doesn't exist, this is likely the first run - skip cache cleaning
// 		if errors.Is(err, os.ErrNotExist) {
// 			s.log.Info().Msg("AniDB file does not exist yet, skipping cache cleanup (first run)")
// 			return nil
// 		}
// 		errMsg := err.Error()
// 		if errMsg != "" && (strings.Contains(errMsg, "does not exist") ||
// 			strings.Contains(errMsg, "no such file")) {
// 			s.log.Info().Msg("AniDB file does not exist yet, skipping cache cleanup (first run)")
// 			return nil
// 		}
// 		return errors.Wrap(err, "failed to get anime data")
// 	}

// 	// Build map of current AniDB IDs
// 	anidbMap := make(map[int]int)
// 	for _, a := range anime {
// 		if a.AnidbID > 0 {
// 			anidbMap[a.MalID] = a.AnidbID
// 		}
// 	}

// 	// Get current year for invalidation logic
// 	currentYear := time.Now().Year()

// 	// Query database for entries to invalidate:
// 	// - TV shows from current year
// 	// - Don't have AniDB ID (or had_anidb_id = false)
// 	// - Either don't exist in current anime data, or still don't have AniDB ID
// 	query := `
// 		SELECT mal_id, anidb_id, release_date, type, had_anidb_id
// 		FROM cache_entries
// 		WHERE type = 'tv'
// 		  AND release_date IS NOT NULL
// 		  AND release_date != ''
// 		  AND (CAST(strftime('%Y', release_date) AS INTEGER) = ? OR 
// 		       CAST(SUBSTR(release_date, 1, 4) AS INTEGER) = ?)
// 		  AND (had_anidb_id = 0 OR anidb_id = 0)
// 	`

// 	rows, err := db.QueryContext(ctx, query, currentYear, currentYear)
// 	if err != nil {
// 		return errors.Wrap(err, "failed to query cache entries")
// 	}
// 	defer rows.Close()

// 	var toDelete []int
// 	for rows.Next() {
// 		var malID, anidbID int
// 		var releaseDate, animeType string
// 		var hadAniDBID bool

// 		if err := rows.Scan(&malID, &anidbID, &releaseDate, &animeType, &hadAniDBID); err != nil {
// 			s.log.Warn().Err(err).Msg("failed to scan cache entry")
// 			continue
// 		}

// 		// Check if this entry still doesn't have an AniDB ID in current data
// 		currentAnidbID, hasAnidbID := anidbMap[malID]
// 		if !hasAnidbID || currentAnidbID == 0 {
// 			toDelete = append(toDelete, malID)
// 		}
// 	}

// 	if err := rows.Err(); err != nil {
// 		return errors.Wrap(err, "error iterating cache entries")
// 	}

// 	// Delete invalidated entries from database
// 	deletedCount := 0
// 	if len(toDelete) > 0 {
// 		deleteStmt, err := db.PrepareContext(ctx, `DELETE FROM cache_entries WHERE mal_id = ?`)
// 		if err != nil {
// 			return errors.Wrap(err, "failed to prepare delete statement")
// 		}
// 		defer deleteStmt.Close()

// 		for _, malID := range toDelete {
// 			if _, err := deleteStmt.ExecContext(ctx, malID); err != nil {
// 				s.log.Warn().Err(err).Int("mal_id", malID).Msg("failed to delete cache entry")
// 				continue
// 			}
// 			deletedCount++
// 		}
// 	}

// 	s.log.Info().
// 		Int("deleted_count", deletedCount).
// 		Int("total_checked", len(toDelete)).
// 		Msg("Cache cleanup complete")

// 	if deletedCount > 0 {
// 		s.log.Debug().
// 			Ints("deleted_ids", toDelete).
// 			Msg("Invalidated cache entries")
// 	}

// 	return nil
// }
//