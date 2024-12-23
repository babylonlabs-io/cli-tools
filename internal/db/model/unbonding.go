package model

import "go.mongodb.org/mongo-driver/bson/primitive"

// TODO Double check with staking-api-service
const (
	UnbondingCollection   = "unbonding_queue"
	UnbondingInitialState = "INSERTED"
)

type UnbondingState string

const (
	Inserted                      UnbondingState = "INSERTED"
	Send                          UnbondingState = "SEND"
	InputAlreadySpent             UnbondingState = "INPUT_ALREADY_SPENT"
	Failed                        UnbondingState = "FAILED"
	FailedToGetCovenantSignatures UnbondingState = "FAILED_TO_GET_COVENANT_SIGNATURES"
)

type UnbondingDocument struct {
	// Can be nil only if we are inserting a document
	ID                 *primitive.ObjectID `bson:"_id,omitempty"`
	StakerPkHex        string              `bson:"staker_pk_hex"`
	FinalityPkHex      string              `bson:"finality_pk_hex"`
	UnbondingTxSigHex  string              `bson:"unbonding_tx_sig_hex"`
	State              UnbondingState      `bson:"state"`
	UnbondingTxHashHex string              `bson:"unbonding_tx_hash_hex"` // Unique Index
	UnbondingTxHex     string              `bson:"unbonding_tx_hex"`
	StakingTxHex       string              `bson:"staking_tx_hex"`
	StakingOutputIndex uint64              `bson:"staking_output_index"`
	StakingTimelock    uint64              `bson:"staking_timelock"`
	StakingAmount      uint64              `bson:"staking_amount"`
	StakingTxHashHex   string              `json:"staking_tx_hash_hex"`
}
