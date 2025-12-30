package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
	cfgFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "shinkrodb",
	Short: "A database for use with shinkro",
	Long: `ShinkroDB is a tool for building and maintaining anime databases
by aggregating data from MyAnimeList, AniDB, TVDB, and TMDB.`,
	Version: version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/shinkrodb/config.toml)")
	rootCmd.PersistentFlags().String("root-path", ".", "the path where output is saved")

	// Bind flags to viper
	viper.BindPFlag("root_path", rootCmd.PersistentFlags().Lookup("root-path"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Environment variables
	viper.SetEnvPrefix("SHINKRODB")
	viper.AutomaticEnv()

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
		viper.SetConfigType("toml")
	} else {
		// Default to $HOME/.config/shinkrodb/config.toml
		home, err := os.UserHomeDir()
		if err == nil {
			defaultConfigPath := filepath.Join(home, ".config", "shinkrodb", "config.toml")
			viper.SetConfigFile(defaultConfigPath)
			viper.SetConfigType("toml")
		} else {
			// Fallback to current directory if home directory can't be determined
			viper.SetConfigFile("./config.toml")
			viper.SetConfigType("toml")
		}
	}

	// If a config file is found, read it in (config file is optional)
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

