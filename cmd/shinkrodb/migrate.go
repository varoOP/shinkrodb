package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/varoOP/shinkrodb/internal/cache"
	"github.com/varoOP/shinkrodb/internal/domain"
	"github.com/varoOP/shinkrodb/internal/logger"
	"github.com/varoOP/shinkrodb/internal/repository"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate-cache",
	Short: "Migrate existing HTML cache to SQLite database",
	Long: `Migrate existing Colly HTML cache files to a new SQLite-based cache.
This is a one-time migration command that reads all HTML files from the cache directory,
extracts MAL IDs and AniDB IDs, and stores them in shinkrodb.db.

After migration, you can use the new efficient cache system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cacheDir := viper.GetString("cache_dir")
		rootPath := viper.GetString("root_path")

		// Create database path in root directory
		dbPath := filepath.Join(rootPath, "shinkrodb.db")

		log := logger.NewLogger()

		log.Info().
			Str("cache_dir", cacheDir).
			Str("db_path", dbPath).
			Msg("Starting cache migration")

		// Initialize repository to get anime data (for release date/type from MAL API)
		animeRepo := repository.NewFileRepository(log)
		paths := domain.NewPaths(rootPath)

		// Run migration - use MAL ID path since that's where release dates come from initially
		if err := cache.MigrateCache(cmd.Context(), cacheDir, dbPath, animeRepo, paths.MalIDPath, log); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}

		log.Info().Msg("Migration completed successfully!")
		fmt.Printf("\n✓ Cache migration complete!\n")
		fmt.Printf("  Database: %s\n", dbPath)
		fmt.Printf("  You can now use the new cache system.\n")
		fmt.Printf("  Old cache directory (%s) can be kept for backup or removed.\n\n", cacheDir)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

