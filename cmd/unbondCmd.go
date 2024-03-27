package cmd

import (
	"fmt"

	"github.com/babylonchain/cli-tools/internal/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(runUnbondingPipelineCmd)
}

var runUnbondingPipelineCmd = &cobra.Command{
	Use:   "run-unbonding-pipeline",
	Short: "runs unbonding pipeline",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := cmd.Flags().GetString(configPathKey)
		if err != nil {
			return err
		}

		cfg, err := config.GetConfig(path)

		if err != nil {
			return err
		}

		fmt.Printf("Running unbonding pipeline with config file path: %s \n", path)
		fmt.Println(cfg.Db.DbName)

		return nil
	},
}
