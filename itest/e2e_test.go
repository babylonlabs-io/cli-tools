//go:build e2e
// +build e2e

package e2etest

import (
	"testing"
	"time"

	staking "github.com/babylonchain/babylon/btcstaking"

	"github.com/babylonchain/cli-tools/internal/btcclient"
	"github.com/babylonchain/cli-tools/internal/config"
	"github.com/babylonchain/cli-tools/internal/logger"
	"github.com/babylonchain/cli-tools/internal/services"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

var (
	netParams              = &chaincfg.RegressionNetParams
	eventuallyPollInterval = 100 * time.Millisecond
	eventuallyTimeout      = 10 * time.Second
)

type TestManager struct {
	t                   *testing.T
	bitcoindHandler     *BitcoindTestHandler
	walletPass          string
	btcClient           *btcclient.BtcClient
	covenantKeys        []*btcec.PrivateKey
	covenantQuorum      uint32
	finalityProviderKey *btcec.PrivateKey
	stakerAddress       btcutil.Address
	stakerPrivKey       *btcec.PrivateKey
	stakerPubKey        *btcec.PublicKey
	magicBytes          []byte
	pipeLine            *services.UnbondingPipeline
	store               *services.InMemoryUnbondingStore
}

type stakingData struct {
	stakingAmount  btcutil.Amount
	stakingTime    uint16
	stakingFeeRate btcutil.Amount
	unbondingTime  uint16
	unbondingFee   btcutil.Amount
}

func defaultStakingData() *stakingData {
	return &stakingData{
		stakingAmount:  btcutil.Amount(100000),
		stakingTime:    10000,
		stakingFeeRate: btcutil.Amount(5000), //feeRatePerKb
		unbondingTime:  100,
		unbondingFee:   btcutil.Amount(10000),
	}
}

func (d *stakingData) unbondingAmount() btcutil.Amount {
	return d.stakingAmount - d.unbondingFee
}

func StartManager(
	t *testing.T,
	numMatureOutputsInWallet uint32) *TestManager {
	h := NewBitcoindHandler(t)
	h.Start()
	passphrase := "pass"
	_ = h.CreateWallet("test-wallet", passphrase)
	// only outputs which are 100 deep are mature
	_ = h.GenerateBlocks(int(numMatureOutputsInWallet) + 100)

	btcConfig := config.DefaultBtcConfig()
	btcConfig.Host = "127.0.0.1:18443"
	btcConfig.User = "user"
	btcConfig.Pass = "pass"
	btcConfig.Network = netParams.Name

	client, err := btcclient.NewBtcClient(btcConfig)
	require.NoError(t, err)

	outputs, err := client.ListOutputs(true)
	require.NoError(t, err)
	require.Len(t, outputs, int(numMatureOutputsInWallet))

	// easiest way to get address controlled by wallet is to retrive address from one
	// of the outputs
	output := outputs[0]
	walletAddress, err := btcutil.DecodeAddress(output.Address, netParams)
	require.NoError(t, err)

	err = client.UnlockWallet(20, passphrase)
	require.NoError(t, err)
	stakerPrivKey, err := client.DumpPrivateKey(walletAddress)
	require.NoError(t, err)

	numCovenantKeys := 3
	quorum := uint32(2)
	var coventantKeys []*btcec.PrivateKey
	for i := 0; i < numCovenantKeys; i++ {
		key, err := btcec.NewPrivateKey()
		require.NoError(t, err)
		coventantKeys = append(coventantKeys, key)
	}

	fpKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	// TODO: Use mongo
	store := services.NewInMemoryUnbondingStore()

	// TODO: Use web service ?
	paramsRetriever := services.NewStaticParamsRetriever(quorum, coventantKeys)

	// TODO: Use more prod ready signer ?
	signer, err := services.NewStaticSigner(coventantKeys)
	require.NoError(t, err)

	logger := logger.DefaultLogger()
	pipeLine := services.NewUnbondingPipeline(
		logger,
		store,
		signer,
		client,
		paramsRetriever,
		netParams,
	)

	return &TestManager{
		t:                   t,
		bitcoindHandler:     h,
		walletPass:          passphrase,
		btcClient:           client,
		covenantKeys:        coventantKeys,
		covenantQuorum:      quorum,
		finalityProviderKey: fpKey,
		stakerAddress:       walletAddress,
		stakerPrivKey:       stakerPrivKey,
		stakerPubKey:        stakerPrivKey.PubKey(),
		magicBytes:          []byte{0x0, 0x1, 0x2, 0x3},
		pipeLine:            pipeLine,
		store:               store,
	}
}

func (tm *TestManager) covenantPubKeys() []*btcec.PublicKey {
	var pubKeys []*btcec.PublicKey
	for _, key := range tm.covenantKeys {
		k := key
		pubKeys = append(pubKeys, k.PubKey())
	}
	return pubKeys
}

type stakingTxSigInfo struct {
	stakingTxHash *chainhash.Hash
	stakingOutput *wire.TxOut
}

func (tm *TestManager) sendStakingTxToBtc(d *stakingData) *stakingTxSigInfo {
	info, err := staking.BuildV0IdentifiableStakingOutputs(
		tm.magicBytes,
		tm.stakerPubKey,
		tm.finalityProviderKey.PubKey(),
		tm.covenantPubKeys(),
		tm.covenantQuorum,
		d.stakingTime,
		d.stakingAmount,
		netParams,
	)
	require.NoError(tm.t, err)

	err = tm.btcClient.UnlockWallet(20, tm.walletPass)
	require.NoError(tm.t, err)
	// staking output will always have index 0
	tx, err := tm.btcClient.CreateAndSignTx(
		[]*wire.TxOut{info.StakingOutput, info.OpReturnOutput},
		d.stakingFeeRate,
		tm.stakerAddress,
	)
	require.NoError(tm.t, err)

	hash, err := tm.btcClient.SendTx(tx)
	require.NoError(tm.t, err)
	// generate blocks to make sure tx will be included into chain
	_ = tm.bitcoindHandler.GenerateBlocks(2)
	return &stakingTxSigInfo{
		stakingTxHash: hash,
		stakingOutput: info.StakingOutput,
	}
}

type unbondingTxWithStakerSig struct {
	unbondingTx *wire.MsgTx
	signature   *schnorr.Signature
}

func (tm *TestManager) createUnbondingTxAndSignByStaker(
	si *stakingTxSigInfo,
	d *stakingData,
) *unbondingTxWithStakerSig {

	info, err := staking.BuildV0IdentifiableStakingOutputs(
		tm.magicBytes,
		tm.stakerPubKey,
		tm.finalityProviderKey.PubKey(),
		tm.covenantPubKeys(),
		tm.covenantQuorum,
		d.stakingTime,
		d.stakingAmount,
		netParams,
	)
	require.NoError(tm.t, err)

	require.NoError(tm.t, err)

	unbondingPathInfo, err := info.UnbondingPathSpendInfo()
	require.NoError(tm.t, err)

	unbondingInfo, err := staking.BuildUnbondingInfo(
		tm.stakerPubKey,
		[]*btcec.PublicKey{tm.finalityProviderKey.PubKey()},
		tm.covenantPubKeys(),
		tm.covenantQuorum,
		d.unbondingTime,
		d.unbondingAmount(),
		netParams,
	)
	require.NoError(tm.t, err)

	unbondingTx := wire.NewMsgTx(2)
	unbondingTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(si.stakingTxHash, 0), nil, nil))
	unbondingTx.AddTxOut(unbondingInfo.UnbondingOutput)

	unbondingTxSignature, err := staking.SignTxWithOneScriptSpendInputFromScript(
		unbondingTx,
		si.stakingOutput,
		tm.stakerPrivKey,
		unbondingPathInfo.RevealedLeaf.Script,
	)
	require.NoError(tm.t, err)

	return &unbondingTxWithStakerSig{
		unbondingTx: unbondingTx,
		signature:   unbondingTxSignature,
	}
}

func (tm *TestManager) createStakingInfo(d *stakingData) *services.StakingInfo {
	return &services.StakingInfo{
		StakerPk:           tm.stakerPubKey,
		FinalityProviderPk: tm.finalityProviderKey.PubKey(),
		StakingTime:        d.stakingTime,
		StakingAmount:      d.stakingAmount,
	}
}

func (tm *TestManager) createNUnbondingTransactions(n int, d *stakingData) []*unbondingTxWithStakerSig {
	var infos []*stakingTxSigInfo
	for i := 0; i < n; i++ {
		sInfo := tm.sendStakingTxToBtc(d)
		_, status, err := tm.btcClient.TxDetails(sInfo.stakingTxHash, sInfo.stakingOutput.PkScript)
		require.NoError(tm.t, err)
		require.Equal(tm.t, btcclient.TxInChain, status)
		infos = append(infos, sInfo)
	}

	var unbondingTxs []*unbondingTxWithStakerSig
	for _, i := range infos {
		info := i
		ubs := tm.createUnbondingTxAndSignByStaker(
			info,
			d,
		)
		unbondingTxs = append(unbondingTxs, ubs)
	}

	return unbondingTxs
}

func TestRunningPipeline(t *testing.T) {
	m := StartManager(t, 10)
	d := defaultStakingData()
	numUnbondingTxs := 10

	// 1. Generate all unbonding transactions
	ubts := m.createNUnbondingTransactions(numUnbondingTxs, d)

	// 2. Add all unbonding transactions to store
	for _, u := range ubts {
		ubs := u
		err := m.store.AddTxWithSignature(
			ubs.unbondingTx,
			ubs.signature,
			m.createStakingInfo(d),
		)
		require.NoError(t, err)
	}

	// 3. Check store is not empty
	txRequireProcessingBefore, err := m.store.GetNotProcessedUnbondingTransactions()
	require.NoError(t, err)
	require.Len(t, txRequireProcessingBefore, numUnbondingTxs)

	// 4. Run pipeline
	err = m.pipeLine.Run()
	require.NoError(t, err)

	// 5. Generate few block to make sure transactions are included in btc
	_ = m.bitcoindHandler.GenerateBlocks(5)

	// 6. Check all included in btc chain
	for _, u := range ubts {
		ubs := u
		unbondingTxHash := ubs.unbondingTx.TxHash()
		_, status, err := m.btcClient.TxDetails(&unbondingTxHash, ubs.unbondingTx.TxOut[0].PkScript)
		require.NoError(t, err)
		require.Equal(t, btcclient.TxInChain, status)
	}

	// 7. Check there is no more transactions to process
	txRequireProcessingAfter, err := m.store.GetNotProcessedUnbondingTransactions()
	require.NoError(t, err)
	require.Len(t, txRequireProcessingAfter, 0)
}
