package model

import "go.mongodb.org/mongo-driver/bson/primitive"

// TODO Double check with staking-api-service
const UnbondingCollection = "unbonding_transactions"

type UnbondingState string

const (
	Inserted          UnbondingState = "inserted"
	Send              UnbondingState = "send"
	InputAlreadySpent UnbondingState = "input_already_spent"
	Failed            UnbondingState = "failed"
)

type UnbondingDocument struct {
	ID                 primitive.ObjectID `bson:"_id"`
	UnbondingTxHashHex string             `bson:"unbonding_tx_hash_hex"`
	UnbondingTxHex     string             `bson:"unbonding_tx_hex"`
	UnbondingTxSigHex  string             `bson:"unbonding_tx_sig_hex"`
	State              UnbondingState     `bson:"state"`
}
