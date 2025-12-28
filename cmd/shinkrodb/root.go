package main

import (
	"fmt"
	"os"

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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.shinkrodb.yaml or ./config.yaml)")
	rootCmd.PersistentFlags().String("root-path", ".", "the path where output is saved")
	rootCmd.PersistentFlags().String("cache-dir", "./mal_cache", "directory for caching MAL pages")

	// Bind flags to viper
	viper.BindPFlag("root_path", rootCmd.PersistentFlags().Lookup("root-path"))
	viper.BindPFlag("cache_dir", rootCmd.PersistentFlags().Lookup("cache-dir"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in home directory and current directory
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
		}
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".shinkrodb")
		viper.SetConfigName("config")
	}

	// Environment variables
	viper.SetEnvPrefix("SHINKRODB")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

