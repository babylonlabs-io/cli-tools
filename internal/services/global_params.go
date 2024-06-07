package services

import (
	"context"
	"fmt"

	"github.com/babylonchain/networks/parameters/parser"
)

type VersionedParamsRetriever struct {
	*parser.ParsedGlobalParams
}

func NewVersionedParamsRetriever(path string) (*VersionedParamsRetriever, error) {
	parsedGlobalParams, err := parser.NewParsedGlobalParamsFromFile(path)
	if err != nil {
		return nil, err
	}
	return &VersionedParamsRetriever{parsedGlobalParams}, nil
}
func (g *VersionedParamsRetriever) ParamsByHeight(_ context.Context, height uint64) (*SystemParams, error) {
	versionedParams := g.ParsedGlobalParams.GetVersionedGlobalParamsByHeight(height)
	if versionedParams == nil {
		return nil, fmt.Errorf("no global params for height %d", height)
	}

	return &SystemParams{
		CovenantPublicKeys: versionedParams.CovenantPks,
		CovenantQuorum:     versionedParams.CovenantQuorum,
		MagicBytes:         versionedParams.Tag,
	}, nil
}
