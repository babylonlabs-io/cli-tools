package config

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
)

type BtcConfig struct {
	Host    string `mapstructure:"host"`
	User    string `mapstructure:"user"`
	Pass    string `mapstructure:"pass"`
	Network string `mapstructure:"network"`
}

func DefaultBtcConfig() *BtcConfig {
	return &BtcConfig{
		Host:    "localhost:18556",
		User:    "user",
		Pass:    "pass",
		Network: "regtest",
	}
}

func (cfg *BtcConfig) Validate() error {
	_, err := cfg.getBtcNetworkParams()

	if err != nil {
		return err
	}

	// TODO: implement host validation
	return nil
}

func (cfg *BtcConfig) getBtcNetworkParams() (*chaincfg.Params, error) {
	switch cfg.Network {
	case "testnet3":
		return &chaincfg.TestNet3Params, nil
	case "mainnet":
		return &chaincfg.MainNetParams, nil
	case "regtest":
		return &chaincfg.RegressionNetParams, nil
	case "simnet":
		return &chaincfg.SimNetParams, nil
	case "signet":
		return &chaincfg.SigNetParams, nil
	default:
		return nil, fmt.Errorf("unknown network %s", cfg.Network)
	}
}

func (cfg *BtcConfig) MustGetBtcNetworkParams() *chaincfg.Params {
	params, err := cfg.getBtcNetworkParams()

	if err != nil {
		panic(err)
	}

	return params
}
