package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
)

const (
	defaultHost    = "http://127.0.0.1"
	defaultPort    = 9791
	defaultTimeout = 2
)

var (
	privKey, _       = btcec.NewPrivateKey()
	defaultPublicKey = hex.EncodeToString(privKey.PubKey().SerializeCompressed())
	defaultUrls      = fmt.Sprintf("http://%s@%s:%d", defaultPublicKey, defaultHost, defaultPort)
)

type RemoteSignerConfig struct {
	Urls []string `mapstructure:"urls"` // in the format http://covenant_pk@signer_host:port
	// timeout in seconds
	TimeoutSeconds int `mapstructure:"timeout_seconds"`
}

type ParsedRemoteSignerConfig struct {
	Urls       []string // in the format http://signer_host:port
	PublicKeys []*btcec.PublicKey
	Timeout    time.Duration
}

func (c *RemoteSignerConfig) Parse() (*ParsedRemoteSignerConfig, error) {
	nUrls := len(c.Urls)
	if nUrls == 0 {
		return nil, fmt.Errorf("must have at least one url")
	}

	urls := make([]string, nUrls)
	publicKeys := make([]*btcec.PublicKey, nUrls)
	for i, urlStr := range c.Urls {
		parsedUrl, err := url.Parse(urlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid url %s: %w", urlStr, err)
		}
		urls[i] = fmt.Sprintf("%s://%s", parsedUrl.Scheme, parsedUrl.Host)

		publicKeyStr := parsedUrl.User.String()
		pkBytes, err := hex.DecodeString(publicKeyStr)
		if err != nil {
			return nil, fmt.Errorf("invalid public key %s: %w", publicKeyStr, err)
		}

		pk, err := btcec.ParsePubKey(pkBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid public key %s: %w", publicKeyStr, err)
		}

		publicKeys[i] = pk
	}

	if c.TimeoutSeconds <= 0 {
		return nil, fmt.Errorf("timeout %d should be positive", c.TimeoutSeconds)
	}

	return &ParsedRemoteSignerConfig{
		Urls:       urls,
		PublicKeys: publicKeys,
		Timeout:    time.Duration(c.TimeoutSeconds) * time.Second,
	}, nil
}

func (pc *ParsedRemoteSignerConfig) GetPubKeyToUrlMap() (map[string]string, error) {
	nUrls := len(pc.Urls)
	if nUrls == 0 {
		return nil, fmt.Errorf("must have at least one url")
	}

	nPubKyes := len(pc.PublicKeys)
	if nPubKyes == 0 {
		return nil, fmt.Errorf("must have at least one public key")
	}

	if nUrls != nPubKyes {
		return nil, fmt.Errorf("the number of urls %d must match the number of public keys %d", nUrls, nPubKyes)
	}

	mapPkToUrl := make(map[string]string)
	for i, u := range pc.Urls {
		pkStr := hex.EncodeToString(pc.PublicKeys[i].SerializeCompressed())
		mapPkToUrl[pkStr] = u
	}

	return mapPkToUrl, nil
}

func DefaultRemoteSignerConfig() *RemoteSignerConfig {
	return &RemoteSignerConfig{
		Urls:           []string{defaultUrls},
		TimeoutSeconds: defaultTimeout,
	}
}
