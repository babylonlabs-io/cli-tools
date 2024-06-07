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

		globalParamPath, err := cmd.Flags().GetString(globalParamKey)

		if err != nil {
			return err
		}

		parsedGlobalParams, err := services.NewVersionedParamsRetriever(globalParamPath)

		if err != nil {
			return err
		}

		log := logger.DefaultLogger()

		pipeLine, err := services.NewUnbondingPipelineFromConfig(log, cfg, parsedGlobalParams)

		if err != nil {
			return err
		}

		return pipeLine.ProcessNewTransactions(context.Background())
	},
}
