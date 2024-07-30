package services

import (
	"bytes"
	"context"
	"encoding/hex"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"

	"github.com/babylonlabs-io/cli-tools/internal/db"
	"github.com/babylonlabs-io/cli-tools/internal/db/model"
)

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

func pubKeyFromHex(hexString string) (*btcec.PublicKey, error) {
	bytes, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}

	key, err := schnorr.ParsePubKey(bytes)

	if err != nil {
		return nil, err
	}

	return key, nil
}

var _ UnbondingStore = (*PersistentUnbondingStorage)(nil)

type PersistentUnbondingStorage struct {
	client *db.Database
}

func NewPersistentUnbondingStorage(
	client *db.Database,
) *PersistentUnbondingStorage {
	return &PersistentUnbondingStorage{
		client: client,
	}
}

func documentToData(d *model.UnbondingDocument) (*UnbondingTxData, error) {
	tx, _, err := newBTCTxFromHex(d.UnbondingTxHex)
	if err != nil {
		return nil, err
	}

	unbondingTxHash, err := chainhash.NewHashFromStr(d.UnbondingTxHashHex)

	if err != nil {
		return nil, err
	}

	sigBytes, err := hex.DecodeString(d.UnbondingTxSigHex)
	if err != nil {
		return nil, err
	}

	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		return nil, err
	}

	stakerPk, err := pubKeyFromHex(d.StakerPkHex)
	if err != nil {
		return nil, err
	}

	fpPk, err := pubKeyFromHex(d.FinalityPkHex)
	if err != nil {
		return nil, err
	}

	// TODO: Check if there are better types at mongo level
	stakingValue := btcutil.Amount(int64(d.StakingAmount))
	stakingTime := uint16(d.StakingTimelock)

	si := &StakingInfo{
		StakerPk:           stakerPk,
		FinalityProviderPk: fpPk,
		StakingTimelock:    stakingTime,
		StakingAmount:      stakingValue,
	}

	stakingTx, _, err := newBTCTxFromHex(d.StakingTxHex)

	if err != nil {
		return nil, err
	}

	sd := &StakingTransactionData{
		StakingTransaction: stakingTx,
		StakingOutputIdx:   d.StakingOutputIndex,
	}

	return NewUnbondingTxData(tx, unbondingTxHash, sig, si, sd), nil
}

func (s *PersistentUnbondingStorage) AddTxWithSignature(
	ctx context.Context,
	tx *wire.MsgTx,
	sig *schnorr.Signature,
	info *StakingInfo,
	stakingtTxData *StakingTransactionData,
) error {
	unbondingTxHex, err := serializeBTCTxToHex(tx)
	if err != nil {
		return err
	}

	unbondingTxHashHex := tx.TxHash().String()

	sigBytes := sig.Serialize()
	sigHex := hex.EncodeToString(sigBytes)

	stakerPkHex := pubKeyToStringSchnorr(info.StakerPk)
	fpPkHex := pubKeyToStringSchnorr(info.FinalityProviderPk)

	stakingTxHex, err := serializeBTCTxToHex(stakingtTxData.StakingTransaction)
	if err != nil {
		return err
	}

	stakingTxHashHex := stakingtTxData.StakingTransaction.TxHash().String()

	err = s.client.SaveUnbondingDocument(
		ctx,
		unbondingTxHashHex,
		unbondingTxHex,
		sigHex,
		stakerPkHex,
		fpPkHex,
		stakingTxHex,
		stakingtTxData.StakingOutputIdx,
		stakingTxHashHex,
		uint64(info.StakingTimelock),
		uint64(info.StakingAmount),
	)

	if err != nil {
		return err
	}
	return nil
}

func transformDocuments(
	ctx context.Context,
	getDocuments func(ctx context.Context) ([]model.UnbondingDocument, error)) ([]*UnbondingTxData, error) {
	docs, err := getDocuments(ctx)
	if err != nil {
		return nil, err
	}

	var res []*UnbondingTxData
	for _, doc := range docs {
		data, err := documentToData(&doc)
		if err != nil {
			return nil, err
		}
		res = append(res, data)
	}

	return res, nil
}

func (s *PersistentUnbondingStorage) GetSendUnbondingTransactions(ctx context.Context) ([]*UnbondingTxData, error) {
	return transformDocuments(
		ctx,
		s.client.FindSendUnbondingDocuments,
	)
}

func (s *PersistentUnbondingStorage) GetFailedUnbondingTransactions(ctx context.Context) ([]*UnbondingTxData, error) {
	return transformDocuments(
		ctx,
		s.client.FindFailedUnbodningDocuments,
	)
}

func (s *PersistentUnbondingStorage) GetUnbondingTransactionsWithNoQuorum(ctx context.Context) ([]*UnbondingTxData, error) {
	return transformDocuments(
		ctx,
		s.client.FindUnbondingDocumentsWithNoCovenantQuorum,
	)
}

func (s *PersistentUnbondingStorage) GetNotProcessedUnbondingTransactions(ctx context.Context) ([]*UnbondingTxData, error) {
	return transformDocuments(
		ctx,
		s.client.FindNewUnbondingDocuments,
	)
}

func (s *PersistentUnbondingStorage) SetUnbondingTransactionProcessed(ctx context.Context, utx *UnbondingTxData) error {
	txHash := utx.UnbondingTransactionHash.String()
	return s.client.SetUnbondingDocumentSend(ctx, txHash)
}

func (s *PersistentUnbondingStorage) SetUnbondingTransactionProcessingFailed(ctx context.Context, utx *UnbondingTxData) error {
	txHash := utx.UnbondingTransactionHash.String()
	return s.client.SetUnbondingDocumentFailed(ctx, txHash)
}

func (s *PersistentUnbondingStorage) SetUnbondingTransactionInputAlreadySpent(ctx context.Context, utx *UnbondingTxData) error {
	txHash := utx.UnbondingTransactionHash.String()
	return s.client.SetUnbondingDocumentFailed(ctx, txHash)
}

func (s *PersistentUnbondingStorage) SetUnbondingTransactionFailedToGetCovenantSignatures(ctx context.Context, utx *UnbondingTxData) error {
	txHash := utx.UnbondingTransactionHash.String()
	return s.client.SetUnbondingDocumentFailedToGetCovenantSignatures(ctx, txHash)
}
