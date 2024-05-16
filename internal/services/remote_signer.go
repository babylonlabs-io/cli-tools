package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/babylonchain/covenant-signer/signerservice"

	"github.com/babylonchain/cli-tools/internal/config"
)

type RemoteSigner struct {
	pkToUrlMap map[string]string
	timeout    time.Duration
}

func NewRemoteSigner(cfg *config.ParsedRemoteSignerConfig) (*RemoteSigner, error) {
	pkToUrlMap, err := cfg.GetPubKeyToUrlMap()
	if err != nil {
		return nil, err
	}
	return &RemoteSigner{
		pkToUrlMap: pkToUrlMap,
		timeout:    cfg.Timeout,
	}, nil
}

func (rs *RemoteSigner) SignUnbondingTransaction(req *SignRequest) (*PubKeySigPair, error) {
	pkStr := hex.EncodeToString(req.SignerPubKey.SerializeCompressed())
	url, ok := rs.pkToUrlMap[pkStr]
	if !ok {
		return nil, fmt.Errorf("cannot find the url of the requested signer %s", pkStr)
	}

	sig, err := signerservice.RequestCovenantSignaure(
		context.Background(),
		url,
		rs.timeout,
		req.UnbondingTransaction,
		req.StakerUnbondingSig,
		req.SignerPubKey,
		req.UnbondingScript,
	)

	if err != nil {
		return nil, fmt.Errorf("request to coventant %s, with url:%s, failed: %w", pkStr, url, err)
	}

	return &PubKeySigPair{
		Signature: sig,
		PubKey:    req.SignerPubKey,
	}, nil
}
