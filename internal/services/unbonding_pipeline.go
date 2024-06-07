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
	"github.com/prometheus/client_golang/prometheus/push"
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

func pubKeyToStringSchnorr(pubKey *btcec.PublicKey) string {
	return hex.EncodeToString(schnorr.SerializePubKey(pubKey))
}

func pubKeyToStringCompressed(pubKey *btcec.PublicKey) string {
	return hex.EncodeToString(pubKey.SerializeCompressed())
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
	Metrics   *PipelineMetrics
	btcParams *chaincfg.Params
}

func NewUnbondingPipelineFromConfig(
	logger *slog.Logger,
	cfg *config.Config,
	ret *VersionedParamsRetriever,
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

	bs := NewBtcClientSender(client)

	parsedRemoteSignerCfg, err := cfg.Signer.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote signer config: %w", err)
	}

	signer, err := NewRemoteSigner(parsedRemoteSignerCfg)

	if err != nil {
		return nil, err
	}

	m := NewPipelineMetrics(&cfg.Metrics)

	return NewUnbondingPipeline(
		logger,
		store,
		signer,
		bs,
		ret,
		m,
		cfg.Btc.MustGetBtcNetworkParams(),
	), nil
}

func NewUnbondingPipeline(
	logger *slog.Logger,
	store UnbondingStore,
	signer CovenantSigner,
	sender BtcSender,
	retriever ParamsRetriever,
	metrics *PipelineMetrics,
	btcParams *chaincfg.Params,
) *UnbondingPipeline {
	return &UnbondingPipeline{
		logger:    logger,
		store:     store,
		signer:    signer,
		sender:    sender,
		retriever: retriever,
		Metrics:   metrics,
		btcParams: btcParams,
	}
}

// signUnbondingTransaction requests signatures from all the
// covenant signers in a concurrent manner
func (up *UnbondingPipeline) signUnbondingTransaction(
	unbondingTransaction *wire.MsgTx,
	stakerSig *schnorr.Signature,
	fundingOutput *wire.TxOut,
	unbondingScript []byte,
	params *SystemParams,
) ([]*PubKeySigPair, error) {
	// send requests concurrently
	resultChan := make(chan *SignResult, len(params.CovenantPublicKeys))
	for _, pk := range params.CovenantPublicKeys {
		req := NewSignRequest(
			unbondingTransaction,
			stakerSig,
			fundingOutput,
			unbondingScript,
			pk,
		)
		go up.requestSigFromCovenant(req, resultChan)
	}

	// check all the results
	// Note that the latency of processing all the results depends on
	// the slowest response
	var signatures []*PubKeySigPair
	for i := 0; i < len(params.CovenantPublicKeys); i++ {
		res := <-resultChan
		if res.Err != nil {
			continue
		}
		signatures = append(signatures, res.PubKeySig)
	}

	if len(signatures) < int(params.CovenantQuorum) {
		return nil, fmt.Errorf("insufficient covenant signatures: expected %d, got: %d",
			params.CovenantQuorum, len(signatures))
	}

	// return a quorum is enough as the script is using OP_NUMEQUAL op code
	// ordered by the order of arrival
	return signatures[:params.CovenantQuorum], nil
}

func (up *UnbondingPipeline) requestSigFromCovenant(req *SignRequest, resultChan chan *SignResult) {
	pkStr := pubKeyToStringCompressed(req.SignerPubKey)
	up.logger.Debug("request signatures from covenant signer",
		"signer_pk", pkStr)

	var res SignResult
	sigPair, err := up.signer.SignUnbondingTransaction(req)
	if err != nil {
		up.Metrics.RecordFailedSigningRequest(pkStr)
		up.logger.Error("failed to get signatures from covenant",
			"signer_pk", pkStr,
			"error", err)

		res.Err = err
	} else {
		up.Metrics.RecordSuccessSigningRequest(pkStr)
		up.logger.Debug("got signatures from covenant signer", "signer_pk", pkStr)

		res.PubKeySig = sigPair
	}

	resultChan <- &res
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

func (up *UnbondingPipeline) pushMetrics() error {
	gatewayUrl, err := up.Metrics.Config.Address()
	if err != nil {
		return fmt.Errorf("failed to get gateway address: %w", err)
	}

	up.logger.Info("Pushing metrics to gateway", "gateway", gatewayUrl)

	return push.New(gatewayUrl, "unbonding-pipeline").
		Collector(up.Metrics.SuccessSigningReqs).
		Collector(up.Metrics.FailedSigningReqs).
		Collector(up.Metrics.SuccessfulSentTransactions).
		Collector(up.Metrics.FailureSentTransactions).
		Push()
}

func (up *UnbondingPipeline) processUnbondingTransactions(
	ctx context.Context,
	transactions []*UnbondingTxData,
) error {
	for _, tx := range transactions {
		utx := tx

		stakingOutputFromDb := utx.StakingOutput()
		stakingTxHash := utx.UnbondingTransaction.TxIn[0].PreviousOutPoint.Hash

		stakingTxInfo, err := up.sender.TxByHash(
			&stakingTxHash,
			stakingOutputFromDb.PkScript,
		)

		// if the staking transaction is not found in btc chain, it means something is wrong
		// as staking service should not allow to create unbonding transaction without staking transaction
		if err != nil {
			return wrapCrititical(err)
		}

		params, err := up.retriever.ParamsByHeight(ctx, uint64(stakingTxInfo.TxInclusionHeight))

		// we should always be able to retrieve params for the height of the staking transaction
		if err != nil {
			return wrapCrititical(err)
		}

		stakingOutputRecovered, unbondingPathSpendInfo, err := CreateUnbondingPathSpendInfo(
			utx.StakingInfo,
			params,
			up.btcParams,
		)

		if err != nil {
			return wrapCrititical(err)
		}

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
			utx.UnbondingTransactionSig,
			stakingOutputRecovered,
			stakingOutputRecovered.PkScript,
			params,
		)

		if err != nil {
			return wrapCrititical(err)
		}

		up.logger.Info("Successfully collected quorum of covenant signatures to unbond",
			"staking_tx_hash", tx.StakingTransactionData.StakingTransaction.TxHash().String(),
			"unbonding_tx_hash", tx.UnbondingTransactionHash.String())

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

		// TODO do we need to check the mempool?
		spendable, err := up.sender.CheckTxOutSpendable(&stakingTxHash, uint32(utx.StakingTransactionData.StakingOutputIdx), true)
		if err != nil {
			up.logger.Error("Failed to check whether the staking output is spendable", "error", err)
			// will still try to send the tx
		}
		if err == nil && !spendable {
			up.logger.Info("The input of the unbonding transaction has already been spent",
				slog.String("staking_tx_hash", stakingTxHash.String()))
			if err := up.store.SetUnbondingTransactionInputAlreadySpent(ctx, utx); err != nil {
				return wrapCrititical(err)
			}
			continue
		}

		hash, err := up.sender.SendTx(utx.UnbondingTransaction)

		if err != nil {
			up.logger.Error("Failed to send unbonding transaction", "error", err)
			if err := up.store.SetUnbondingTransactionProcessingFailed(ctx, utx); err != nil {
				return wrapCrititical(err)
			}
			up.Metrics.RecordFailedUnbodingTransaction()
		} else {
			up.logger.Info(
				"Successfully sent unbonding transaction",
				slog.String("tx_hash", hash.String()),
			)
			if err := up.store.SetUnbondingTransactionProcessed(ctx, utx); err != nil {
				return wrapCrititical(err)
			}
			up.Metrics.RecordSentUnbondingTransaction()
		}
	}

	return nil
}

// Main Pipeline function which:
// 1. Retrieves unbonding transactions from store in order they were added
// 2. Sends them to covenant member for signing
// 3. Creates witness for unbonding transaction
// 4. Sends transaction to bitcoin network
// 5. Marks transaction as processed sending succeded or failed if sending failed
func (up *UnbondingPipeline) ProcessNewTransactions(ctx context.Context) error {
	up.logger.Info("Running unbonding pipeline with new transactions")

	unbondingTransactions, err := up.store.GetNotProcessedUnbondingTransactions(ctx)

	if err != nil {
		return err
	}

	defer func() {
		if up.Metrics.Config.Enabled {
			if err := up.pushMetrics(); err != nil {
				up.logger.Error("Failed to push metrics", "error", err)
			}
		}
	}()

	if len(unbondingTransactions) == 0 {
		up.logger.Info("No unbonding new transactions to process")
		return nil
	}

	if err := up.processUnbondingTransactions(ctx, unbondingTransactions); err != nil {
		return err
	}

	up.logger.Info("Unbonding pipeline run for new transactions finished.", "num_tx_processed", len(unbondingTransactions))
	return nil
}

func (up *UnbondingPipeline) ProcessFailedTransactions(ctx context.Context) error {
	up.logger.Info("Running unbonding pipeline for failed transactions")

	unbondingTransactions, err := up.store.GetFailedUnbondingTransactions(ctx)

	if err != nil {
		return err
	}

	defer func() {
		if up.Metrics.Config.Enabled {
			if err := up.pushMetrics(); err != nil {
				up.logger.Error("Failed to push metrics", "error", err)
			}
		}
	}()

	if len(unbondingTransactions) == 0 {
		up.logger.Info("No failed unbonding transactions to process")
		return nil
	}

	if err := up.processUnbondingTransactions(ctx, unbondingTransactions); err != nil {
		return err
	}

	up.logger.Info("Unbonding pipeline for failed transactions finished.", "num_tx_processed", len(unbondingTransactions))
	return nil
}
