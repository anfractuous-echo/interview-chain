package indexer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"interview-chain/internal/blockchain"
	"interview-chain/internal/transaction"
)

func TestPollOnceIndexesSmallPage(t *testing.T) {
	t.Parallel()
	page := blockchain.BlockPage{NextHeight: 3}
	for n := int64(1); n <= 3; n++ {
		tx := transaction.Signed{ID: paymentID(n), PaymentID: paymentID(n), From: "alice", To: "bob", AmountUnits: 1, Signature: "test"}
		page.Blocks = append(page.Blocks, blockchain.Block{Height: n, Hash: paymentID(n), Transactions: []transaction.Signed{tx}})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer server.Close()
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "indexer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewService(store, NewNodeClient(server.URL), 100)
	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	blocks, transactions, err := store.Counts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if blocks != 3 || transactions != 3 {
		t.Fatalf("counts = (%d, %d), want (3, 3)", blocks, transactions)
	}
}

func TestPollOnceIndexesEmptyBlock(t *testing.T) {
	t.Parallel()
	page := blockchain.BlockPage{
		Blocks:     []blockchain.Block{{Height: 1, Hash: "empty-1", Transactions: []transaction.Signed{}}},
		NextHeight: 1,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer server.Close()
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "indexer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewService(store, NewNodeClient(server.URL), 100)
	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	blocks, transactions, err := store.Counts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if blocks != 1 || transactions != 0 {
		t.Fatalf("counts = (%d, %d), want (1, 0)", blocks, transactions)
	}
}

func paymentID(n int64) string {
	return string(rune('a' + n))
}
