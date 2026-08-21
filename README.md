# Interview Chain

This repository models a small payment pipeline:

```text
payments.jsonl -> payment sender -> toy blockchain -> indexer -> SQLite
```

The system is intentionally compact. Read the code as if it were a small internal production service. Be ready to explain what you would change, why it matters, and what trade-offs your change introduces.

## Components

### Input data

`input-data/payments.jsonl` is the external payment input. Each line describes one payment with a stable `payment_id`, sender, receiver, and decimal amount. `input-data/wallet.json` contains the toy accounts, signing secrets, and genesis balances used by the local node.

### Payment sender

`payment-sender` imports the JSONL file into its own SQLite database, builds a stable signed transaction for each payment, submits it to the node, and records that the node accepted it. Its local database is also used when the process restarts, so a retry can reuse the transaction that was already materialized instead of creating a different identity.

### Toy blockchain

`toy-chain` is an in-memory node with a small REST API. It validates transaction IDs, toy signatures, accounts, and available balances. A valid `POST /transactions` is accepted immediately into a pending pool. By default the node mines one block every 10 seconds. A block contains every transaction accumulated since the previous block, or an empty transaction list when nothing was submitted. Balances change when the block is mined.

Restarting the node resets its in-memory chain, pending pool, and balances to the wallet fixture.

### Indexer

`indexer` polls blocks from the node and writes blocks and their transactions into a separate SQLite database. Empty blocks are valid and must still be indexed. The indexer keeps a checkpoint so it can continue after a restart without reading the complete chain every time.

At startup, the indexer logs the checkpoint and counts already present in its database. After each successful fetch, it logs the saved block range, batch counts, its new checkpoint, and the node height when it has caught up. The configured fetch limit is only the maximum number of blocks requested in one HTTP call.

## Candidate implementation

The starter repository contains three functions for the candidate to implement:

- `ReadPaymentFile` reads and validates the JSONL input, converts decimal amounts into smallest units, and preserves the source line information needed by the importer.
- `BuildSignedTransaction` follows `BLOCKCHAIN.md` to create the canonical toy signature and stable transaction ID.
- `ProcessPayment` materializes a transaction durably, submits that exact transaction to the node, and records the accepted submission while remaining safe to replay.

The surrounding parser contracts, SQLite stores, HTTP clients, CLIs, node, and indexer are already present. The repository compiles before these functions are implemented and reports a clear error when the missing path is reached.

## Requirements

- Go 1.24 or newer
- Make

No Docker, external database, or blockchain knowledge is required. The blockchain contract is documented in [BLOCKCHAIN.md](BLOCKCHAIN.md).

## Run

Start each long-running process in its own terminal:

```bash
make chain
make indexer
make sender
```

The chain logs every mined block. The first block appears approximately 10 seconds after startup even when no transactions were submitted. The interval can be changed for local experiments:

```bash
go run ./cmd/toy-chain -listen :8080 -wallet input-data/wallet.json -block-interval 1s
```

Useful commands:

```bash
make test
make vet
make generate
make scenario-replay
make scenario-restart
make clean
```

Runtime SQLite files are written to `.data/`.

## Input

Each line in `input-data/payments.jsonl` is one payment:

```json
{"payment_id":"pay-001","from":"alice","to":"bob","amount":"12.340000"}
```

Amounts use exactly six decimal places at the blockchain boundary. Account keys and genesis balances are provided in `input-data/wallet.json`; they are example input data, not real secrets.
