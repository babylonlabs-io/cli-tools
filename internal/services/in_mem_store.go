package services

import (
	"fmt"
	"sort"
	"sync"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type state string

const (
	inserted          state = "inserted"
	send              state = "send"
	inputAlreadySpent state = "input_already_spent"
	failed            state = "failed"
)

type unbondingTxDataWithCounter struct {
	UnbondingTxData
	Counter int
	state   state
}

func newUnbondingTxDataWithCounter(
	tx *wire.MsgTx,
	hash *chainhash.Hash,
	sig *schnorr.Signature,
	info *StakingInfo,
	counter int,
) *unbondingTxDataWithCounter {
	return &unbondingTxDataWithCounter{
		UnbondingTxData: *NewUnbondingTxData(tx, hash, sig, info),
		Counter:         counter,
		state:           inserted,
	}
}

type InMemoryUnbondingStore struct {
	mu      sync.Mutex
	mapping map[chainhash.Hash]*unbondingTxDataWithCounter
}

func NewInMemoryUnbondingStore() *InMemoryUnbondingStore {
	return &InMemoryUnbondingStore{
		mapping: make(map[chainhash.Hash]*unbondingTxDataWithCounter),
	}
}

func (s *InMemoryUnbondingStore) AddTxWithSignature(tx *wire.MsgTx, sig *schnorr.Signature, info *StakingInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := tx.TxHash()

	_, exists := s.mapping[hash]

	if exists {
		return fmt.Errorf("tx with hash %s already exists", hash)
	}

	nextCounter := len(s.mapping) + 1

	s.mapping[hash] = newUnbondingTxDataWithCounter(tx, &hash, sig, info, nextCounter)

	return nil
}

func (s *InMemoryUnbondingStore) GetNotProcessedUnbondingTransactions() ([]*UnbondingTxData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var res []*unbondingTxDataWithCounter

	for _, tx := range s.mapping {
		txCopy := tx
		// get only not processed transactions
		if tx.state == inserted {
			res = append(res, txCopy)
		}
	}

	// sort by counter
	sort.SliceStable(res, func(i, j int) bool {
		return res[i].Counter < res[j].Counter
	})

	// convert to UnbondingTxData
	var resUnbondingTxData []*UnbondingTxData
	for _, tx := range res {
		txCopy := tx
		resUnbondingTxData = append(resUnbondingTxData, &txCopy.UnbondingTxData)
	}

	return resUnbondingTxData, nil
}

func (s *InMemoryUnbondingStore) SetUnbondingTransactionProcessed(utx *UnbondingTxData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, exists := s.mapping[*utx.UnbondingTransactionHash]

	if !exists {
		return fmt.Errorf("tx with hash %s does not exist", *utx.UnbondingTransactionHash)
	}

	tx.state = send

	return nil
}

func (s *InMemoryUnbondingStore) SetUnbondingTransactionProcessingFailed(utx *UnbondingTxData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, exists := s.mapping[*utx.UnbondingTransactionHash]

	if !exists {
		return fmt.Errorf("tx with hash %s does not exist", *utx.UnbondingTransactionHash)
	}

	tx.state = failed

	return nil
}
