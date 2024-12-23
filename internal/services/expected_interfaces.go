package services

import (
	"context"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Minimal set of data necessary to sign unbonding transaction
type SignRequest struct {
	// Unbonding transaction which should be signed
	UnbondingTransaction *wire.MsgTx
	// Staker signature of the unbonding transaction
	StakerUnbondingSig *schnorr.Signature
	// Staking output which was used to fund unbonding transaction
	FundingOutput *wire.TxOut
	// Public key of the signer
	SignerPubKey *btcec.PublicKey
}

type SignResult struct {
	PubKeySig *PubKeySigPair
	Err       error
}

func NewSignRequest(
	tx *wire.MsgTx,
	stakerSig *schnorr.Signature,
	fundingOutput *wire.TxOut,
	pubKey *btcec.PublicKey,
) *SignRequest {
	return &SignRequest{
		UnbondingTransaction: tx,
		StakerUnbondingSig:   stakerSig,
		FundingOutput:        fundingOutput,
		SignerPubKey:         pubKey,
	}
}

type PubKeySigPair struct {
	Signature *schnorr.Signature
	PubKey    *btcec.PublicKey
}

type CovenantSigner interface {
	// This interface assumes that covenant signer has access to params and all necessary data
	SignUnbondingTransaction(req *SignRequest) (*PubKeySigPair, error)
}

type TxInfo struct {
	Tx                *wire.MsgTx
	TxInclusionHeight uint32
}

type BtcSender interface {
	TxByHash(txHash *chainhash.Hash, pkScript []byte) (*TxInfo, error)
	SendTx(tx *wire.MsgTx) (*chainhash.Hash, error)
	CheckTxOutSpendable(txHash *chainhash.Hash, index uint32, mempool bool) (bool, error)
}

type SystemParams struct {
	CovenantPublicKeys []*btcec.PublicKey
	CovenantQuorum     uint32
	Tag                []byte
}

type ParamsRetriever interface {
	ParamsByHeight(ctx context.Context, height uint64) (*SystemParams, error)
}

type StakingInfo struct {
	StakerPk           *btcec.PublicKey
	FinalityProviderPk *btcec.PublicKey
	StakingTimelock    uint16
	StakingAmount      btcutil.Amount
}

type StakingTransactionData struct {
	StakingTransaction *wire.MsgTx
	StakingOutputIdx   uint64
}

type UnbondingTxData struct {
	UnbondingDocID           primitive.ObjectID
	UnbondingTransaction     *wire.MsgTx
	UnbondingTransactionHash *chainhash.Hash
	UnbondingTransactionSig  *schnorr.Signature
	StakingInfo              *StakingInfo
	StakingTransactionData   *StakingTransactionData
}

func (u UnbondingTxData) StakingOutput() *wire.TxOut {
	return u.StakingTransactionData.StakingTransaction.TxOut[u.StakingTransactionData.StakingOutputIdx]
}

func NewUnbondingTxData(
	id primitive.ObjectID,
	tx *wire.MsgTx,
	hash *chainhash.Hash,
	sig *schnorr.Signature,
	info *StakingInfo,
	sd *StakingTransactionData,
) *UnbondingTxData {
	return &UnbondingTxData{
		UnbondingDocID:           id,
		UnbondingTransaction:     tx,
		UnbondingTransactionHash: hash,
		UnbondingTransactionSig:  sig,
		StakingInfo:              info,
		StakingTransactionData:   sd,
	}
}

type UnbondingStore interface {
	// TODO: For now it returns all not processed unbonding transactions but should:
	// 1. either return iterator over view of results
	// 2. or have limit argument to retrieve only N records
	// Interface Contract: results should be returned in the order they were added to the store
	GetNotProcessedUnbondingTransactions(ctx context.Context) ([]*UnbondingTxData, error)

	GetFailedUnbondingTransactions(ctx context.Context) ([]*UnbondingTxData, error)

	GetUnbondingTransactionsWithNoQuorum(ctx context.Context) ([]*UnbondingTxData, error)

	SetUnbondingTransactionProcessed(ctx context.Context, utx *UnbondingTxData) error

	SetUnbondingTransactionProcessingFailed(ctx context.Context, utx *UnbondingTxData) error

	SetUnbondingTransactionInputAlreadySpent(ctx context.Context, utx *UnbondingTxData) error

	SetUnbondingTransactionFailedToGetCovenantSignatures(ctx context.Context, utx *UnbondingTxData) error
}
