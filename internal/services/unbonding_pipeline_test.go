package services_test

import (
	"context"
	"testing"

	"github.com/babylonchain/babylon/btcstaking"
	"github.com/babylonchain/cli-tools/internal/config"
	"github.com/babylonchain/cli-tools/internal/logger"
	"github.com/babylonchain/cli-tools/internal/mocks"
	"github.com/babylonchain/cli-tools/internal/services"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	testParams = chaincfg.MainNetParams
	magicBytes = []byte{0x00, 0x01, 0x02, 0x03}
)

type MockedDependencies struct {
	bs *mocks.MockBtcSender
	cs *mocks.MockCovenantSigner
	st *mocks.MockUnbondingStore
	pr *mocks.MockParamsRetriever
}

func NewMockedDependencies(t *testing.T) *MockedDependencies {
	ctrl := gomock.NewController(t)
	return &MockedDependencies{
		bs: mocks.NewMockBtcSender(ctrl),
		cs: mocks.NewMockCovenantSigner(ctrl),
		st: mocks.NewMockUnbondingStore(ctrl),
		pr: mocks.NewMockParamsRetriever(ctrl),
	}
}

func NewUnbondingData(
	t *testing.T,
	covenantInfo *CovenantInfo,
	covenantQuorum uint32,
) (*services.UnbondingTxData, []*services.PubKeySigPair) {
	stakingTime := uint16(1000)
	stakingAmount := btcutil.Amount(100000)
	unbondingFee := btcutil.Amount(1000)
	unbondingTime := uint16(100)
	stakerKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	stakerPubKey := stakerKey.PubKey()
	fpKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	stakingInfo, stakingTx, err := btcstaking.BuildV0IdentifiableStakingOutputsAndTx(
		magicBytes,
		stakerPubKey,
		fpKey.PubKey(),
		covenantInfo.GetCovenantPublicKeys(),
		covenantQuorum,
		stakingTime,
		stakingAmount,
		&testParams,
	)
	require.NoError(t, err)

	stakingUnbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)

	fakeInputHashBytes := [32]byte{}
	fakeInputHash, err := chainhash.NewHash(fakeInputHashBytes[:])
	require.NoError(t, err)
	fakeInputIndex := uint32(0)
	stakingTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(fakeInputHash, fakeInputIndex), nil, nil))

	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		stakerPubKey,
		[]*btcec.PublicKey{fpKey.PubKey()},
		covenantInfo.GetCovenantPublicKeys(),
		covenantQuorum,
		unbondingTime,
		stakingAmount-unbondingFee,
		&testParams,
	)
	require.NoError(t, err)
	stakingTxHash := stakingTx.TxHash()
	unbondingTx := wire.NewMsgTx(wire.TxVersion)
	unbondingTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&stakingTxHash, 0), nil, nil))
	unbondingTx.AddTxOut(unbondingInfo.UnbondingOutput)

	unbondingTxHash := unbondingTx.TxHash()

	validStakerSig, err := btcstaking.SignTxWithOneScriptSpendInputFromTapLeaf(
		unbondingTx,
		stakingInfo.StakingOutput,
		stakerKey,
		stakingUnbondingPathInfo.RevealedLeaf,
	)

	require.NoError(t, err)

	cov1sig, err := btcstaking.SignTxWithOneScriptSpendInputFromTapLeaf(
		unbondingTx,
		stakingInfo.StakingOutput,
		covenantInfo.Cov1PrivKey,
		stakingUnbondingPathInfo.RevealedLeaf,
	)
	require.NoError(t, err)

	var sigs []*services.PubKeySigPair
	sigs = append(sigs, &services.PubKeySigPair{
		Signature: cov1sig,
		PubKey:    covenantInfo.Cov1PrivKey.PubKey(),
	})

	utxData := &services.UnbondingTxData{
		UnbondingTransaction:     unbondingTx,
		UnbondingTransactionHash: &unbondingTxHash,
		UnbondingTransactionSig:  validStakerSig,
		StakingInfo: &services.StakingInfo{
			StakerPk:           stakerPubKey,
			FinalityProviderPk: fpKey.PubKey(),
			StakingTimelock:    stakingTime,
			StakingAmount:      stakingAmount,
		},
		StakingTransactionData: &services.StakingTransactionData{
			StakingTransaction: stakingTx,
			StakingOutputIdx:   0,
		},
	}

	return utxData, sigs
}

type CovenantInfo struct {
	Cov1PrivKey *btcec.PrivateKey
}

func (c *CovenantInfo) GetCovenantPublicKeys() []*btcec.PublicKey {
	return []*btcec.PublicKey{c.Cov1PrivKey.PubKey()}
}

func NewCovenantInfo(t *testing.T) *CovenantInfo {
	cov1Key, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	return &CovenantInfo{
		Cov1PrivKey: cov1Key,
	}
}

func TestValidSigningFlow(t *testing.T) {
	log := logger.DefaultLogger()
	deps := NewMockedDependencies(t)

	covenantMembers := NewCovenantInfo(t)
	covenantQuorum := uint32(1)
	unbondingData, covenantSignatures := NewUnbondingData(t, covenantMembers, covenantQuorum)
	pipeLine := services.NewUnbondingPipeline(
		log,
		deps.st,
		deps.cs,
		deps.bs,
		deps.pr,
		services.NewPipelineMetrics(config.DefaultMetricsConfig()),
		&testParams,
	)
	require.NotNil(t, pipeLine)

	stakingTransactionHash := unbondingData.StakingTransactionData.StakingTransaction.TxHash()
	deps.st.EXPECT().GetNotProcessedUnbondingTransactions(gomock.Any()).Return([]*services.UnbondingTxData{unbondingData}, nil)
	stakingTxHeight := uint32(100)
	deps.bs.EXPECT().TxByHash(
		&stakingTransactionHash,
		unbondingData.StakingOutput().PkScript,
	).Return(&services.TxInfo{
		Tx:                unbondingData.StakingTransactionData.StakingTransaction,
		TxInclusionHeight: stakingTxHeight,
	}, nil)
	deps.pr.EXPECT().ParamsByHeight(gomock.Any(), uint64(stakingTxHeight)).Return(&services.SystemParams{
		CovenantPublicKeys: covenantMembers.GetCovenantPublicKeys(),
		CovenantQuorum:     covenantQuorum,
		MagicBytes:         magicBytes,
	}, nil)
	deps.cs.EXPECT().SignUnbondingTransaction(gomock.Any()).Return(covenantSignatures[0], nil)
	deps.bs.EXPECT().CheckTxOutSpendable(
		&stakingTransactionHash,
		uint32(unbondingData.StakingTransactionData.StakingOutputIdx),
		true,
	).Return(true, nil)
	deps.bs.EXPECT().SendTx(unbondingData.UnbondingTransaction).Return(unbondingData.UnbondingTransactionHash, nil)
	deps.st.EXPECT().SetUnbondingTransactionProcessed(gomock.Any(), unbondingData).Return(nil)

	err := pipeLine.ProcessNewTransactions(context.Background())
	require.NoError(t, err)
}

func TestInvalidSignatureHandling(t *testing.T) {
	log := logger.DefaultLogger()
	deps := NewMockedDependencies(t)

	covenantMembers := NewCovenantInfo(t)
	covenantQuorum := uint32(1)
	unbondingData, covenantSignatures := NewUnbondingData(t, covenantMembers, covenantQuorum)
	pipeLine := services.NewUnbondingPipeline(
		log,
		deps.st,
		deps.cs,
		deps.bs,
		deps.pr,
		services.NewPipelineMetrics(config.DefaultMetricsConfig()),
		&testParams,
	)
	require.NotNil(t, pipeLine)

	stakingTransactionHash := unbondingData.StakingTransactionData.StakingTransaction.TxHash()
	deps.st.EXPECT().GetNotProcessedUnbondingTransactions(gomock.Any()).Return([]*services.UnbondingTxData{unbondingData}, nil)
	stakingTxHeight := uint32(100)
	deps.bs.EXPECT().TxByHash(
		&stakingTransactionHash,
		unbondingData.StakingOutput().PkScript,
	).Return(&services.TxInfo{
		Tx:                unbondingData.StakingTransactionData.StakingTransaction,
		TxInclusionHeight: stakingTxHeight,
	}, nil)
	deps.pr.EXPECT().ParamsByHeight(gomock.Any(), uint64(stakingTxHeight)).Return(&services.SystemParams{
		CovenantPublicKeys: covenantMembers.GetCovenantPublicKeys(),
		CovenantQuorum:     covenantQuorum,
		MagicBytes:         magicBytes,
	}, nil)

	// tamper signature so it is invalid
	validSignatureKeyPair := covenantSignatures[0]

	signatureBytes := validSignatureKeyPair.Signature.Serialize()
	// modify lastByte
	signatureBytes[len(signatureBytes)-1] = signatureBytes[len(signatureBytes)-1] + 1
	invalidSig, err := schnorr.ParseSignature(signatureBytes)
	require.NoError(t, err)

	validSignatureKeyPair.Signature = invalidSig

	deps.cs.EXPECT().SignUnbondingTransaction(gomock.Any()).Return(validSignatureKeyPair, nil)
	// we receive invalid signature from the only signer, so we should fail to get quorum
	deps.st.EXPECT().SetUnbondingTransactionFailedToGetCovenantSignatures(gomock.Any(), unbondingData).Return(nil)

	err = pipeLine.ProcessNewTransactions(context.Background())
	require.NoError(t, err)
}
