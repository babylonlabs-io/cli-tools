package cmd

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/babylonlabs-io/babylon/btcstaking"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/spf13/cobra"

	"github.com/babylonlabs-io/cli-tools/internal/btcclient"
	"github.com/babylonlabs-io/cli-tools/internal/config"
)

var (
	FlagWithdrawTxFee         = "withdraw-tx-fee"
	FlagWithdrawTxDestination = "withdraw-tx-destination"
	FlagUnbondingTxHex        = "unbonding-tx-hex"
)

func init() {
	// It is important that staking tx is already funded ie. has inputs. Othersie
	// parsing it wil fail
	createWithdrawCmd.Flags().String(FlagStakingTxHex, "", "funded staking tx hex")
	_ = createWithdrawCmd.MarkFlagRequired(FlagStakingTxHex)
	createWithdrawCmd.Flags().String(FlagTag, "", "tag")
	_ = createWithdrawCmd.MarkFlagRequired(FlagTag)
	createWithdrawCmd.Flags().Int64(FlagWithdrawTxFee, 0, "withdraw fee")
	_ = createWithdrawCmd.MarkFlagRequired(FlagWithdrawTxFee)
	createWithdrawCmd.Flags().StringSlice(FlagCovenantCommitteePks, nil, "covenant committee pks")
	_ = createWithdrawCmd.MarkFlagRequired(FlagCovenantCommitteePks)
	createWithdrawCmd.Flags().Int64(FlagCovenantQuorum, 0, "covenant quorum")
	_ = createWithdrawCmd.MarkFlagRequired(FlagCovenantQuorum)
	createWithdrawCmd.Flags().String(FlagWithdrawTxDestination, "", "withdraw tx destination")
	_ = createWithdrawCmd.MarkFlagRequired(FlagWithdrawTxDestination)
	createWithdrawCmd.Flags().String(FlagNetwork, "", "network one of (mainnet, testnet3, regtest, simnet, signet)")
	_ = createWithdrawCmd.MarkFlagRequired(FlagNetwork)

	createWithdrawCmd.Flags().String(FlagUnbondingTxHex, "", "unbonding tx hex. If set, we will build withdraw tx from unbodning tx.")
	// This need to be set to correct value if want to withdraw from unbonding tx, otherwise tx will
	// fail when sent
	createWithdrawCmd.Flags().Int64(FlagUnbondingTime, 0, "unbonding time")

	// If those flags are provided we will sign the withdraw tx with staker wallet
	createWithdrawCmd.Flags().String(FlagStakerWalletAddressHost, "", "staker wallet address host")
	createWithdrawCmd.Flags().String(FlagStakerWalletRpcUser, "", "staker wallet rpc user")
	createWithdrawCmd.Flags().String(FlagStakerWalletRpcPass, "", "staker wallet rpc pass")
	createWithdrawCmd.Flags().String(FlagWalletPassphrase, "", "wallet passphrase")

	rootCmd.AddCommand(createWithdrawCmd)
}

type CreateWithdrawResponse struct {
	WitdrawTxHex string `json:"withdraw_tx_hex"`
	// Signed will be true if we signed the tx with staker wallet
	Signed bool `json:"signed"`
}

type spendStakeTxInfo struct {
	spendStakeTx           *wire.MsgTx
	fundingOutput          *wire.TxOut
	fundingOutputSpendInfo *btcstaking.SpendInfo
	fee                    btcutil.Amount
}

func parseRPCFlagsAndGetBTCClient(cmd *cobra.Command, btcParams *chaincfg.Params) (*btcclient.BtcClient, *rpcclient.Client, error) {
	host, err := cmd.Flags().GetString(FlagStakerWalletAddressHost)

	if err != nil {
		return nil, nil, err
	}

	if host == "" {
		return nil, nil, nil
	}

	rpcUser, err := cmd.Flags().GetString(FlagStakerWalletRpcUser)

	if err != nil {
		return nil, nil, err
	}

	rpcPass, err := cmd.Flags().GetString(FlagStakerWalletRpcPass)

	if err != nil {
		return nil, nil, err
	}

	client, err := btcclient.NewBtcClient(&config.BtcConfig{
		Host:    host,
		User:    rpcUser,
		Pass:    rpcPass,
		Network: btcParams.Name,
	})

	if err != nil {
		return nil, nil, err
	}

	// same as btcConfigToConnConfig that NewBtcClient uses but since it's not exposed, we need to create it again
	connConfig := &rpcclient.ConnConfig{
		Host:                 host,
		User:                 rpcUser,
		Pass:                 rpcPass,
		DisableTLS:           true,
		DisableConnectOnNew:  true,
		DisableAutoReconnect: false,
		HTTPPostMode:         true,
	}

	rpcClient, err := rpcclient.New(connConfig, nil)

	if err != nil {
		return nil, nil, err
	}
	return client, rpcClient, nil
}

var createWithdrawCmd = &cobra.Command{
	Use:   "create-phase1-withdaw-request",
	Short: "create phase1 withdraw tx ",
	RunE: func(cmd *cobra.Command, args []string) error {
		btcParams, err := getBtcNetworkParams(mustGetStringFlag(cmd, FlagNetwork))

		if err != nil {
			return err
		}

		tag, err := parseTagFromHex(mustGetStringFlag(cmd, FlagTag))

		if err != nil {
			return err
		}

		stakingTxHex := mustGetStringFlag(cmd, FlagStakingTxHex)

		stakingTx, _, err := newBTCTxFromHex(stakingTxHex)

		if err != nil {
			if strings.Contains(err.Error(), "witness tx but flag byte is 02") {
				_, rpcClient, err := parseRPCFlagsAndGetBTCClient(cmd, btcParams)

				if err != nil {
					return err
				}

				defer rpcClient.Shutdown()
				stakingTxBytes, err := hex.DecodeString(stakingTxHex)

				if err != nil {
					return err
				}
				rawTxResult, err := rpcClient.DecodeRawTransaction(stakingTxBytes)

				if err != nil {
					return err
				}

				stakingTx = wire.NewMsgTx(int32(rawTxResult.Version))
				stakingTx.LockTime = uint32(rawTxResult.LockTime)
				// Convert each Vin to wire.TxIn
				stakingTx.TxIn = make([]*wire.TxIn, len(rawTxResult.Vin))
				for i, vin := range rawTxResult.Vin {
					txid, err := chainhash.NewHashFromStr(vin.Txid)
					if err != nil {
						return err
					}
					outPoint := wire.NewOutPoint(txid, vin.Vout)
					scriptSigBytes, err := hex.DecodeString(vin.ScriptSig.Hex)
					if err != nil {
						return err
					}
					witness := make([][]byte, len(vin.Witness))
					for i, w := range vin.Witness {
						witness[i], err = hex.DecodeString(w)
						if err != nil {
							return err
						}
					}

					txIn := wire.NewTxIn(outPoint, scriptSigBytes, witness)
					stakingTx.TxIn[i] = txIn
				}

				// Convert each Vout to wire.TxOut
				stakingTx.TxOut = make([]*wire.TxOut, len(rawTxResult.Vout))
				for i, vout := range rawTxResult.Vout {
					pkScript, err := hex.DecodeString(vout.ScriptPubKey.Hex)
					if err != nil {
						return err
					}
					txOut := wire.NewTxOut(int64(vout.Value*1e8), pkScript)
					stakingTx.TxOut[i] = txOut
				}
			} else {
				return err
			}
		}

		covenantCommitteePks, err := parseCovenantKeysFromSlice(mustGetStringSliceFlag(cmd, FlagCovenantCommitteePks))

		if err != nil {
			return err
		}

		covenantQuorum, err := parsePosNum(mustGetInt64Flag(cmd, FlagCovenantQuorum))

		if err != nil {
			return err
		}

		withdrawTxFee, err := parseBtcAmount(mustGetInt64Flag(cmd, FlagWithdrawTxFee))

		if err != nil {
			return err
		}

		withdrawalDestination, err := btcutil.DecodeAddress(mustGetStringFlag(cmd, FlagWithdrawTxDestination), btcParams)

		if err != nil {
			return err
		}

		withdrawPkScript, err := txscript.PayToAddrScript(withdrawalDestination)

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

		unbondingTxHex, err := cmd.Flags().GetString(FlagUnbondingTxHex)

		if err != nil {
			return err
		}

		var info *spendStakeTxInfo
		if unbondingTxHex != "" {
			// unbonding timelock must be specified if we withdraw from unbonding output
			unbondingTime, err := parseTimeLock(mustGetInt64Flag(cmd, FlagUnbondingTime))

			if err != nil {
				return err
			}

			// here we are withdrawing from unbonding tx
			unbondingTx, _, err := newBTCTxFromHex(unbondingTxHex)

			if err != nil {
				return err
			}

			unbondingTxHash := unbondingTx.TxHash()

			// unbonding tx must have only one output
			if int64(withdrawTxFee) >= unbondingTx.TxOut[0].Value {
				return fmt.Errorf("withdraw fee is too high")
			}
			withdrawValue := unbondingTx.TxOut[0].Value - int64(withdrawTxFee)

			withdrawOutput := wire.NewTxOut(
				withdrawValue,
				withdrawPkScript,
			)

			unbondingTxInput := wire.NewOutPoint(
				&unbondingTxHash,
				0,
			)

			wTx := wire.NewMsgTx(2)
			wTx.AddTxIn(wire.NewTxIn(unbondingTxInput, nil, nil))
			wTx.AddTxOut(withdrawOutput)
			// we need to set sequence to unbonding time to properly unlock the timeloc
			wTx.TxIn[0].Sequence = uint32(unbondingTime)

			unbondingInfo, err := btcstaking.BuildUnbondingInfo(
				parsedStakingTx.OpReturnData.StakerPublicKey.PubKey,
				[]*btcec.PublicKey{parsedStakingTx.OpReturnData.FinalityProviderPublicKey.PubKey},
				covenantCommitteePks,
				covenantQuorum,
				unbondingTime,
				btcutil.Amount(unbondingTx.TxOut[0].Value),
				btcParams,
			)

			if err != nil {
				return err
			}

			timelockPathInfo, err := unbondingInfo.TimeLockPathSpendInfo()

			if err != nil {
				return err
			}

			info = &spendStakeTxInfo{
				spendStakeTx:           wTx,
				fundingOutput:          unbondingTx.TxOut[0],
				fundingOutputSpendInfo: timelockPathInfo,
				fee:                    withdrawTxFee,
			}
		} else {
			// here we are withdrawing from staking tx
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

			if int64(withdrawTxFee) >= parsedStakingTx.StakingOutput.Value {
				return fmt.Errorf("withdraw fee is too high")
			}
			withdrawValue := parsedStakingTx.StakingOutput.Value - int64(withdrawTxFee)

			withdrawOutput := wire.NewTxOut(
				withdrawValue,
				withdrawPkScript,
			)

			wTx := wire.NewMsgTx(2)
			wTx.AddTxIn(wire.NewTxIn(stakingTxInput, nil, nil))
			wTx.AddTxOut(withdrawOutput)
			// we need to set sequence to staking time to properly unlock the timeloc
			wTx.TxIn[0].Sequence = uint32(parsedStakingTx.OpReturnData.StakingTime)

			timelockInfo, err := stakingInfo.TimeLockPathSpendInfo()

			if err != nil {
				return err
			}

			info = &spendStakeTxInfo{
				spendStakeTx:           wTx,
				fundingOutput:          parsedStakingTx.StakingOutput,
				fundingOutputSpendInfo: timelockInfo,
				fee:                    withdrawTxFee,
			}
		}

		// at this point we created unsigned withdraw tx lets create response
		serializedWithdrawTx, err := SerializeBTCTxToHex(info.spendStakeTx)

		if err != nil {
			return err
		}

		resp := &CreateWithdrawResponse{
			WitdrawTxHex: serializedWithdrawTx,
			Signed:       false,
		}

		// whatever happens now, we will print out the response
		defer func() {
			PrintRespJSON(resp)
		}()

		// we will try to sign our withdraw tx with staker wallet and create valid
		// witness
		// Note this signing approach works only with legacy bitcoind wallets as
		// in new desciptor wallets we cannot dump private key from address
		client, _, err := parseRPCFlagsAndGetBTCClient(cmd, btcParams)

		if err != nil {
			return err
		}

		passphrase, err := cmd.Flags().GetString(FlagWalletPassphrase)

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

		stakerSig, err := btcstaking.SignTxWithOneScriptSpendInputFromTapLeaf(
			info.spendStakeTx,
			info.fundingOutput,
			stakerPrivKey,
			info.fundingOutputSpendInfo.RevealedLeaf,
		)

		if err != nil {
			return err
		}

		witness, err := info.fundingOutputSpendInfo.CreateTimeLockPathWitness(
			stakerSig,
		)

		if err != nil {
			return err
		}

		// attach witness to spend stake tx
		info.spendStakeTx.TxIn[0].Witness = witness

		// serialize tx with witness

		serializedWithdrawTx, err = SerializeBTCTxToHex(info.spendStakeTx)

		if err != nil {
			return err
		}

		resp.WitdrawTxHex = serializedWithdrawTx
		resp.Signed = true

		return nil
	},
}
