package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/spf13/cobra"
)

const (
	FlagFeeInTx = "fee-in-tx"
)

type TimestampFileOutput struct {
	TimestampTx string `json:"timestamp_tx_hex"`
	FileHash    string `json:"file_hash"`
}

func init() {
	_ = btcTimestampFileCmd.Flags().Int64(FlagFeeInTx, 2000, "the amount of satoshi to pay as fee for the tx")
	_ = btcTimestampFileCmd.Flags().String(FlagNetwork, "signet", "network one of (mainnet, testnet3, regtest, simnet, signet)")

	rootCmd.AddCommand(btcTimestampFileCmd)
}

var btcTimestampFileCmd = &cobra.Command{
	Use:     "create-timestamp-transaction [funded-tx-addr-hex] [file-path] [address]",
	Example: `cli-tools create-timestamp-transaction [funded-tx-addr-hex] ./path/to/file/to/timestamp 836e9fc730ff37de48f2ff3a76b3c2380fbabaf66d9e50754d86b2a2e2952156`,
	Short:   "Creates a timestamp btc transaction by hashing the file input.",
	Long: `Creates a timestamp BTC transaction with 2 outputs and one input.
	One output is the nullDataScript of the file hash, as the file hash
	being the sha256 of the input file path. This output is the timestamp of the file.
	The other output is the pay to addr script which contains the pay to witness pubkey
  with the value as ({funded-tx-output-value} - {FlagFeeInTx}). This output is needed
	to continue to have spendable funds to the p2wpkh address.`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		fundedTxHex, inputFilePath, addressStr := args[0], args[1], args[2]
		flags := cmd.Flags()
		feeInTx, err := flags.GetInt64(FlagFeeInTx)
		if err != nil {
			return fmt.Errorf("failed to parse flag %s: %w", FlagFeeInTx, err)
		}

		networkParamStr, err := flags.GetString(FlagNetwork)
		if err != nil {
			return fmt.Errorf("failed to parse flag %s: %w", FlagNetwork, err)
		}

		btcParams, err := getBtcNetworkParams(networkParamStr)
		if err != nil {
			return fmt.Errorf("unable parse BTC network %s: %w", networkParamStr, err)
		}

		timestampOutput, err := CreateTimestampTx(fundedTxHex, inputFilePath, addressStr, feeInTx, btcParams)
		if err != nil {
			return fmt.Errorf("failed to create timestamping tx: %w", err)
		}

		PrintRespJSON(timestampOutput)
		return nil
	},
}

func outputIndexForPkScript(pkScript []byte, tx *wire.MsgTx) (int, error) {
	for i, txOut := range tx.TxOut {
		if bytes.Equal(txOut.PkScript, pkScript) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("unable to find output index for pk script")
}

// CreateTimestampTx outputs the hash of file and BTC transaction that timestamp that
// hash in one of the outputs. The funded tx needs to have one output with value
// for the changeAddress p2wpkh. The changeAddress needs to be a EncodeAddress
// which is the encoding of the payment address associated with the Address value,
// to be used to generate a pay to address script pay-to-witness-pubkey-hash (P2WKH) format.
func CreateTimestampTx(
	fundedTxHex, filePath, changeAddress string,
	fee int64,
	networkParams *chaincfg.Params,
) (*TimestampFileOutput, error) {
	txOutFileHash, fileHash, err := txOutTimestampFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to create tx out with filepath %s: %w", filePath, err)
	}

	fundingTx, _, err := newBTCTxFromHex(fundedTxHex)
	if err != nil {
		return nil, fmt.Errorf("unable parse BTC Tx %s: %w", fundedTxHex, err)
	}

	address, err := btcutil.DecodeAddress(changeAddress, networkParams)
	if err != nil {
		return nil, fmt.Errorf("invalid address %s: %w", changeAddress, err)
	}

	addressPkScript, err := txscript.PayToAddrScript(address)
	if err != nil {
		return nil, fmt.Errorf("unable to create pk script from address %s: %w", changeAddress, err)
	}

	if !txscript.IsPayToWitnessPubKeyHash(addressPkScript) {
		return nil, fmt.Errorf("address %s is not a pay-to-witness-pubkey-hash", changeAddress)
	}

	fundingOutputIdx, err := outputIndexForPkScript(addressPkScript, fundingTx)
	if err != nil {
		return nil, fmt.Errorf("unable to find output index for pk script: %w", err)
	}
	fundingTxHash := fundingTx.TxHash()
	fundingInput := wire.NewTxIn(
		wire.NewOutPoint(&fundingTxHash, uint32(fundingOutputIdx)),
		nil,
		nil,
	)

	valueIn := fundingTx.TxOut[fundingOutputIdx].Value
	if valueIn < fee {
		return nil, fmt.Errorf("the value of input in %d is bigger than the fee %d", valueIn, fee)
	}

	changeOutput := wire.NewTxOut(
		valueIn-fee,
		addressPkScript,
	)

	timestampTx := wire.NewMsgTx(2)
	timestampTx.AddTxIn(fundingInput)
	timestampTx.AddTxOut(changeOutput)
	timestampTx.AddTxOut(txOutFileHash)

	txHex, err := SerializeBTCTxToHex(timestampTx)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize timestamping tx: %w", err)
	}

	return &TimestampFileOutput{
		TimestampTx: txHex,
		FileHash:    hex.EncodeToString(fileHash),
	}, nil
}

func txOutTimestampFile(filePath string) (txOut *wire.TxOut, fileHash []byte, err error) {
	fileHash, err = hashFromFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate hash from file %s: %w", filePath, err)
	}

	dataScript, err := txscript.NullDataScript(fileHash)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create op return with hash from file %s: %w", fileHash, err)
	}

	return wire.NewTxOut(0, dataScript), fileHash, nil
}

func hashFromFile(filePath string) ([]byte, error) {
	h := sha256.New()

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open the file %s: %w", filePath, err)
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}
