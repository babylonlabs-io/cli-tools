package cmd

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"

	"github.com/babylonchain/babylon/btcstaking"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/spf13/cobra"
)

var (
	FlagMagicBytes           = "magic-bytes"
	FlagStakerPk             = "staker-pk"
	FlagStakingAmount        = "staking-amount"
	FlagStakingTime          = "staking-time"
	FlagCovenantCommitteePks = "covenant-committee-pks"
	FlagCovenantQuorum       = "covenant-quorum"
	FlagNetwork              = "network"
	FlagFinalityProviderPk   = "finality-provider-pk"
)

// PrintRespJSON receive an interface and parses it into json for print
func PrintRespJSON(resp interface{}) {
	jsonBytes, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		fmt.Println("unable to decode response: ", err)
		return
	}

	fmt.Printf("%s\n", jsonBytes)
}

func newBTCTxFromBytes(txBytes []byte) (*wire.MsgTx, error) {
	var msgTx wire.MsgTx
	rbuf := bytes.NewReader(txBytes)
	if err := msgTx.Deserialize(rbuf); err != nil {
		return nil, err
	}

	return &msgTx, nil
}

func newBTCTxFromHex(txHex string) (*wire.MsgTx, []byte, error) {
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, nil, err
	}

	parsed, err := newBTCTxFromBytes(txBytes)

	if err != nil {
		return nil, nil, err
	}

	return parsed, txBytes, nil
}

func serializeBTCTx(tx *wire.MsgTx) ([]byte, error) {
	var txBuf bytes.Buffer
	if err := tx.Serialize(&txBuf); err != nil {
		return nil, err
	}
	return txBuf.Bytes(), nil
}

func serializeBTCTxToHex(tx *wire.MsgTx) (string, error) {
	bytes, err := serializeBTCTx(tx)

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func mustGetStringFlag(cmd *cobra.Command, name string) string {
	val, err := cmd.Flags().GetString(name)
	if err != nil {
		panic(err)
	}
	return val
}

func mustGetInt64Flag(cmd *cobra.Command, name string) int64 {
	val, err := cmd.Flags().GetInt64(name)
	if err != nil {
		panic(err)
	}
	return val
}

func mustGetStringSliceFlag(cmd *cobra.Command, name string) []string {
	val, err := cmd.Flags().GetStringSlice(name)
	if err != nil {
		panic(err)
	}
	return val
}

func getBtcNetworkParams(network string) (*chaincfg.Params, error) {
	switch network {
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
		return nil, fmt.Errorf("unknown network %s", network)
	}
}

func parsePubKeyFromHex(pkHex string) (*btcec.PublicKey, error) {
	pkBytes, err := hex.DecodeString(pkHex)
	if err != nil {
		return nil, err
	}

	pk, err := btcec.ParsePubKey(pkBytes)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

func parseCovenantKeysFromSlice(covenantMembersPks []string) ([]*btcec.PublicKey, error) {
	covenantPubKeys := make([]*btcec.PublicKey, len(covenantMembersPks))

	for i, fpPk := range covenantMembersPks {
		covPk, err := parsePubKeyFromHex(fpPk)

		if err != nil {
			return nil, err
		}
		covenantPubKeys[i] = covPk
	}

	return covenantPubKeys, nil
}

func parseBtcAmount(amt int64) (btcutil.Amount, error) {
	if amt < 0 {
		return 0, fmt.Errorf("amount should be greater or equal to 0")
	}

	return btcutil.Amount(amt), nil
}

func parseTimeLock(timeBlocks int64) (uint16, error) {
	if timeBlocks <= 0 {
		return 0, fmt.Errorf("staking time blocks should be greater than 0")
	}

	if timeBlocks > math.MaxUint16 {
		return 0, fmt.Errorf("staking time blocks should be less or equal to %d", math.MaxUint16)
	}

	return uint16(timeBlocks), nil
}

func parseMagicBytesFromHex(magicBytesHex string) ([]byte, error) {
	magicBytes, err := hex.DecodeString(magicBytesHex)
	if err != nil {
		return nil, err
	}

	if len(magicBytes) != btcstaking.MagicBytesLen {
		return nil, fmt.Errorf("magic bytes should be of length %d", btcstaking.MagicBytesLen)
	}

	return magicBytes, nil
}

func parsePosNum(num int64) (uint32, error) {
	if num < 0 {
		return 0, fmt.Errorf("number should be greater or equal to 0")
	}

	if num > math.MaxUint32 {
		return 0, fmt.Errorf("number should be less or equal to %d", math.MaxUint32)
	}

	return uint32(num), nil
}

type CreateStakingTxResp struct {
	StakingTxHex string `json:"staking_tx_hex"`
}

func init() {
	createStakingTxCmd.Flags().String(FlagMagicBytes, "", "magic bytes")
	_ = createStakingTxCmd.MarkFlagRequired(FlagMagicBytes)
	createStakingTxCmd.Flags().String(FlagStakerPk, "", "staker pk")
	_ = createStakingTxCmd.MarkFlagRequired(FlagStakerPk)
	createStakingTxCmd.Flags().String(FlagFinalityProviderPk, "", "finality provider pk")
	_ = createStakingTxCmd.MarkFlagRequired(FlagFinalityProviderPk)
	createStakingTxCmd.Flags().Int64(FlagStakingAmount, 0, "staking amount")
	_ = createStakingTxCmd.MarkFlagRequired(FlagStakingAmount)
	createStakingTxCmd.Flags().Int64(FlagStakingTime, 0, "staking time")
	_ = createStakingTxCmd.MarkFlagRequired(FlagStakingTime)
	createStakingTxCmd.Flags().StringSlice(FlagCovenantCommitteePks, nil, "covenant committee pks")
	_ = createStakingTxCmd.MarkFlagRequired(FlagCovenantCommitteePks)
	createStakingTxCmd.Flags().Int64(FlagCovenantQuorum, 0, "covenant quorum")
	_ = createStakingTxCmd.MarkFlagRequired(FlagCovenantQuorum)
	createStakingTxCmd.Flags().String(FlagNetwork, "", "network one of (mainnet, testnet3, regtest, simnet, signet)")
	_ = createStakingTxCmd.MarkFlagRequired(FlagNetwork)

	rootCmd.AddCommand(createStakingTxCmd)
}

var createStakingTxCmd = &cobra.Command{
	Use:   "create-phase1-staking-tx",
	Short: "create phase1 staking tx ",
	RunE: func(cmd *cobra.Command, args []string) error {
		btcParams, err := getBtcNetworkParams(mustGetStringFlag(cmd, FlagNetwork))

		if err != nil {
			return err
		}

		magicBytes, err := parseMagicBytesFromHex(mustGetStringFlag(cmd, FlagMagicBytes))

		if err != nil {
			return err
		}

		stakerPk, err := parsePubKeyFromHex(mustGetStringFlag(cmd, FlagStakerPk))

		if err != nil {
			return err
		}

		finalityProviderPk, err := parsePubKeyFromHex(mustGetStringFlag(cmd, FlagFinalityProviderPk))

		if err != nil {
			return err
		}

		stakingAmount, err := parseBtcAmount(mustGetInt64Flag(cmd, FlagStakingAmount))

		if err != nil {
			return err
		}

		stakingTime, err := parseTimeLock(mustGetInt64Flag(cmd, FlagStakingTime))

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

		_, tx, err := btcstaking.BuildV0IdentifiableStakingOutputsAndTx(
			magicBytes,
			stakerPk,
			finalityProviderPk,
			covenantCommitteePks,
			covenantQuorum,
			stakingTime,
			stakingAmount,
			btcParams,
		)
		if err != nil {
			return err
		}

		serializedTx, err := serializeBTCTxToHex(tx)
		if err != nil {
			return err
		}

		resp := CreateStakingTxResp{
			StakingTxHex: serializedTx,
		}
		PrintRespJSON(resp)
		return nil
	},
}
