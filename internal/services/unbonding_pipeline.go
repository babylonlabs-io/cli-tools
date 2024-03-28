package services

import (
	"encoding/hex"
	"fmt"
	"log/slog"

	staking "github.com/babylonchain/babylon/btcstaking"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
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

type SystemParams struct {
	CovenantQuorum     uint32
	CovenantPublicKeys []*btcec.PublicKey
}

// TODO This data is necessary to recreate staking script tree, and create proof
// of inclusion when sending unbonding transaction. Defeine how it should be passed
// from api to unbodning pipeline and covenant signer
type StakingInfo struct {
	StakerPk           *btcec.PublicKey
	FinalityProviderPk *btcec.PublicKey
	StakingTime        uint16
	StakingAmount      btcutil.Amount
}

type UnbondingTxData struct {
	UnbondingTransaction     *wire.MsgTx
	UnbondingTransactionHash *chainhash.Hash
	UnbondingTransactionSig  *schnorr.Signature
	// TODO: For now we assume that staking info is part of db
	StakingInfo *StakingInfo
}

func NewUnbondingTxData(
	tx *wire.MsgTx,
	hash *chainhash.Hash,
	sig *schnorr.Signature,
	info *StakingInfo,
) *UnbondingTxData {
	return &UnbondingTxData{
		UnbondingTransaction:     tx,
		UnbondingTransactionHash: hash,
		UnbondingTransactionSig:  sig,
		StakingInfo:              info,
	}
}

type PubKeySigPair struct {
	Signature *schnorr.Signature
	PubKey    *btcec.PublicKey
}

// Minimal set of data necessary to sign unbonding transaction
type SignRequest struct {
	// Unbonding transaction which should be signed
	UnbondingTransaction *wire.MsgTx
	// Staking output which was used to fund unbonding transaction
	FundingOutput *wire.TxOut
	//Script of the path which should be execute - unbonding path
	UnbondingScript []byte
	// Public key of the signer
	SignerPubKey *btcec.PublicKey
}

func NewSignRequest(
	tx *wire.MsgTx,
	fundingOutput *wire.TxOut,
	script []byte,
	pubKey *btcec.PublicKey,
) *SignRequest {
	return &SignRequest{
		UnbondingTransaction: tx,
		FundingOutput:        fundingOutput,
		UnbondingScript:      script,
		SignerPubKey:         pubKey,
	}
}

type CovenantSigner interface {
	// This interface assumes that covenant signer has access to params and all necessary data
	SignUnbondingTransaction(req *SignRequest) (*PubKeySigPair, error)
}

func pubKeyToString(pubKey *btcec.PublicKey) string {
	return hex.EncodeToString(schnorr.SerializePubKey(pubKey))
}

// signer with initialized in memory private keys
type StaticSigner struct {
	mapPubKey map[string]*btcec.PrivateKey
}

func NewStaticSigner(privateKeys []*btcec.PrivateKey) (*StaticSigner, error) {
	if len(privateKeys) == 0 {
		return nil, fmt.Errorf("no private keys provided for static signer")
	}

	mapPubKey := make(map[string]*btcec.PrivateKey)

	for _, priv := range privateKeys {
		k := priv

		pubKeyHex := pubKeyToString(k.PubKey())

		if _, found := mapPubKey[pubKeyHex]; found {
			return nil, fmt.Errorf("duplicate public key provided for static signer")
		}

		mapPubKey[pubKeyHex] = k
	}

	return &StaticSigner{
		mapPubKey: mapPubKey,
	}, nil
}

func (s *StaticSigner) SignUnbondingTransaction(req *SignRequest) (*PubKeySigPair, error) {
	pubKeyStr := pubKeyToString(req.SignerPubKey)

	privKey, found := s.mapPubKey[pubKeyStr]

	if !found {
		return nil, fmt.Errorf("private key not found for public key %s", pubKeyStr)
	}

	signature, err := staking.SignTxWithOneScriptSpendInputFromScript(
		req.UnbondingTransaction,
		req.FundingOutput,
		privKey,
		req.UnbondingScript,
	)

	if err != nil {
		return nil, err
	}

	return &PubKeySigPair{
		Signature: signature,
		PubKey:    req.SignerPubKey,
	}, nil
}

type UnbondingStore interface {
	// TODO: For now it returns all not processed unbonding transactions but should:
	// 1. either return iterator over view of results
	// 2. or have limit argument to retrieve only N records
	// Interface Contract: results should be returned in the order they were added to the store
	GetNotProcessedUnbondingTransactions() ([]*UnbondingTxData, error)

	SetUnbondingTransactionProcessed(utx *UnbondingTxData) error

	SetUnbondingTransactionProcessingFailed(utx *UnbondingTxData) error
}

type BtcSender interface {
	SendTx(tx *wire.MsgTx) (*chainhash.Hash, error)
}

type ParamsRetriever interface {
	GetParams() (*SystemParams, error)
}

type StaticParamsRetriever struct {
	CovenantQuorum      uint32
	CovenantPrivateKeys []*btcec.PrivateKey
}

func NewStaticParamsRetriever(
	quorum uint32,
	privateKeys []*btcec.PrivateKey,
) *StaticParamsRetriever {
	return &StaticParamsRetriever{
		CovenantQuorum:      quorum,
		CovenantPrivateKeys: privateKeys,
	}
}

func (p *StaticParamsRetriever) GetParams() (*SystemParams, error) {
	publicKeys := make([]*btcec.PublicKey, 0, len(p.CovenantPrivateKeys))

	for _, pk := range p.CovenantPrivateKeys {
		publicKeys = append(publicKeys, pk.PubKey())
	}

	return &SystemParams{
		CovenantQuorum:     p.CovenantQuorum,
		CovenantPublicKeys: publicKeys,
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

// Main Pipeline function which:
// 1. Retrieves unbonding transactions from store in order they were added
// 2. Sends them to covenant member for signing
// 3. Creates witness for unbonding transaction
// 4. Sends transaction to bitcoin network
// 5. Marks transaction as processed sending succeded or failed if sending failed
func (up *UnbondingPipeline) Run() error {
	up.logger.Info("Running unbonding pipeline")

	params, err := up.retriever.GetParams()

	if err != nil {
		return err
	}

	unbondingTransactions, err := up.store.GetNotProcessedUnbondingTransactions()

	if err != nil {
		return err
	}

	for _, tx := range unbondingTransactions {
		utx := tx

		stakingOutput, unbondingPathSpendInfo, err := CreateUnbondingPathSpendInfo(
			utx.StakingInfo,
			params,
			up.btcParams,
		)

		if err != nil {
			return wrapCrititical(err)
		}

		sigs, err := up.signUnbondingTransaction(
			utx.UnbondingTransaction,
			stakingOutput,
			unbondingPathSpendInfo.RevealedLeaf.Script,
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
			if err := up.store.SetUnbondingTransactionProcessingFailed(utx); err != nil {
				return wrapCrititical(err)
			}
		} else {
			up.logger.Info(
				"Succesfully sent unbonding transaction",
				slog.String("tx_hash", hash.String()),
			)
			if err := up.store.SetUnbondingTransactionProcessed(utx); err != nil {
				return wrapCrititical(err)
			}
		}
	}

	return nil
}
