package cmd

import (
	"encoding/hex"
	"fmt"

	"github.com/babylonlabs-io/babylon/btcstaking"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/cli-tools/internal/btcclient"
	"github.com/babylonlabs-io/cli-tools/internal/config"
)

var (
	FlagUnbondingTime  = "unbonding-time"
	FlagUnbondingTxFee = "unbonding-fee"
	FlagStakingTxHex   = "staking-tx-hex"

	FlagStakerWalletAddressHost = "staker-wallet-address-host"
	FlagStakerWalletRpcUser     = "staker-wallet-rpc-user"
	//#nosec G101 - false positive
	FlagStakerWalletRpcPass = "staker-wallet-rpc-pass"
	FlagWalletPassphrase    = "staker-wallet-passphrase"
)

func init() {
	// It is important that staking tx is already funded ie. has inputs. Othersie
	// parsing it wil fail
	createUnbondingTxCmd.Flags().String(FlagStakingTxHex, "", "funded staking tx hex")
	_ = createUnbondingTxCmd.MarkFlagRequired(FlagStakingTxHex)
	createUnbondingTxCmd.Flags().String(FlagTag, "", "tag")
	_ = createUnbondingTxCmd.MarkFlagRequired(FlagTag)
	createUnbondingTxCmd.Flags().Int64(FlagUnbondingTime, 0, "unbonding time")
	_ = createUnbondingTxCmd.MarkFlagRequired(FlagUnbondingTime)
	createUnbondingTxCmd.Flags().Int64(FlagUnbondingTxFee, 0, "unbonding fee")
	_ = createUnbondingTxCmd.MarkFlagRequired(FlagUnbondingTxFee)
	createUnbondingTxCmd.Flags().StringSlice(FlagCovenantCommitteePks, nil, "covenant committee pks")
	_ = createUnbondingTxCmd.MarkFlagRequired(FlagCovenantCommitteePks)
	createUnbondingTxCmd.Flags().Int64(FlagCovenantQuorum, 0, "covenant quorum")
	_ = createUnbondingTxCmd.MarkFlagRequired(FlagCovenantQuorum)
	createUnbondingTxCmd.Flags().String(FlagNetwork, "", "network one of (mainnet, testnet3, regtest, simnet, signet)")
	_ = createUnbondingTxCmd.MarkFlagRequired(FlagNetwork)

	// If those flags are provided we will sign the unbonding tx with staker wallet
	createUnbondingTxCmd.Flags().String(FlagStakerWalletAddressHost, "", "staker wallet address host")
	createUnbondingTxCmd.Flags().String(FlagStakerWalletRpcUser, "", "staker wallet rpc user")
	createUnbondingTxCmd.Flags().String(FlagStakerWalletRpcPass, "", "staker wallet rpc pass")
	createUnbondingTxCmd.Flags().String(FlagWalletPassphrase, "", "wallet passphrase")

	rootCmd.AddCommand(createUnbondingTxCmd)
}

// response matches data required by staking-api-service
type CreateUnbondingResponse struct {
	StakingTxHashHex         string `json:"staking_tx_hash_hex"`
	UnbondingTxHex           string `json:"unbonding_tx_hex"`
	UnbondingTxHashHex       string `json:"unbonding_tx_hash_hex"`
	StakerSignedSignatureHex string `json:"staker_signed_signature_hex"`
}

func pubKeyToAddress(pubKey *btcec.PublicKey, net *chaincfg.Params) (btcutil.Address, error) {
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())
	witnessAddr, err := btcutil.NewAddressWitnessPubKeyHash(
		pubKeyHash, net,
	)

	if err != nil {
		return nil, err
	}
	return witnessAddr, nil
}

// to get staker key we need create two address as we do not know where original key
// is odd or ever.
// This assumes staker created his key using `getnewaddress“ rpc call which by default
// uses p2wpkh address
func getStakerPrivKey(
	client *btcclient.BtcClient,
	stakerXOnlyKey *btcstaking.XonlyPubKey,
	passphrase string,
	net *chaincfg.Params,
) (*btcec.PrivateKey, error) {
	// unlock wallet for 60 seconds
	if err := client.UnlockWallet(60, passphrase); err != nil {
		return nil, err
	}

	stakerKeyBytes := stakerXOnlyKey.Marshall()

	var keyCompressedEven [btcec.PubKeyBytesLenCompressed]byte
	keyCompressedEven[0] = secp256k1.PubKeyFormatCompressedEven
	copy(keyCompressedEven[1:], stakerKeyBytes)

	var PubKeyFormatCompressedOdd [btcec.PubKeyBytesLenCompressed]byte
	PubKeyFormatCompressedOdd[0] = secp256k1.PubKeyFormatCompressedOdd
	copy(PubKeyFormatCompressedOdd[1:], stakerKeyBytes)

	stakerKeyEven, err := btcec.ParsePubKey(keyCompressedEven[:])

	if err != nil {
		return nil, err
	}

	stakerKeyOdd, err := btcec.ParsePubKey(PubKeyFormatCompressedOdd[:])

	if err != nil {
		return nil, err
	}

	addressEven, err := pubKeyToAddress(stakerKeyEven, net)

	if err != nil {
		return nil, err
	}

	addressOdd, err := pubKeyToAddress(stakerKeyOdd, net)

	if err != nil {
		return nil, err
	}

	keyEven, err := client.DumpPrivateKey(addressEven)

	if err == nil {
		// it turned out that key is even retun early
		return keyEven, nil
	}

	return client.DumpPrivateKey(addressOdd)
}

var createUnbondingTxCmd = &cobra.Command{
	Use:   "create-phase1-unbonding-request",
	Short: "create phase1 unbonding tx ",
	RunE: func(cmd *cobra.Command, args []string) error {
		btcParams, err := getBtcNetworkParams(mustGetStringFlag(cmd, FlagNetwork))

		if err != nil {
			return err
		}

		tag, err := parseTagFromHex(mustGetStringFlag(cmd, FlagTag))

		if err != nil {
			return err
		}

		stakingTx, _, err := newBTCTxFromHex(mustGetStringFlag(cmd, FlagStakingTxHex))

		if err != nil {
			return err
		}

		covenantCommitteePks, err := parseCovenantKeysFromSlice(mustGetStringSliceFlag(cmd, FlagCovenantCommitteePks))

		if err != nil {
			return err
		}

		covenantQuorum, err := parsePosNum(mustGetInt64Flag(cmd, FlagCovenantQuorum))

		if err != nil {
			return err
		}

		unbondingTime, err := parseTimeLock(mustGetInt64Flag(cmd, FlagUnbondingTime))

		if err != nil {
			return err
		}

		unbondingFee, err := parseBtcAmount(mustGetInt64Flag(cmd, FlagUnbondingTxFee))

		if err != nil {
			return err
		}

		parsedStakingTx, err := btcstaking.ParseV0StakingTx(
			stakingTx,
			tag,
			covenantCommitteePks,
			covenantQuorum,
			btcParams,
		)

		if err != nil {
			return err
		}

		stakingTxHash := stakingTx.TxHash()

		stakingTxInput := wire.NewOutPoint(
			&stakingTxHash,
			uint32(parsedStakingTx.StakingOutputIdx),
		)

		// TODO: We should validate that fee is valid ie. matches babylon minimum
		// amount
		if int64(unbondingFee) >= parsedStakingTx.StakingOutput.Value {
			return fmt.Errorf("unbonding fee is too high")
		}

		ubondingOutputValue := parsedStakingTx.StakingOutput.Value - int64(unbondingFee)

		unbondingInfo, err := btcstaking.BuildUnbondingInfo(
			parsedStakingTx.OpReturnData.StakerPublicKey.PubKey,
			[]*btcec.PublicKey{parsedStakingTx.OpReturnData.FinalityProviderPublicKey.PubKey},
			covenantCommitteePks,
			covenantQuorum,
			unbondingTime,
			btcutil.Amount(ubondingOutputValue),
			btcParams,
		)

		if err != nil {
			return err
		}

		unbondingTx := wire.NewMsgTx(2)
		unbondingTx.AddTxIn(wire.NewTxIn(stakingTxInput, nil, nil))
		unbondingTx.AddTxOut(unbondingInfo.UnbondingOutput)

		unbondingTxHash := unbondingTx.TxHash()

		unbondingTxHex, err := SerializeBTCTxToHex(unbondingTx)

		if err != nil {
			return err
		}

		resp := &CreateUnbondingResponse{
			StakingTxHashHex:   stakingTxHash.String(),
			UnbondingTxHashHex: unbondingTxHash.String(),
			UnbondingTxHex:     unbondingTxHex,
		}

		// whatever happens now, we will pring out the response
		defer func() {
			PrintRespJSON(resp)
		}()

		// Note this signing approach works only with legacy bitcoind wallets as
		// in new desciptor wallets we cannot dump private key from address
		host, err := cmd.Flags().GetString(FlagStakerWalletAddressHost)

		if err != nil {
			return err
		}

		if host == "" {
			return nil
		}

		rpcUser, err := cmd.Flags().GetString(FlagStakerWalletRpcUser)

		if err != nil {
			return err
		}

		rpcPass, err := cmd.Flags().GetString(FlagStakerWalletRpcPass)

		if err != nil {
			return err
		}

		passphrase, err := cmd.Flags().GetString(FlagWalletPassphrase)

		if err != nil {
			return err
		}

		client, err := btcclient.NewBtcClient(&config.BtcConfig{
			Host:    host,
			User:    rpcUser,
			Pass:    rpcPass,
			Network: btcParams.Name,
		})

		if err != nil {
			return err
		}

		stakerPrivKey, err := getStakerPrivKey(
			client,
			parsedStakingTx.OpReturnData.StakerPublicKey,
			passphrase,
			btcParams,
		)

		if err != nil {
			return err
		}

		// We got the staker key we need to create signature for unbonding tx on unbonding path
		stakingInfo, err := btcstaking.BuildStakingInfo(
			parsedStakingTx.OpReturnData.StakerPublicKey.PubKey,
			[]*btcec.PublicKey{parsedStakingTx.OpReturnData.FinalityProviderPublicKey.PubKey},
			covenantCommitteePks,
			covenantQuorum,
			parsedStakingTx.OpReturnData.StakingTime,
			btcutil.Amount(parsedStakingTx.StakingOutput.Value),
			btcParams,
		)

		if err != nil {
			return err
		}

		unbondingSpendInfo, err := stakingInfo.UnbondingPathSpendInfo()

		if err != nil {
			return err
		}

		stakerSignature, err := btcstaking.SignTxWithOneScriptSpendInputFromScript(
			unbondingTx,
			parsedStakingTx.StakingOutput,
			stakerPrivKey,
			unbondingSpendInfo.RevealedLeaf.Script,
		)

		if err != nil {
			return err
		}

		// We signed the unbonding tx with staker key, add it to response
		resp.StakerSignedSignatureHex = hex.EncodeToString(stakerSignature.Serialize())

		return nil
	},
}
