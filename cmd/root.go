package cmd

import (
	"path/filepath"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/spf13/cobra"
)

var (
	// Used for flags.
	configPath    string
	configPathKey = "config"

	rootCmd = &cobra.Command{
		Use:   "cli-tools",
		Short: "Set of cli tools to run batch jobs on phase-1 mainnet",
	}

	//   C:\Users\<username>\AppData\Local\tools on Windows
	//   ~/.tools on Linux
	//   ~/Library/Application Support/tools on MacOS
	dafaultConfigDir  = btcutil.AppDataDir("tools", false)
	dafaultConfigPath = filepath.Join(dafaultConfigDir, "config.toml")
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, configPathKey, dafaultConfigPath, "path to the directory with configuration file")
}
