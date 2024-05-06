<img align="left" src="./.github/resources/goomy.png" width="75">
<h1>Spamooor the Transaction Spammer</h1>

spamooor is a simple tool that can be used to generate various types of random transactions for astria testnets.

This is a fork of https://github.com/ethpandaops/spamoor . It is a modification of the original spamoor tool to work with Astria load testing pipelines to test Astria EVM.

spamooor provides two commands:
* `spamooor`: Tool for spamming txs on the network

## Build

Or build it yourself:

```
git clone https://github.com/astriaorg/goomy-blob
cd goomy-blob
go build ./cmd/spamooor
```



## Usage

### `spamooor`
`spamooor` is a tool for spamming an EVM with a lot of transactions.

```
Usage of spamooor:
Required:
  -p, --privkey string        The private key of the wallet to send funds from.
  
  -h, --rpchost stringArray        The RPC host to send transactions to (multiple allowed).
      --rpchost-file string   File with a list of RPC hosts to send transactions to.
      
Optional:
  -s, --seed string           The child wallet seed.
  -v, --verbose               Run the tool with verbose output.
```

The tool provides multiple scenarios, that focus on different aspects of blob transactions. One of the scenarios must be selected to run the tool:

#### `spamooor eoatx`

This sends out a lot of EOAs (Externally Owned Accounts) transactions to the network.

```
Usage of spamooor combined:
Required (at least one of):
  -c, --count uint            Total number of transactions to send
  -t, --throughput uint       Number of transactions to send per block time
  
Optional:
      --amount uint           Transfer amount per transaction (in gwei) (default 20)
      --basefee uint          Max fee per gas to use in transfer transactions (in gwei) (default 20)
      --max-pending uint      Maximum number of pending transactions
      --max-wallets uint      Maximum number of child wallets to use
  -p, --privkey string        The private key of the wallet to send funds from.
      --random-amount         Use random amounts for transactions (with --amount as limit)
      --random-target         Use random to addresses for transactions
  -s, --seed string           The child wallet seed.
  -t, --throughput uint       Number of transfer transactions to send per slot
      --timeout uint          Number of seconds to wait before timing out the test (default 120)
      --tipfee uint           Max tip per gas to use in transfer transactions (in gwei) (default 2)
      --trace                 Run the script with tracing output
  -v, --verbose               Run the script with verbose output
```

#### `spamooor erctx`

This sends out a lot of ERC20 Txs transactions to the network. Options are the same as for `spamooor eoatx`.

#### `spamooor gasburnertx`

This sends out a lot of gas burner transactions to the network. Options are the same as for `spamooor eoatx`.
There is one extra optional parameter:

```
      --gas-units-to-burn uint      The number of gas units for each tx to cost
```

#### `spamooor univ2tx`

This sends out a lot of Uniswap V2 ETH/DAI swaps transactions to the network. Options are the same as for `spamooor eoatx`.

```
      --dai-mint-amount uint      The amount of Dai to mint for each child wallet
      --amount-to-swap uint         The amount of Eth/Dai to swap in each tx
      --random-amount-to-swap bool Whether to use random amounts to swap
```