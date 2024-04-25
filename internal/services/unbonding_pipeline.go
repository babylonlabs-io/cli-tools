package services

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/babylonchain/cli-tools/internal/btcclient"
	"github.com/babylonchain/cli-tools/internal/config"
	"github.com/babylonchain/cli-tools/internal/db"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
)

var (
	// ErrCriticalError is returned only when there is some programming error in our
	// code, or we allowed some invalid data into database.
	// When this happend we stop processing pipeline and return immediately, without
	// changing status of any unbonding transaction.
	ErrCriticalError = fmt.Errorf("critical error encountered")
)

func wrapCrititical(err error) error {
	return fmt.Errorf("%s:%w", err.Error(), ErrCriticalError)
}

func pubKeyToString(pubKey *btcec.PublicKey) string {
	return hex.EncodeToString(schnorr.SerializePubKey(pubKey))
}

type SystemParamsRetriever struct {
	CovenantPublicKeys []*btcec.PublicKey
	CovenantQuorum     uint32
	MagicBytes         []byte
}

func NewSystemParamsRetriever(
	quorum uint32,
	pubKeys []*btcec.PublicKey,
	magicBytes []byte,
) *SystemParamsRetriever {
	return &SystemParamsRetriever{
		CovenantQuorum:     quorum,
		CovenantPublicKeys: pubKeys,
		MagicBytes:         magicBytes,
	}
}

func (p *SystemParamsRetriever) GetParams() (*SystemParams, error) {
	return &SystemParams{
		CovenantQuorum:     p.CovenantQuorum,
		CovenantPublicKeys: p.CovenantPublicKeys,
		MagicBytes:         p.MagicBytes,
	}, nil
}

type UnbondingPipeline struct {
	logger    *slog.Logger
	store     UnbondingStore
	signer    CovenantSigner
	sender    BtcSender
	retriever ParamsRetriever
	btcParams *chaincfg.Params
}

func NewUnbondingPipelineFromConfig(
	logger *slog.Logger,
	cfg *config.Config,
) (*UnbondingPipeline, error) {

	db, err := db.New(context.TODO(), cfg.Db.DbName, cfg.Db.Address)

	if err != nil {
		return nil, err
	}

	store := NewPersistentUnbondingStorage(db)

	client, err := btcclient.NewBtcClient(&cfg.Btc)

	if err != nil {
		return nil, err
	}

	parsedRemoteSignerCfg, err := cfg.Signer.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote signer config: %w", err)
	}

	signer, err := NewRemoteSigner(parsedRemoteSignerCfg)

	if err != nil {
		return nil, err
	}

	// TODO: Add parse func to other configs, and do parsing in one place
	parsedParams, err := cfg.Params.Parse()

	if err != nil {
		return nil, fmt.Errorf("failed to parse params: %w", err)
	}

	paramsRetriever := NewSystemParamsRetriever(
		parsedParams.CovenantQuorum,
		parsedParams.CovenantPublicKeys,
		parsedParams.MagicBytes,
	)

	return NewUnbondingPipeline(
		logger,
		store,
		signer,
		client,
		paramsRetriever,
		cfg.Btc.MustGetBtcNetworkParams(),
	), nil
}

func NewUnbondingPipeline(
	logger *slog.Logger,
	store UnbondingStore,
	signer CovenantSigner,
	sender BtcSender,
	retriever ParamsRetriever,
	btcParams *chaincfg.Params,
) *UnbondingPipeline {
	return &UnbondingPipeline{
		logger:    logger,
		store:     store,
		signer:    signer,
		sender:    sender,
		retriever: retriever,
		btcParams: btcParams,
	}
}

func (up *UnbondingPipeline) signUnbondingTransaction(
	unbondingTransaction *wire.MsgTx,
	fundingOutput *wire.TxOut,
	unbondingScript []byte,
	params *SystemParams,
) ([]*PubKeySigPair, error) {
	var requests []*SignRequest

	// TODO: Naive serialized way. In practice request shoud be done:
	// - in parallel
	// - only params.CovenantQuorum/len(params.CovenantPublicKeys) is required to succeed
	for _, pk := range params.CovenantPublicKeys {
		requests = append(requests, NewSignRequest(
			unbondingTransaction,
			fundingOutput,
			unbondingScript,
			pk,
		))
	}

	var signatures []*PubKeySigPair

	for _, req := range requests {
		sig, err := up.signer.SignUnbondingTransaction(req)

		if err != nil {
			return nil, fmt.Errorf("failed to sign unbonding tx: %w", err)
		}

		signatures = append(signatures, sig)
	}

	return signatures, nil
}

func (up *UnbondingPipeline) Store() UnbondingStore {
	return up.store
}

func outputsAreEqual(a, b *wire.TxOut) bool {
	if a.Value != b.Value {
		return false
	}

	if !bytes.Equal(a.PkScript, b.PkScript) {
		return false
	}

	return true
}

// Main Pipeline function which:
// 1. Retrieves unbonding transactions from store in order they were added
// 2. Sends them to covenant member for signing
// 3. Creates witness for unbonding transaction
// 4. Sends transaction to bitcoin network
// 5. Marks transaction as processed sending succeded or failed if sending failed
func (up *UnbondingPipeline) Run(ctx context.Context) error {
	up.logger.Info("Running unbonding pipeline")

	params, err := up.retriever.GetParams()

	if err != nil {
		return err
	}

	unbondingTransactions, err := up.store.GetNotProcessedUnbondingTransactions(ctx)

	if err != nil {
		return err
	}

	for _, tx := range unbondingTransactions {
		utx := tx

		stakingOutputRecovered, unbondingPathSpendInfo, err := CreateUnbondingPathSpendInfo(
			utx.StakingInfo,
			params,
			up.btcParams,
		)

		if err != nil {
			return wrapCrititical(err)
		}

		stakingOutputFromDb := utx.StakingOutput()

		// This the last line check before sending unbonding transaction for signing. It checks
		// whether staking output built from all the parameters: stakerPk, finalityProviderPk, stakingTimelock,
		// covenantPublicKeys, covenantQuorum, stakingAmount is equal to the one stored in db.
		// Potential reasons why it could fail:
		// - parameters changed (covenantQuorurm or convenanPks)
		// - pipeline is run on bad BTC network
		// - stakingApi service has a bug
		if !outputsAreEqual(stakingOutputRecovered, stakingOutputFromDb) {
			return wrapCrititical(fmt.Errorf("staking output from staking tx and staking output re-build from params are different"))
		}

		sigs, err := up.signUnbondingTransaction(
			utx.UnbondingTransaction,
			stakingOutputRecovered,
			stakingOutputRecovered.PkScript,
			params,
		)

		if err != nil {
			return wrapCrititical(err)
		}

		// TODO this functions re-creates staking output, maybe we should compare it with
		// staking output from db for double check
		witness, err := CreateUnbondingTxWitness(
			unbondingPathSpendInfo,
			params,
			utx.UnbondingTransactionSig,
			sigs,
			up.btcParams,
		)

		if err != nil {
			return wrapCrititical(err)
		}

		// We assume that this is valid unbodning transaction, with 1 input
		utx.UnbondingTransaction.TxIn[0].Witness = witness

		hash, err := up.sender.SendTx(utx.UnbondingTransaction)

		if err != nil {
			up.logger.Error("Failed to send unbonding transaction: %v", err)
			if err := up.store.SetUnbondingTransactionProcessingFailed(ctx, utx); err != nil {
				return wrapCrititical(err)
			}
		} else {
			up.logger.Info(
				"Succesfully sent unbonding transaction",
				slog.String("tx_hash", hash.String()),
			)
			if err := up.store.SetUnbondingTransactionProcessed(ctx, utx); err != nil {
				return wrapCrititical(err)
			}
		}
	}

	up.logger.Info("Unbonding pipeline run finished.", "num_tx_processed", len(unbondingTransactions))
	return nil
}
