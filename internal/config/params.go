package config

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
)

type ParamsConfig struct {
	CovenantPublicKeys []string `mapstructure:"covenant_public_keys"`
	CovenantQuorum     uint32   `mapstructure:"covenant_quorum"`
	MagicBytes         string   `mapstructure:"magic_bytes"`
}

func DefaultParamsConfig() *ParamsConfig {
	return &ParamsConfig{
		CovenantPublicKeys: []string{},
		CovenantQuorum:     0,
		MagicBytes:         "01020304",
	}
}

type ParsedParamsConfig struct {
	CovenantPublicKeys []*btcec.PublicKey
	CovenantQuorum     uint32
	MagicBytes         []byte
}

func (c *ParamsConfig) Parse() (*ParsedParamsConfig, error) {
	var covenantPublicKeys []*btcec.PublicKey

	for _, key := range c.CovenantPublicKeys {
		decodedBytes, err := hex.DecodeString(key)

		if err != nil {
			return nil, err
		}

		pk, err := btcec.ParsePubKey(decodedBytes)
		if err != nil {
			return nil, err
		}
		covenantPublicKeys = append(covenantPublicKeys, pk)
	}

	magicBytes, err := hex.DecodeString(c.MagicBytes)

	if err != nil {
		return nil, err
	}

	if len(magicBytes) != 4 {
		return nil, fmt.Errorf("invalid magic bytes length. Magic bytes should be 4 bytes long")
	}

	return &ParsedParamsConfig{
		CovenantPublicKeys: covenantPublicKeys,
		CovenantQuorum:     c.CovenantQuorum,
		MagicBytes:         magicBytes,
	}, nil
}
