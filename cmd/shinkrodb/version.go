package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version number and build information for shinkrodb.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("shinkrodb: %v\n", version)
		if commit != "" {
			fmt.Printf("Commit: %v\n", commit)
		}
		if date != "" {
			fmt.Printf("Build Date: %v\n", date)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

