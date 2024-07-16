package services

import (
	"bytes"
	"fmt"
	"sort"

	staking "github.com/babylonchain/babylon/btcstaking"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
)

// Helper function to sort all signatures in reverse lexicographical order of signing public keys
// this way signatures are ready to be used in multisig witness with corresponding public keys
func sortPubKeysForWitness(infos []*btcec.PublicKey) []*btcec.PublicKey {
	sortedInfos := make([]*btcec.PublicKey, len(infos))
	copy(sortedInfos, infos)
	sort.SliceStable(sortedInfos, func(i, j int) bool {
		keyIBytes := schnorr.SerializePubKey(sortedInfos[i])
		keyJBytes := schnorr.SerializePubKey(sortedInfos[j])
		return bytes.Compare(keyIBytes, keyJBytes) == 1
	})

	return sortedInfos
}

func createWitnessSignaturesForPubKeys(
	covenantPubKeys []*btcec.PublicKey,
	receivedSignaturePairs []*PubKeySigPair,
) []*schnorr.Signature {
	// create map of received signatures
	receivedSignatures := make(map[string]*schnorr.Signature)

	for _, pair := range receivedSignaturePairs {
		receivedSignatures[pubKeyToStringSchnorr(pair.PubKey)] = pair.Signature
	}

	sortedPubKeys := sortPubKeysForWitness(covenantPubKeys)

	// this makes sure number of signatures is equal to number of public keys
	signatures := make([]*schnorr.Signature, len(sortedPubKeys))

	for i, key := range sortedPubKeys {
		k := key
		if signature, found := receivedSignatures[pubKeyToStringSchnorr(k)]; found {
			signatures[i] = signature
		}
	}

	return signatures
}

func CreateUnbondingPathSpendInfo(
	stakingInfo *StakingInfo,
	params *SystemParams,
	net *chaincfg.Params,
) (*wire.TxOut, *staking.SpendInfo, error) {
	info, err := staking.BuildV0IdentifiableStakingOutputs(
		params.Tag,
		stakingInfo.StakerPk,
		stakingInfo.FinalityProviderPk,
		params.CovenantPublicKeys,
		params.CovenantQuorum,
		stakingInfo.StakingTimelock,
		stakingInfo.StakingAmount,
		net,
	)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to build staking info: %w", err)
	}

	unbondingPathInfo, err := info.UnbondingPathSpendInfo()

	if err != nil {
		return nil, nil, fmt.Errorf("failed to build unbonding spend info: %w", err)
	}

	return info.StakingOutput, unbondingPathInfo, nil
}

// returns:
// - staking output which was used to fund unbonding transaction
// - witness for unbonding transaction
func CreateUnbondingTxWitness(
	unbondingPathInfo *staking.SpendInfo,
	params *SystemParams,
	stakerUnbondingSig *schnorr.Signature,
	covenantSignatures []*PubKeySigPair,
	net *chaincfg.Params,
) (wire.TxWitness, error) {

	covenantSigantures := createWitnessSignaturesForPubKeys(
		params.CovenantPublicKeys,
		covenantSignatures,
	)

	witness, err := unbondingPathInfo.CreateUnbondingPathWitness(
		covenantSigantures,
		stakerUnbondingSig,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to build unbonding tx wittness: %w", err)
	}

	return witness, nil
}
