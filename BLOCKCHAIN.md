# Toy Blockchain Contract

The node is deliberately smaller than a real blockchain. These rules are the complete domain contract.

## Accounts and amounts

- Accounts have an address and a balance.
- Amounts are positive signed 64-bit integers in smallest units.
- One display unit equals 1,000,000 smallest units.
- The node is authoritative for balance validation.
- There are no nonces, fees, reorgs, or confirmation depth after inclusion.
- `GET /accounts/{address}` reports balances from mined blocks. Pending transactions reserve funds for validation but do not change the reported balance.

## Transaction

```json
{
  "id": "hex sha256",
  "payment_id": "pay-001",
  "from": "alice",
  "to": "bob",
  "amount_units": 12340000,
  "signature": "hex sha256"
}
```

The canonical payload is the UTF-8 JSON encoding of this fixed field order, with no extra whitespace:

```json
{"version":1,"payment_id":"pay-001","from":"alice","to":"bob","amount_units":12340000}
```

The transaction package exposes `CanonicalPayload`, which produces these bytes.

Toy signature:

```text
signature = hex(SHA256(secret || 0x00 || canonical_payload))
```

Transaction ID:

```text
id = hex(SHA256(canonical_payload || 0x00 || raw_signature_bytes))
```

The node knows the fixture secrets so it can verify the toy signature. This is not a real public-key signature scheme.

## Submission and mining

When `POST /transactions` succeeds, the node immediately:

1. verifies the transaction ID and signature;
2. checks accounts, available balance, and receiver overflow, including transactions already pending;
3. reserves the balance change;
4. adds the transaction to the in-memory pending pool.

The response does not wait for a block and has status `pending`. The same transaction ID with exactly the same payload is an idempotent duplicate. Reusing an existing ID with a different payload is a conflict.

By default the node mines one block every 10 seconds. Each block contains all transactions accepted since the previous block, in acceptance order. When the pending pool is empty, the node still mines an empty block. All transactions in a mined block become `included`, their reserved balance changes become visible, and they share the block height.

Blocks are final and returned in ascending height order. The node stores everything in memory, so restarting it creates a new chain from the wallet fixture.

Block hashes use the ordered transaction IDs:

```text
hash = hex(SHA256(previous_hash || 0x00 || decimal_height ||
                  0x00 || transaction_id_1 || ... || 0x00 || transaction_id_N))
```

An empty block hashes only `previous_hash || 0x00 || decimal_height`.

## REST API

### `POST /transactions`

- `202 Accepted`: accepted into the pending pool.
- `200 OK`: exact duplicate already pending or included.
- `400 Bad Request`: malformed transaction, invalid ID, or invalid signature.
- `404 Not Found`: unknown account.
- `409 Conflict`: insufficient balance or conflicting transaction ID.

Pending response:

```json
{"transaction_id":"...","status":"pending","duplicate":false}
```

An included duplicate also contains its block height:

```json
{"transaction_id":"...","status":"included","block_height":7,"duplicate":true}
```

### `GET /transactions/{id}`

Returns a pending transaction without `block_height`:

```json
{"transaction":{},"status":"pending"}
```

After mining it returns `status: "included"` and `block_height`. An unknown transaction returns `404`.

### `GET /accounts/{address}`

Returns `{"address":"alice","balance_units":1000000}`, or `404`.

### `GET /blocks?after_height=N&limit=M`

Returns blocks strictly after `N`, up to `M` entries:

```json
{"blocks":[],"next_height":0,"has_more":false}
```

`next_height` is the last returned height, or the supplied `after_height` when the page is empty. The maximum page size is 1,000.

`transactions` is an empty JSON array for an empty block. A non-empty block may contain multiple transactions.
