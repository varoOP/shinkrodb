package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/varoOP/shinkrodb/internal/app"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the full database update process",
	Long: `Run performs a complete update of the anime database:
1. Fetches anime IDs from MyAnimeList
2. Scrapes AniDB IDs from MAL pages
3. Maps TVDB IDs from AniDB IDs
4. Maps TMDB IDs for movies
5. Checks for duplicates
6. Creates TVDB mapping files

Scraping mode can be configured via --scrape-mode flag or scrape_mode in config:
  - default: Only scrape MAL IDs without AniDB ID, released in past 1 year, type = "tv" (default)
  - missing: Scrape all MAL IDs without AniDB ID (no year/type filter)
  - all: Scrape everything, even if already has AniDB ID in cache
  - skip: Skip scraping entirely`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rootPath := viper.GetString("root_path")

		// Override scrape mode from CLI flag if provided
		if scrapeMode, _ := cmd.Flags().GetString("scrape-mode"); scrapeMode != "" {
			viper.Set("scrape_mode", scrapeMode)
		}

		// Initialize application
		application, err := app.NewApp()
		if err != nil {
			return fmt.Errorf("failed to initialize application: %w", err)
		}

		// Run the update process
		if err := application.Run(rootPath); err != nil {
			return fmt.Errorf("run failed: %w", err)
		}

		return nil
	},
}

func init() {
	runCmd.Flags().String("scrape-mode", "", "Scraping mode: 'default' (past year, tv only), 'missing' (all without AniDB ID), 'all' (everything), or 'skip' (skip scraping)")
	rootCmd.AddCommand(runCmd)
}

