# Explain CLI commands

## BTC Timestamp File

The command `create-timestamp-transaction` is used to timestamp a file into bitcoin.
In this case, timestamping refers to generating a SHA256 hash of the file and
creating an output containing a `NullDataScript` with the hash in it and a zero
value. This transaction needs a funded encoded address that converts pubkeys to
P2WPKH addresses.

Following are instructions on how the timestamp can be generated and submitted
to Bitcoin. To execute the below commands, you need a running
[bitcoind](https://github.com/bitcoin/bitcoin) daemon that has access to a
funded wallet.

1. Create a new Wallet

```shell
$ bitcoin-cli createwallet timestamp-w

{
  "name": "timestamp-w"
}
```

2. Generate a new address

```shell
$ bitcoin-cli -rpcwallet=timestamp-w getnewaddress

bcrt1qagw463xxngygwsdjrm5f4etvr2hkq0yq0up8ly
```

3. Send funds to this address

```shell
$ bitcoin-cli -rpcwallet=wallet-with-funds sendtoaddress bcrt1qagw463xxngygwsdjrm5f4etvr2hkq0yq0up8ly 15

6ce4a2b2797da995d2f49ffd707d1bb34f1717205e6f7fa3d6f3f1aa8b2c9eca
```

4. Get the tx data of the `sendtoaddress` transaction

```shell
$ bitcoin-cli -rpcwallet=timestamp-w getrawtransaction 6ce4a2b2797da995d2f49ffd707d1bb34f1717205e6f7fa3d6f3f1aa8b2c9eca

020000000001014db94bb61981724775d3a9a27dc62facbb422e5d76887205019dab39e031f7400000000000fdffffff02002f685900000000160014ea1d5d44c69a088741b21ee89ae56c1aaf603c80389c9bd000000000160014dd619252c3983779e6938366ece5d4e2015172b3024730440220180472b948c4fc4e2c608462375b41ff545ef2d0fe97d3ae8ac1e6ad25a6de1f0220036e23c181cb4b65264a58524215e97b27501d1039cd3a2f7d009f860aa8ed46012102a932128cbf3ddf90e12a4c94a6acce81ea3d1ea1dc325101f90b013ccecc60fe95000000
```

5. Create the timestamp transaction using the `create-timestamp-transaction` utility.
The first parameter corresponds to the transaction data, the second is the file
that will be timestamped, and the last parameter is the address previously
created that will fund the transaction.

```shell
$ ./build/cli-tools create-timestamp-transaction 020000000001014db94bb61981724775d3a9a27dc62facbb422e5d76887205019dab39e031f7400000000000fdffffff02002f685900000000160014ea1d5d44c69a088741b21ee89ae56c1aaf603c80389c9bd000000000160014dd619252c3983779e6938366ece5d4e2015172b3024730440220180472b948c4fc4e2c608462375b41ff545ef2d0fe97d3ae8ac1e6ad25a6de1f0220036e23c181cb4b65264a58524215e97b27501d1039cd3a2f7d009f860aa8ed46012102a932128cbf3ddf90e12a4c94a6acce81ea3d1ea1dc325101f90b013ccecc60fe95000000 ./README.md bcrt1qagw463xxngygwsdjrm5f4etvr2hkq0yq0up8ly

{
  "timestamp_tx_hex": "0200000001ca9e2c8baaf1f3d6a37f6f5e2017174fb31b7d70fd9ff4d295a97d79b2a2e46c0000000000ffffffff023027685900000000160014ea1d5d44c69a088741b21ee89ae56c1aaf603c800000000000000000226a203531589f3a9715ed9a130aa2702a17d8251f767e5412447ecbcfedfef5b9798f00000000",
  "file_hash": "3531589f3a9715ed9a130aa2702a17d8251f767e5412447ecbcfedfef5b9798f"
}
```

6. Sign the transaction by passing the `timestamp_tx_hex` property of the
`create-timestamp-transaction` command.

```shell
$ bitcoin-cli -rpcwallet=timestamp-w signrawtransactionwithwallet 0200000001ca9e2c8baaf1f3d6a37f6f5e2017174fb31b7d70fd9ff4d295a97d79b2a2e46c0000000000ffffffff023027685900000000160014ea1d5d44c69a088741b21ee89ae56c1aaf603c800000000000000000226a203531589f3a9715ed9a130aa2702a17d8251f767e5412447ecbcfedfef5b9798f00000000

{
  "hex": "02000000000101ca9e2c8baaf1f3d6a37f6f5e2017174fb31b7d70fd9ff4d295a97d79b2a2e46c0000000000ffffffff023027685900000000160014ea1d5d44c69a088741b21ee89ae56c1aaf603c800000000000000000226a203531589f3a9715ed9a130aa2702a17d8251f767e5412447ecbcfedfef5b9798f0247304402206f272dcc7b94474dd6df3f0d4eafd5e96ef7e568ec391ca9e06b466cdb694677022039546f7fc09a955d2b71ba02b927a839b91865c72d04d811e2a0f2f0838030ef0121029928fe0c0b89122500dee8a6b29cdac54925770f0d484778ff2be878854e1c4a00000000",
  "complete": true
}
```

7. Broadcast the transaction to Bitcoin

```shell
$ bitcoin-cli -rpcwallet=timestamp-w sendrawtransaction 02000000000101ca9e2c8baaf1f3d6a37f6f5e2017174fb31b7d70fd9ff4d295a97d79b2a2e46c0000000000ffffffff023027685900000000160014ea1d5d44c69a088741b21ee89ae56c1aaf603c800000000000000000226a203531589f3a9715ed9a130aa2702a17d8251f767e5412447ecbcfedfef5b9798f0247304402206f272dcc7b94474dd6df3f0d4eafd5e96ef7e568ec391ca9e06b466cdb694677022039546f7fc09a955d2b71ba02b927a839b91865c72d04d811e2a0f2f0838030ef0121029928fe0c0b89122500dee8a6b29cdac54925770f0d484778ff2be878854e1c4a00000000

25b65b31c6f4d2f46ebeb5fa4c9a6d1fa4de03813404718f9797269b79023122
```

8. To verify whether the transactions has been included, you can query Bitcoin
and check the transaction's number of confirmations.

```shell
$ bitcoin-cli -rpcwallet=timestamp-w gettransaction 25b65b31c6f4d2f46ebeb5fa4c9a6d1fa4de03813404718f9797269b79023122

{
  "amount": 0.00000000,
  "fee": -0.00002000,
  "confirmations": 4,
  "blockhash": "46582163473053a7e5f8743a79675d9f9cc5cbf4890f28f1e7e816316b28ef69",
  "blockheight": 201,
  "blockindex": 2,
  "blocktime": 1715821032,
  "txid": "25b65b31c6f4d2f46ebeb5fa4c9a6d1fa4de03813404718f9797269b79023122",
  "wtxid": "3b2ce841ef57b43bc5b59d772ccc6fcaf38ca2bce94dbb787a5791a4ee0765df",
  "walletconflicts": [
  ],
  "time": 1715820976,
  "timereceived": 1715820976,
  "bip125-replaceable": "no",
  "details": [
    {
      "address": "bcrt1qagw463xxngygwsdjrm5f4etvr2hkq0yq0up8ly",
      "category": "send",
      "amount": -14.99998000,
      "label": "",
      "vout": 0,
      "fee": -0.00002000,
      "abandoned": false
    },
    {
      "category": "send",
      "amount": 0.00000000,
      "vout": 1,
      "fee": -0.00002000,
      "abandoned": false
    },
    {
      "address": "bcrt1qagw463xxngygwsdjrm5f4etvr2hkq0yq0up8ly",
      "parent_descs": [
        "wpkh(tpubD6NzVbkrYhZ4X4XfxiyNepqE9Uy48atSoXCcd68V1hNokNUFiVUF6AptTPPDcU2mnLumM3m5Jq3eLjaoNd2vQdDtNjhGyfU2HJVSBEPYdtD/84h/1h/0h/0/*)#fawa8eh6"
      ],
      "category": "receive",
      "amount": 14.99998000,
      "label": "",
      "vout": 0,
      "abandoned": false
    }
  ],
  "hex": "02000000000101ca9e2c8baaf1f3d6a37f6f5e2017174fb31b7d70fd9ff4d295a97d79b2a2e46c0000000000ffffffff023027685900000000160014ea1d5d44c69a088741b21ee89ae56c1aaf603c800000000000000000226a203531589f3a9715ed9a130aa2702a17d8251f767e5412447ecbcfedfef5b9798f0247304402206f272dcc7b94474dd6df3f0d4eafd5e96ef7e568ec391ca9e06b466cdb694677022039546f7fc09a955d2b71ba02b927a839b91865c72d04d811e2a0f2f0838030ef0121029928fe0c0b89122500dee8a6b29cdac54925770f0d484778ff2be878854e1c4a00000000",
  "lastprocessedblock": {
    "hash": "27bc75d8210c69b72f0304aecb8c72c109691b8f94dd7d022ae5fc9026fb4008",
    "height": 204
  }
}
```

## BTC Transaction creation commands

The following set of commands are used to create phase-1 compatible transactions
with the goal to ease up testing of the phase-1 system:
- `create-phase1-staking-tx`
- `create-phase1-unbonding-request`
- `create-phase1-withdaw-request`

Disclaimer: Those commands should only be used for testing purposes and should not be
used with real BTC.

## Dump config command

Some of the commands require a config file to work properly. To generate a config
file the `dump-cfg` command can be used.

```shell
cli-tools dump-cfg --config "./config.toml"
```

will generate a `config.toml` file in the same directory as the one
the `cli-tools` program binary resides in.


## Unbonding processing commands

There are two commands responsible for processing unbonding requests:
- `run-unbonding-pipeline`
- `process-failed-transactions`

Both of those commands require:
- config file path
- global parameters paths

Example:

```shell
cli-tools run-unbonding-pipeline --config "./config/config.toml" --params "./config/global-params.json"
```

For these commands to work, they must have access to the data source (mongo-db) containing
unbonding transactions already signed by staker and validated by some other process.
