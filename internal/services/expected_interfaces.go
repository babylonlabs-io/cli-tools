package services

import (
	"context"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// Minimal set of data necessary to sign unbonding transaction
type SignRequest struct {
	// Unbonding transaction which should be signed
	UnbondingTransaction *wire.MsgTx
	// Staking output which was used to fund unbonding transaction
	FundingOutput *wire.TxOut
	// Script of the path which should be execute - unbonding path
	UnbondingScript []byte
	// Public key of the signer
	SignerPubKey *btcec.PublicKey
}

type SignResult struct {
	PubKeySig *PubKeySigPair
	Err       error
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

type PubKeySigPair struct {
	Signature *schnorr.Signature
	PubKey    *btcec.PublicKey
}

type CovenantSigner interface {
	// This interface assumes that covenant signer has access to params and all necessary data
	SignUnbondingTransaction(req *SignRequest) (*PubKeySigPair, error)
}

type BtcSender interface {
	SendTx(tx *wire.MsgTx) (*chainhash.Hash, error)
	CheckTxOutSpendable(txHash *chainhash.Hash, index uint32, mempool bool) (bool, error)
}

type SystemParams struct {
	CovenantPublicKeys []*btcec.PublicKey
	CovenantQuorum     uint32
	MagicBytes         []byte
}

type ParamsRetriever interface {
	GetParams() (*SystemParams, error)
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
	tx *wire.MsgTx,
	hash *chainhash.Hash,
	sig *schnorr.Signature,
	info *StakingInfo,
	sd *StakingTransactionData,
) *UnbondingTxData {
	return &UnbondingTxData{
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

	SetUnbondingTransactionProcessed(ctx context.Context, utx *UnbondingTxData) error

	SetUnbondingTransactionProcessingFailed(ctx context.Context, utx *UnbondingTxData) error

	SetUnbondingTransactionInputAlreadySpent(ctx context.Context, utx *UnbondingTxData) error
}
