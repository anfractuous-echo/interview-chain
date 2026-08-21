package symptoms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"interview-chain/internal/blockchain"
	"interview-chain/internal/indexer"
	"interview-chain/internal/payment"
	"interview-chain/internal/paymentstore"
	"interview-chain/internal/transaction"
)

func TestScenarioReplay(t *testing.T) {
	ctx := context.Background()
	database, err := paymentstore.OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	first := payment.Instruction{ID: "same-id", From: "alice", To: "bob", Amount: "1.000000", AmountUnits: 1_000_000, RawJSON: []byte("first")}
	conflict := payment.Instruction{ID: "same-id", From: "alice", To: "dave", Amount: "900.000000", AmountUnits: 900_000_000, RawJSON: []byte("conflict")}
	if err := database.InsertPayment(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := database.InsertPayment(ctx, conflict); err == nil {
		t.Fatal("expected conflicting replay to be rejected")
	}
	payments, err := database.ListReceivedPayments(ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 1 || payments[0].To != "bob" {
		t.Fatalf("unexpected replay result: %+v", payments)
	}
	t.Logf("conflict rejected; durable row is receiver=%s amount=%s", payments[0].To, payments[0].Amount)
}

func TestScenarioRestart(t *testing.T) {
	ctx := context.Background()
	firstHash := blockchain.CalculateBlockHash("", 1, "ok")
	page := blockchain.BlockPage{
		Blocks: []blockchain.Block{
			{Height: 1, Hash: firstHash, Transactions: []transaction.Signed{{ID: "ok", PaymentID: "p1", From: "alice", To: "bob", AmountUnits: 1, Signature: "sig"}}},
			{Height: 2, Hash: blockchain.CalculateBlockHash(firstHash, 2, "bad"), PreviousHash: firstHash, Transactions: []transaction.Signed{{ID: "bad", PaymentID: "p2", From: "alice", To: "bob", AmountUnits: 0, Signature: "sig"}}},
		},
		NextHeight: 2,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer server.Close()
	database, err := indexer.OpenSQLiteStore(filepath.Join(t.TempDir(), "indexer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := indexer.NewService(database, indexer.NewNodeClient(server.URL), 100)
	if err := service.PollOnce(ctx); err == nil {
		t.Fatal("expected page processing error")
	}
	checkpoint, err := database.Checkpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	blocks, transactions, err := database.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("persisted state: checkpoint=%d blocks=%d transactions=%d", checkpoint, blocks, transactions)
}
