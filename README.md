# Interview Chain

Этот репозиторий моделирует небольшой платёжный pipeline:

```text
payments.jsonl -> payment sender -> toy blockchain -> indexer -> SQLite
```

Система намеренно сделана компактной. Читайте код как небольшой внутренний production-сервис. Будьте готовы объяснить, что вы изменили бы, почему это важно и какие компромиссы создаёт ваше изменение.

## Компоненты

### Входные данные

`input-data/payments.jsonl` является внешним источником платежей. Каждая строка описывает один платёж со стабильным `payment_id`, отправителем, получателем и decimal-суммой. В `input-data/wallet.json` находятся тестовые аккаунты, signing secrets и genesis balances для локальной node.

### Payment sender

`payment-sender` импортирует JSONL в собственную SQLite-базу, строит стабильную подписанную transaction для каждого платежа, отправляет её в node и записывает факт принятия. Та же локальная база используется после restart, поэтому retry может повторно использовать уже materialized transaction, а не создавать новую identity.

### Toy blockchain

`toy-chain` является in-memory node с небольшим REST API. Она проверяет transaction ID, toy signature, аккаунты и доступные balances. Корректный `POST /transactions` сразу помещает transaction в pending pool. По умолчанию node выпускает один block каждые 10 секунд. В block входят все transaction, накопленные после предыдущего block, либо пустой список, если новых transaction нет. Balances изменяются при выпуске block.

После restart node теряет in-memory chain и pending pool, а balances возвращаются к wallet fixture.

### Indexer

`indexer` опрашивает node и записывает blocks вместе с их transactions в отдельную SQLite-базу. Пустые blocks являются корректными и тоже должны индексироваться. Checkpoint позволяет продолжить после restart без повторного чтения всей chain.

При запуске indexer пишет в log checkpoint и counts, уже находящиеся в его database. После каждого успешного fetch он показывает сохранённый диапазон blocks, batch counts, новый checkpoint и height node, когда он её догнал. Fetch limit является только максимальным числом blocks в одном HTTP-запросе.

## Что реализует кандидат

В starter-репозитории оставлены три функции:

- `ReadPaymentFile` читает и проверяет JSONL, переводит decimal-суммы в smallest units и сохраняет данные исходной строки, необходимые importer;
- `BuildSignedTransaction` следует `BLOCKCHAIN.md`, чтобы построить canonical toy signature и стабильный transaction ID;
- `ProcessPayment` надёжно materialize-ит transaction, отправляет именно сохранённый объект в node и записывает факт принятия с учётом replay.

Окружающие контракты parser, SQLite stores, HTTP clients, CLI, node и indexer уже существуют. Репозиторий компилируется до реализации этих функций и возвращает понятную ошибку при достижении недостающего кода.

## Требования

- Go 1.24 или новее
- Make

Docker, внешняя база данных и предварительное знание blockchain не требуются. Контракт blockchain описан в [BLOCKCHAIN.md](BLOCKCHAIN.md).

## Запуск

Запустите каждый долгоживущий процесс в отдельном терминале:

```bash
make chain
make indexer
make sender
```

Chain пишет в log каждый выпущенный block. Первый block появляется примерно через 10 секунд после запуска, даже если transaction не поступали. Для локальных экспериментов интервал можно изменить:

```bash
go run ./cmd/toy-chain -listen :8080 -wallet input-data/wallet.json -block-interval 1s
```

Полезные команды:

```bash
make test
make vet
make generate
make scenario-replay
make scenario-restart
make clean
```

Runtime-файлы SQLite записываются в `.data/`.

## Входные данные

Каждая строка `input-data/payments.jsonl` является отдельным платежом:

```json
{"payment_id":"pay-001","from":"alice","to":"bob","amount":"12.340000"}
```

На границе blockchain суммы имеют ровно шесть знаков после точки. Ключи аккаунтов и genesis balances находятся в `input-data/wallet.json`. Это понятные примеры входных данных, а не настоящие секреты.
