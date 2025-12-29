package database

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/varoOP/shinkrodb/internal/domain"
)

// CacheRepo implements domain.CacheRepo interface
type CacheRepo struct {
	log zerolog.Logger
	db  *DB
}

// NewCacheRepo creates a new cache repository
func NewCacheRepo(log zerolog.Logger, db *DB) domain.CacheRepo {
	return &CacheRepo{
		log: log.With().Str("repo", "cache").Logger(),
		db:  db,
	}
}

// GetAniDBIDs returns a map of MAL ID to AniDB ID for entries that have AniDB IDs
func (r *CacheRepo) GetAniDBIDs(ctx context.Context) (map[int]int, error) {
	queryBuilder := r.db.squirrel.
		Select("mal_id", "anidb_id").
		From("cache_entries").
		Where(sq.Gt{"anidb_id": 0})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	r.log.Trace().Str("query", query).Interface("args", args).Msg("GetAniDBIDs")

	rows, err := r.db.handler.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing query")
	}
	defer rows.Close()

	result := make(map[int]int)
	for rows.Next() {
		var malID, anidbID int
		if err := rows.Scan(&malID, &anidbID); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}
		result[malID] = anidbID
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating rows")
	}

	return result, nil
}

// UpsertEntry inserts or updates a cache entry
func (r *CacheRepo) UpsertEntry(ctx context.Context, entry *domain.CacheEntry) error {
	// Try to update first
	updateBuilder := r.db.squirrel.
		Update("cache_entries").
		Set("anidb_id", entry.AnidbID).
		Set("had_anidb_id", entry.HadAniDBID).
		Set("last_used", entry.LastUsed).
		Set("release_date", entry.ReleaseDate).
		Set("type", entry.Type).
		Where(sq.Eq{"mal_id": entry.MalID})

	query, args, err := updateBuilder.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building update query")
	}

	r.log.Trace().Str("query", query).Interface("args", args).Msg("UpsertEntry update")

	result, err := r.db.handler.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "error executing update query")
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		return nil // Update successful
	}

	// No rows affected, insert new entry using Replace (SQLite INSERT OR REPLACE)
	insertBuilder := r.db.squirrel.
		Replace("cache_entries").
		Columns("mal_id", "anidb_id", "url", "cached_at", "last_used", "had_anidb_id", "release_date", "type").
		Values(entry.MalID, entry.AnidbID, entry.URL, entry.CachedAt, entry.LastUsed, entry.HadAniDBID, entry.ReleaseDate, entry.Type)

	query, args, err = insertBuilder.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building insert query")
	}

	r.log.Trace().Str("query", query).Interface("args", args).Msg("UpsertEntry insert")

	_, err = r.db.handler.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "error executing insert query")
	}

	return nil
}

// InsertEntry inserts a new cache entry (used during migration)
// Uses Replace (SQLite INSERT OR REPLACE) for SQLite compatibility
func (r *CacheRepo) InsertEntry(ctx context.Context, entry *domain.CacheEntry) error {
	queryBuilder := r.db.squirrel.
		Replace("cache_entries").
		Columns("mal_id", "anidb_id", "url", "cached_at", "last_used", "had_anidb_id", "release_date", "type").
		Values(entry.MalID, entry.AnidbID, entry.URL, entry.CachedAt, entry.LastUsed, entry.HadAniDBID, entry.ReleaseDate, entry.Type)

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building insert query")
	}

	r.log.Trace().Str("query", query).Interface("args", args).Msg("InsertEntry")

	_, err = r.db.handler.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "error executing insert query")
	}

	return nil
}

// GetEntriesByReleaseYear returns cache entries for anime released in a specific year
func (r *CacheRepo) GetEntriesByReleaseYear(ctx context.Context, year int) ([]*domain.CacheEntry, error) {
	queryBuilder := r.db.squirrel.
		Select("mal_id", "anidb_id", "release_date", "type", "had_anidb_id").
		From("cache_entries").
		Where(sq.And{
			sq.Expr("release_date IS NOT NULL"),
			sq.NotEq{"release_date": ""},
			sq.Or{
				sq.Expr("CAST(strftime('%Y', release_date) AS INTEGER) = ?", year),
				sq.Expr("CAST(SUBSTR(release_date, 1, 4) AS INTEGER) = ?", year),
			},
			sq.Or{
				sq.Eq{"had_anidb_id": 0},
				sq.Eq{"anidb_id": 0},
			},
		})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	r.log.Trace().Str("query", query).Interface("args", args).Msg("GetEntriesByReleaseYear")

	rows, err := r.db.handler.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing query")
	}
	defer rows.Close()

	var entries []*domain.CacheEntry
	for rows.Next() {
		entry := &domain.CacheEntry{}
		var hadAniDBID bool
		if err := rows.Scan(&entry.MalID, &entry.AnidbID, &entry.ReleaseDate, &entry.Type, &hadAniDBID); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}
		entry.HadAniDBID = hadAniDBID
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating rows")
	}

	return entries, nil
}

// DeleteEntry deletes a cache entry by MAL ID
func (r *CacheRepo) DeleteEntry(ctx context.Context, malID int) error {
	queryBuilder := r.db.squirrel.
		Delete("cache_entries").
		Where(sq.Eq{"mal_id": malID})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building delete query")
	}

	r.log.Trace().Str("query", query).Interface("args", args).Msg("DeleteEntry")

	_, err = r.db.handler.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "error executing delete query")
	}

	return nil
}
