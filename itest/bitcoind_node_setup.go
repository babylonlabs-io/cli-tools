package e2etest

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/babylonchain/cli-tools/itest/containers"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
)

var (
	startTimeout = 30 * time.Second
)

type CreateWalletResponse struct {
	Name    string `json:"name"`
	Warning string `json:"warning"`
}

type GenerateBlockResponse struct {
	// address of the recipient of rewards
	Address string `json:"address"`
	// blocks generated
	Blocks []string `json:"blocks"`
}

type BitcoindTestHandler struct {
	t *testing.T
	m *containers.Manager
}

func NewBitcoindHandler(t *testing.T, m *containers.Manager) *BitcoindTestHandler {
	return &BitcoindTestHandler{
		t: t,
		m: m,
	}
}

func (h *BitcoindTestHandler) Start() {
	tempPath, err := os.MkdirTemp("", "bitcoind-staker-test-*")
	require.NoError(h.t, err)

	h.t.Cleanup(func() {
		_ = os.RemoveAll(tempPath)
	})

	_, err = h.m.RunBitcoindResource(tempPath)
	require.NoError(h.t, err)

	require.Eventually(h.t, func() bool {
		_, err := h.GetBlockCount()
		h.t.Logf("failed to get block count: %v", err)
		return err == nil
	}, startTimeout, 500*time.Millisecond, "bitcoind did not start")
}

func (h *BitcoindTestHandler) GetBlockCount() (int, error) {
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, []string{"getblockcount"})
	if err != nil {
		return 0, err
	}

	buffStr := buff.String()

	parsedBuffStr := strings.TrimSuffix(buffStr, "\n")

	num, err := strconv.Atoi(parsedBuffStr)
	if err != nil {
		return 0, err
	}

	return num, nil
}

func (h *BitcoindTestHandler) CreateWallet(walletName string, passphrase string) *CreateWalletResponse {
	// last false on the list will create legacy wallet. This is needed, as currently
	// we are signing all taproot transactions by dumping the private key and signing it
	// on app level. Descriptor wallets do not allow dumping private keys.
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, []string{"createwallet", walletName, "false", "false", passphrase})
	require.NoError(h.t, err)

	var response CreateWalletResponse
	err = json.Unmarshal(buff.Bytes(), &response)
	require.NoError(h.t, err)

	return &response
}

func (h *BitcoindTestHandler) GenerateBlocks(count int) *GenerateBlockResponse {
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, []string{fmt.Sprintf("-rpcwallet=%s", "test-wallet"), "-generate", fmt.Sprintf("%d", count)})
	require.NoError(h.t, err)

	var response GenerateBlockResponse
	err = json.Unmarshal(buff.Bytes(), &response)
	require.NoError(h.t, err)

	return &response
}

func (h *BitcoindTestHandler) GetNewAddress(rpcWallet string) btcutil.Address {
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, []string{fmt.Sprintf("-rpcwallet=%s", rpcWallet), "getnewaddress"})
	require.NoError(h.t, err)

	trimAddr := strings.TrimSpace(buff.String())

	newAddr, err := btcutil.DecodeAddress(trimAddr, &chaincfg.RegressionNetParams)
	require.NoError(h.t, err)

	return newAddr
}

func (h *BitcoindTestHandler) ListUnspent(rpcWallet string) string {
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, []string{fmt.Sprintf("-rpcwallet=%s", rpcWallet), "listunspent"})
	require.NoError(h.t, err)

	unspentTrim := strings.TrimSpace(buff.String())
	return unspentTrim
}

func (h *BitcoindTestHandler) GetAddressInfo(rpcWallet, address string) btcjson.GetAddressInfoResult {
	cmd := []string{fmt.Sprintf("-rpcwallet=%s", rpcWallet), "getaddressinfo", address}
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, cmd)
	require.NoError(h.t, err)

	var result btcjson.GetAddressInfoResult
	err = json.Unmarshal(buff.Bytes(), &result)
	require.NoError(h.t, err)

	return result
}

func (h *BitcoindTestHandler) GetTransaction(txID string) btcjson.GetTransactionResult {
	cmd := []string{"gettransaction", txID}
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, cmd)
	require.NoError(h.t, err)

	var result btcjson.GetTransactionResult
	err = json.Unmarshal(buff.Bytes(), &result)
	require.NoError(h.t, err)

	return result
}

func (h *BitcoindTestHandler) FundRawTx(rpcWallet, rawTxHex string) btcjson.FundRawTransactionResult {
	opt := btcjson.FundRawTransactionOpts{
		FeeRate: btcjson.Float64(0.005),
	}
	optBz, err := json.Marshal(opt)
	require.NoError(h.t, err)

	cmd := []string{fmt.Sprintf("-rpcwallet=%s", rpcWallet), "fundrawtransaction", rawTxHex, string(optBz)}
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, cmd)
	require.NoError(h.t, err)

	var result btcjson.FundRawTransactionResult
	err = result.UnmarshalJSON(buff.Bytes())
	require.NoError(h.t, err)

	return result
}

func (h *BitcoindTestHandler) SignRawTxWithWallet(rpcWallet, fundedRawTxHex string) btcjson.SignRawTransactionWithWalletResult {
	cmd := []string{fmt.Sprintf("-rpcwallet=%s", rpcWallet), "signrawtransactionwithwallet", fundedRawTxHex}
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, cmd)
	require.NoError(h.t, err)

	var result btcjson.SignRawTransactionWithWalletResult
	err = json.Unmarshal(buff.Bytes(), &result)
	require.NoError(h.t, err)

	return result
}

func (h *BitcoindTestHandler) WalletPassphrase(rpcWallet, passphrase, timeoutSec string) string {
	cmd := []string{fmt.Sprintf("-rpcwallet=%s", rpcWallet), "walletpassphrase", passphrase, timeoutSec}
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, cmd)
	require.NoError(h.t, err)
	return buff.String()
}

func (h *BitcoindTestHandler) SendRawTx(rpcWallet, fundedSignedTxHex string) string {
	cmd := []string{fmt.Sprintf("-rpcwallet=%s", rpcWallet), "sendrawtransaction", fundedSignedTxHex}
	buff, _, err := h.m.ExecBitcoindCliCmd(h.t, cmd)
	require.NoError(h.t, err)
	return buff.String()
}

func (h *BitcoindTestHandler) SendToAddress(rpcWallet, address, amount string) *chainhash.Hash {
	buff, errBuf, err := h.m.ExecBitcoindCliCmd(h.t, []string{
		fmt.Sprintf("-rpcwallet=%s", rpcWallet), "-named",
		"sendtoaddress",
		fmt.Sprintf("address=%s", address),
		fmt.Sprintf("amount=%s", amount),
		"fee_rate=5",
	})
	require.NoError(h.t, err, "errBuf", errBuf)

	trimTxHash := strings.TrimSpace(buff.String())
	txHash, err := chainhash.NewHashFromStr(trimTxHash)
	require.NoError(h.t, err)

	return txHash
}

func (h *BitcoindTestHandler) LoadWallet(walletName string) {
	_, _, err := h.m.ExecBitcoindCliCmd(h.t, []string{"loadwallet", walletName})
	require.NoError(h.t, err)
}
