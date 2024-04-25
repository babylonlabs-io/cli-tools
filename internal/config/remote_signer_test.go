package config

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name        string
	cfg         *RemoteSignerConfig
	expectedErr bool
}

func FuzzRemoteSignerConfig_Parse(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		var testCases []*testCase

		sk, err := btcec.NewPrivateKey()
		require.NoError(t, err)
		validPublicKey := hex.EncodeToString(sk.PubKey().SerializeCompressed())
		validPort := 9321
		validTimeout := 1 * time.Second

		case1 := &testCase{
			name: "valid url",
			cfg: &RemoteSignerConfig{
				Urls: []string{
					fmt.Sprintf("http://%s@127.0.0.1:%d", validPublicKey, validPort),
				},
				Timeout: validTimeout,
			},
			expectedErr: false,
		}
		testCases = append(testCases, case1)

		case2 := &testCase{
			name: "invalid public key",
			cfg: &RemoteSignerConfig{
				Urls: []string{
					fmt.Sprintf("http://%s@127.0.0.1:%d",
						datagen.GenRandomHexStr(r, datagen.RandomInt(r, 100)+1), validPort),
				},
				Timeout: validTimeout,
			},
			expectedErr: true,
		}
		testCases = append(testCases, case2)

		case3 := &testCase{
			name: "invalid host",
			cfg: &RemoteSignerConfig{
				Urls: []string{
					fmt.Sprintf("http:%s@127.0.0.1.1:%d",
						validPublicKey, validPort),
				},
				Timeout: validTimeout,
			},
			expectedErr: true,
		}
		testCases = append(testCases, case3)

		case4 := &testCase{
			name: "invalid timeout",
			cfg: &RemoteSignerConfig{
				Urls: []string{
					fmt.Sprintf("http://%s@127.0.0.1:%d", validPublicKey, validPort),
				},
				Timeout: 0,
			},
			expectedErr: true,
		}
		testCases = append(testCases, case4)

		for _, tc := range testCases {
			parsedCfg, err := tc.cfg.Parse()
			if tc.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				_, err = parsedCfg.GetPubKeyToUrlMap()
				require.NoError(t, err)
			}
		}
	})
}
