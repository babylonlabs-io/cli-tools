package model

import "go.mongodb.org/mongo-driver/bson/primitive"

// TODO Double check with staking-api-service
const (
	UnbondingCollection   = "unbonding_queue"
	UnbondingInitialState = "INSERTED"
)

type UnbondingState string

const (
	Inserted UnbondingState = "INSERTED"
	Send     UnbondingState = "SEND"
	// TODO: This is not used now, but it will be necessary to cover the case when
	// we try to send unbonding transaction but someone already withdrew the staking
	// output
	InputAlreadySpent UnbondingState = "INPUT_ALREADY_SPENT"
	Failed            UnbondingState = "FAILED"
)

type UnbondingDocument struct {
	ID                 primitive.ObjectID `bson:"_id"`
	UnbondingTxHashHex string             `bson:"unbonding_tx_hash_hex"`
	UnbondingTxHex     string             `bson:"unbonding_tx_hex"`
	UnbondingTxSigHex  string             `bson:"unbonding_tx_sig_hex"`
	StakerPkHex        string             `bson:"staker_pk_hex"`
	FinalityPkHex      string             `bson:"finality_pk_hex"`
	StakingTimelock    uint64             `bson:"staking_timelock"`
	StakingAmount      uint64             `bson:"staking_amount"`
	// TODO: Staking pkscript is not necessary here as we can derive it from other
	// staking data + covenant_params. Although maybe it would be worth to have it
	// here to double check everything is ok
	State UnbondingState `bson:"state"`
}
