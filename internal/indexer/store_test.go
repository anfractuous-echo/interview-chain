package indexer

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"interview-chain/internal/blockchain"
	"interview-chain/internal/transaction"
)

func TestSaveBlockIsAtomicAndDetectsConflicts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "indexer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	firstTransaction := transaction.Signed{ID: "tx-1", PaymentID: "pay-1", From: "alice", To: "bob", AmountUnits: 1, Signature: "signature"}
	firstBlock := blockchain.Block{Height: 1, Hash: blockchain.CalculateBlockHash("", 1, firstTransaction.ID), Transactions: []transaction.Signed{firstTransaction}}
	if err := store.SaveBlock(ctx, firstBlock); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBlock(ctx, firstBlock); err != nil {
		t.Fatalf("identical block replay: %v", err)
	}

	otherTransaction := firstTransaction
	otherTransaction.ID = "tx-other"
	conflictingBlock := blockchain.Block{Height: 1, Hash: blockchain.CalculateBlockHash("", 1, otherTransaction.ID), Transactions: []transaction.Signed{otherTransaction}}
	if err := store.SaveBlock(ctx, conflictingBlock); !errors.Is(err, ErrBlockConflict) {
		t.Fatalf("block conflict error = %v, want ErrBlockConflict", err)
	}

	conflictingTransaction := firstTransaction
	conflictingTransaction.To = "mallory"
	secondBlock := blockchain.Block{Height: 2, Hash: blockchain.CalculateBlockHash(firstBlock.Hash, 2, conflictingTransaction.ID), PreviousHash: firstBlock.Hash, Transactions: []transaction.Signed{conflictingTransaction}}
	if err := store.SaveBlock(ctx, secondBlock); !errors.Is(err, ErrIndexedTransaction) {
		t.Fatalf("transaction conflict error = %v, want ErrIndexedTransaction", err)
	}

	blocks, transactions, err := store.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if blocks != 1 || transactions != 1 {
		t.Fatalf("counts = (%d, %d), want (1, 1)", blocks, transactions)
	}
}

func TestSaveBlockRejectsInvalidTransactionBeforeWriting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "indexer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	block := blockchain.Block{
		Height: 1,
		Hash:   blockchain.CalculateBlockHash("", 1, "tx-1", "tx-2"),
		Transactions: []transaction.Signed{
			{ID: "tx-1", PaymentID: "pay-1", From: "alice", To: "bob", AmountUnits: 1, Signature: "signature"},
			{ID: "tx-2", PaymentID: "pay-2", From: "alice", To: "bob", AmountUnits: 0, Signature: "signature"},
		},
	}
	if err := store.SaveBlock(ctx, block); err == nil {
		t.Fatal("expected invalid transaction error")
	}
	blocks, transactions, err := store.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if blocks != 0 || transactions != 0 {
		t.Fatalf("counts = (%d, %d), want (0, 0)", blocks, transactions)
	}
}
