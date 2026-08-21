.PHONY: chain indexer sender generate test vet scenario-replay scenario-restart scenario-race clean

DATA_DIR := .data

chain:
	@mkdir -p $(DATA_DIR)
	go run ./cmd/toy-chain -listen :8080 -wallet input-data/wallet.json

indexer:
	@mkdir -p $(DATA_DIR)
	go run ./cmd/indexer -node http://localhost:8080 -db $(DATA_DIR)/indexer.db

sender:
	@mkdir -p $(DATA_DIR)
	go run ./cmd/payment-sender -node http://localhost:8080 -db $(DATA_DIR)/sender.db -wallet input-data/wallet.json -payments input-data/payments.jsonl

generate:
	go run ./cmd/payment-generator -count 100000 -out payments.generated.jsonl

test:
	go test ./...

vet:
	go vet ./...

scenario-replay:
	go test -v ./internal/symptoms -run TestScenarioReplay -count=1

scenario-restart:
	go test -v ./internal/symptoms -run TestScenarioRestart -count=1

scenario-race:
	go test -race ./internal/indexer -run TestPollOnceIndexesSmallPage -count=10

clean:
	@mkdir -p $(DATA_DIR)
	rm -f $(DATA_DIR)/sender.db $(DATA_DIR)/sender.db-shm $(DATA_DIR)/sender.db-wal
	rm -f $(DATA_DIR)/indexer.db $(DATA_DIR)/indexer.db-shm $(DATA_DIR)/indexer.db-wal
	rm -f payments.generated.jsonl
