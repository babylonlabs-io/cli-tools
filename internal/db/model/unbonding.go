package model

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
	StakerPkHex        string         `bson:"staker_pk_hex"`
	FinalityPkHex      string         `bson:"finality_pk_hex"`
	UnbondingTxSigHex  string         `bson:"unbonding_tx_sig_hex"`
	State              UnbondingState `bson:"state"`
	UnbondingTxHashHex string         `bson:"unbonding_tx_hash_hex"` // Unique Index
	UnbondingTxHex     string         `bson:"unbonding_tx_hex"`
	StakingTxHex       string         `bson:"staking_tx_hex"`
	StakingOutputIndex uint64         `bson:"staking_output_index"`
	StakingTimelock    uint64         `bson:"staking_timelock"`
	StakingAmount      uint64         `bson:"staking_amount"`
	StakingTxHashHex   string         `json:"staking_tx_hash_hex"`
}
