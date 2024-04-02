package cmd

import (
	"context"

	"github.com/babylonchain/cli-tools/internal/config"
	"github.com/babylonchain/cli-tools/internal/logger"
	"github.com/babylonchain/cli-tools/internal/services"

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

		log := logger.DefaultLogger()

		pipeLine, err := services.NewUnbondingPipelineFromConfig(log, cfg)

		if err != nil {
			return err
		}

		return pipeLine.Run(context.Background())
	},
}
