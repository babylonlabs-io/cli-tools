package services

import (
	"fmt"

	"github.com/babylonchain/cli-tools/internal/btcclient"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

var _ BtcSender = (*BtcClientSender)(nil)

type BtcClientSender struct {
	client *btcclient.BtcClient
}

func NewBtcClientSender(client *btcclient.BtcClient) *BtcClientSender {
	return &BtcClientSender{client: client}
}

func (b *BtcClientSender) TxByHash(txHash *chainhash.Hash, pkScript []byte) (*TxInfo, error) {
	conf, status, err := b.client.TxDetails(txHash, pkScript)

	if err != nil {
		return nil, err
	}

	if status != btcclient.TxInChain {
		return nil, fmt.Errorf("tx %s not found in chain", txHash.String())
	}

	return &TxInfo{
		Tx:                conf.Tx,
		TxInclusionHeight: conf.BlockHeight,
	}, nil
}

func (b *BtcClientSender) SendTx(tx *wire.MsgTx) (*chainhash.Hash, error) {
	return b.client.SendTx(tx)
}

func (b *BtcClientSender) CheckTxOutSpendable(txHash *chainhash.Hash, index uint32, mempool bool) (bool, error) {
	return b.client.CheckTxOutSpendable(txHash, index, mempool)
}
