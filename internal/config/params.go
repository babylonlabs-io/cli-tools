package config

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
)

type UnsafeParamsConfig struct {
	CovenantPrivateKeys []string `mapstructure:"covenant_private_keys"`
	CovenantQuorum      uint64   `mapstructure:"covenant_quorum"`
}

func DefaultUnsafeParamsConfig() *UnsafeParamsConfig {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		panic(err)
	}

	encoded := hex.EncodeToString(privKey.Serialize())

	return &UnsafeParamsConfig{
		CovenantPrivateKeys: []string{encoded},
		CovenantQuorum:      1,
	}
}

type ParsedUnsafeParamsConfig struct {
	CovenantPrivateKeys []*btcec.PrivateKey
	CovenantQuorum      uint32
}

func (cfg *UnsafeParamsConfig) Parse() (*ParsedUnsafeParamsConfig, error) {
	var covenantPrivateKeys []*btcec.PrivateKey

	for _, key := range cfg.CovenantPrivateKeys {
		decoded, err := hex.DecodeString(key)
		if err != nil {
			return nil, err
		}

		privKey, _ := btcec.PrivKeyFromBytes(decoded)
		covenantPrivateKeys = append(covenantPrivateKeys, privKey)
	}

	if len(covenantPrivateKeys) < int(cfg.CovenantQuorum) {
		return nil, fmt.Errorf("not enough private keys for the quorum")
	}

	return &ParsedUnsafeParamsConfig{
		CovenantPrivateKeys: covenantPrivateKeys,
		CovenantQuorum:      uint32(cfg.CovenantQuorum),
	}, nil
}
