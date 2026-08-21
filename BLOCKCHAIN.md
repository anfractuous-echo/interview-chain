# Контракт toy blockchain

Эта node намеренно гораздо проще настоящего blockchain. Перечисленные правила являются полным domain contract.

## Аккаунты и суммы

- У аккаунта есть address и balance.
- Amount является положительным signed 64-bit integer в smallest units.
- Одна отображаемая единица равна 1 000 000 smallest units.
- Node является authoritative системой для проверки balance.
- Nonce, fee, reorg и confirmation depth после inclusion отсутствуют.
- `GET /accounts/{address}` возвращает balances из выпущенных blocks. Pending transactions резервируют средства для validation, но не меняют отображаемый balance.

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

Canonical payload является UTF-8 JSON в фиксированном порядке полей и без дополнительного whitespace:

```json
{"version":1,"payment_id":"pay-001","from":"alice","to":"bob","amount_units":12340000}
```

Пакет transaction экспортирует `CanonicalPayload`, который создаёт эти bytes.

Toy signature:

```text
signature = hex(SHA256(secret || 0x00 || canonical_payload))
```

Transaction ID:

```text
id = hex(SHA256(canonical_payload || 0x00 || raw_signature_bytes))
```

Node знает fixture secrets и может проверить toy signature. Это не настоящая public-key signature scheme.

## Submission и mining

При успешном `POST /transactions` node сразу:

1. проверяет transaction ID и signature;
2. проверяет accounts, доступный balance и overflow у receiver с учётом уже pending transactions;
3. резервирует изменение balances;
4. добавляет transaction в in-memory pending pool.

Ответ не ждёт выпуска block и содержит status `pending`. Повтор с той же transaction ID и полностью идентичным payload является идемпотентным. Использование существующей ID с другим payload является конфликтом.

По умолчанию node выпускает один block каждые 10 секунд. В него входят все transactions, принятые после предыдущего block, в порядке приёма. Если pending pool пуст, node всё равно выпускает пустой block. После выпуска transactions получают status `included`, зарезервированные изменения balances становятся видимыми, а все transactions из block получают одну height.

Blocks являются final и возвращаются по возрастанию height. Node хранит всё в памяти, поэтому restart создаёт новую chain из wallet fixture.

Block hash учитывает упорядоченные transaction IDs:

```text
hash = hex(SHA256(previous_hash || 0x00 || decimal_height ||
                  0x00 || transaction_id_1 || ... || 0x00 || transaction_id_N))
```

Для пустого block hash вычисляется только из `previous_hash || 0x00 || decimal_height`.

## REST API

### `POST /transactions`

- `202 Accepted`: transaction принята в pending pool.
- `200 OK`: идентичная transaction уже находится в pending pool или включена в block.
- `400 Bad Request`: некорректная transaction, ID или signature.
- `404 Not Found`: неизвестный account.
- `409 Conflict`: недостаточный balance или конфликтующая transaction ID.

Ответ для pending transaction:

```json
{"transaction_id":"...","status":"pending","duplicate":false}
```

Ответ для уже included duplicate также содержит block height:

```json
{"transaction_id":"...","status":"included","block_height":7,"duplicate":true}
```

### `GET /transactions/{id}`

Для pending transaction возвращает ответ без `block_height`:

```json
{"transaction":{},"status":"pending"}
```

После mining возвращает `status: "included"` и `block_height`. Для неизвестной transaction возвращает `404`.

### `GET /accounts/{address}`

Возвращает `{"address":"alice","balance_units":1000000}` либо `404`.

### `GET /blocks?after_height=N&limit=M`

Возвращает blocks строго после `N`, не более `M` записей:

```json
{"blocks":[],"next_height":0,"has_more":false}
```

`next_height` равен height последнего возвращённого block либо переданному `after_height`, если page пуста. Максимальный размер page равен 1 000.

Для пустого block поле `transactions` является пустым JSON array. Непустой block может содержать несколько transactions.
