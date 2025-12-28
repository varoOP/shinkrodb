package cache

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

// OpenDB opens the cache database and ensures schema is up to date
// This should be called whenever the database is accessed
func OpenDB(ctx context.Context, dbPath string, log zerolog.Logger) (*sql.DB, error) {
	// Open SQLite database (using same driver as shinkro)
	dsn := dbPath + "?_pragma=busy_timeout%3d1000&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open database")
	}

	// Ensure schema is up to date (migrates if needed)
	if err := MigrateSchema(ctx, db, log); err != nil {
		db.Close()
		return nil, errors.Wrap(err, "failed to migrate schema")
	}

	return db, nil
}

// MigrateSchema handles database schema creation and migrations using versioning
// Follows the same pattern as shinkro's database migration strategy
// This is called automatically when opening the database
func MigrateSchema(ctx context.Context, db *sql.DB, log zerolog.Logger) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return errors.Wrap(err, "failed to query schema version")
	}

	if version == len(migrations) {
		log.Debug().Int("version", version).Msg("Database schema is up to date")
		return nil
	} else if version > len(migrations) {
		return errors.Errorf("cache database schema version (%d) is newer than supported (%d)", version, len(migrations))
	}

	log.Info().Msgf("Beginning database schema upgrade from version %v to version: %v", version, len(migrations))

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback()

	if version == 0 {
		// Create initial schema
		if _, err := tx.ExecContext(ctx, schema); err != nil {
			return errors.Wrap(err, "failed to initialize schema")
		}
		log.Info().Msg("Created initial cache database schema")
	} else {
		// Apply incremental migrations
		for i := version; i < len(migrations); i++ {
			if migrations[i] == "" {
				continue // Skip empty migration (version 0)
			}
			log.Info().Msgf("Upgrading cache database schema to version: %v", i+1)
			if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
				return errors.Wrapf(err, "failed to execute migration #%v", i)
			}
		}
	}

	// Update schema version
	_, err = tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", len(migrations)))
	if err != nil {
		return errors.Wrap(err, "failed to bump schema version")
	}

	log.Info().Msgf("Database schema upgraded to version: %v", len(migrations))
	return tx.Commit()
}

// GetDBPath returns the database path for a given root path
func GetDBPath(rootPath string) string {
	return filepath.Join(rootPath, "shinkrodb.db")
}
