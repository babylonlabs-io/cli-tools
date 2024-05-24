# CLI-Tools

Set of CLI tool which are used for different purposes:
- processing unbonding requests
- building test staking/unbonding/withdraw phase-1 Bitcoin transactions
- building Bitcon transactions timestamping files

See  [deployment guide](/docs/commands.md) to review all available commands.

### Installation

#### Prerequisites

This project requires Go version 1.21 or later.

Install Go by following the instructions on the official Go installation [guide](https://go.dev/doc/install).

#### Download the code

To get started, clone the repository to your local machine from GitHub; please
use the version according to the phase-1 system guidelines --
you can find all versions in the official
[releases](https://github.com/babylonchain/cli-tools/releases) page.

```shell
git clone https://github.com/babylonchain/cli-tools.git
cd cli-tools
git checkout <release-tag>
```

#### Build and install the binary

At the top-level directory of the project

```shell
make install
```

The above command will build and install the `cli-tools` binary to
`$GOPATH/bin`.

If your shell cannot find the installed binaries, make sure `$GOPATH/bin` is in
the `$PATH` of your shell. The following updates to `$PATH` can be performed to
this direction:

```shell
export PATH=$HOME/go/bin:$PATH
echo 'export PATH=$HOME/go/bin:$PATH' >> ~/.profile
```

### Usage

To see all available commands:

```shell
cli-tools --help
```

Example output:

```shell
Set of cli tools used in phase-1

Usage:
  cli-tools [command]

Available Commands:
  completion                      Generate the autocompletion script for the specified shell
  create-phase1-staking-tx        create phase1 staking tx
  create-phase1-unbonding-request create phase1 unbonding tx
  create-phase1-withdaw-request   create phase1 withdraw tx
  create-timestamp-transaction    Creates a timestamp btc transaction by hashing the file input.
  dump-cfg                        dumps default confiiguration file
  help                            Help about any command
  process-failed-transactions     tries to re-send unbonding transactions that failed to be sent to the network
  run-unbonding-pipeline          runs unbonding pipeline

Flags:
      --config string   path to the directory with configuration file
  -h, --help            help for cli-tools
      --params string   path to the global params file

Use "cli-tools [command] --help" for more information about a command.
```
