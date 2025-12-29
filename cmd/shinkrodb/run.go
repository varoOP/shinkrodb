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
6. Creates TVDB mapping files`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rootPath := viper.GetString("root_path")

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
	rootCmd.AddCommand(runCmd)
}

