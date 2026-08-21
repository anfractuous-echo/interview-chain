package toychain

import (
	"errors"
	"math"
	"testing"

	"interview-chain/internal/blockchain"
	"interview-chain/internal/transaction"
	"interview-chain/internal/wallet"
)

func signedTransaction(t *testing.T, paymentID string, amount int64) transaction.Signed {
	t.Helper()
	tx := transaction.Signed{PaymentID: paymentID, From: "alice", To: "bob", AmountUnits: amount}
	var err error
	tx.Signature, err = calculateExpectedSignature(tx, "alice-secret")
	if err != nil {
		t.Fatal(err)
	}
	tx.ID, err = calculateExpectedID(tx)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func testWallet() wallet.Snapshot {
	return wallet.Snapshot{Accounts: []wallet.Account{
		{Address: "alice", Secret: "alice-secret", BalanceUnits: 100},
		{Address: "bob", Secret: "bob-secret", BalanceUnits: 0},
	}}
}

func TestSubmitQueuesUntilBlockAndIsIdempotent(t *testing.T) {
	t.Parallel()
	ledger := NewLedger(testWallet())
	tx := signedTransaction(t, "pay-1", 25)
	first, err := ledger.Submit(tx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ledger.Submit(tx)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != blockchain.TransactionPending || first.BlockHeight != nil || second.Status != blockchain.TransactionPending || !second.Duplicate {
		t.Fatalf("unexpected results: first=%+v second=%+v", first, second)
	}
	alice, _ := ledger.Account("alice")
	bob, _ := ledger.Account("bob")
	if alice.BalanceUnits != 100 || bob.BalanceUnits != 0 {
		t.Fatalf("pending transaction changed balances: alice=%d bob=%d", alice.BalanceUnits, bob.BalanceUnits)
	}
	block := ledger.MineBlock()
	if block.Height != 1 || len(block.Transactions) != 1 || block.Transactions[0].ID != tx.ID {
		t.Fatalf("unexpected block: %+v", block)
	}
	alice, _ = ledger.Account("alice")
	bob, _ = ledger.Account("bob")
	if alice.BalanceUnits != 75 || bob.BalanceUnits != 25 {
		t.Fatalf("unexpected balances: alice=%d bob=%d", alice.BalanceUnits, bob.BalanceUnits)
	}
	third, err := ledger.Submit(tx)
	if err != nil {
		t.Fatal(err)
	}
	if third.Status != blockchain.TransactionIncluded || third.BlockHeight == nil || *third.BlockHeight != 1 || !third.Duplicate {
		t.Fatalf("unexpected included duplicate: %+v", third)
	}
	if page := ledger.BlocksAfter(0, 10); len(page.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(page.Blocks))
	}
}

func TestMineBlockProducesEmptyBlock(t *testing.T) {
	t.Parallel()
	ledger := NewLedger(testWallet())
	first := ledger.MineBlock()
	second := ledger.MineBlock()
	if first.Height != 1 || len(first.Transactions) != 0 {
		t.Fatalf("unexpected first block: %+v", first)
	}
	if second.Height != 2 || second.PreviousHash != first.Hash || len(second.Transactions) != 0 {
		t.Fatalf("unexpected second block: %+v", second)
	}
}

func TestBlocksAfterReturnsEmptyArrayBeforeFirstBlock(t *testing.T) {
	t.Parallel()
	page := NewLedger(testWallet()).BlocksAfter(0, 10)
	if page.Blocks == nil || len(page.Blocks) != 0 {
		t.Fatalf("blocks = %#v, want non-nil empty slice", page.Blocks)
	}
}

func TestMineBlockIncludesAllPendingTransactionsInAcceptanceOrder(t *testing.T) {
	t.Parallel()
	ledger := NewLedger(testWallet())
	first := signedTransaction(t, "pay-1", 25)
	second := signedTransaction(t, "pay-2", 30)
	if _, err := ledger.Submit(first); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Submit(second); err != nil {
		t.Fatal(err)
	}

	block := ledger.MineBlock()
	if len(block.Transactions) != 2 {
		t.Fatalf("got %d transactions, want 2", len(block.Transactions))
	}
	if block.Transactions[0].ID != first.ID || block.Transactions[1].ID != second.ID {
		t.Fatalf("unexpected transaction order: %+v", block.Transactions)
	}
	for _, tx := range []transaction.Signed{first, second} {
		result, err := ledger.Transaction(tx.ID)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != blockchain.TransactionIncluded || result.BlockHeight == nil || *result.BlockHeight != block.Height {
			t.Fatalf("unexpected transaction result: %+v", result)
		}
	}
}

func TestPendingTransactionsReserveBalance(t *testing.T) {
	t.Parallel()
	ledger := NewLedger(testWallet())
	if _, err := ledger.Submit(signedTransaction(t, "pay-1", 80)); err != nil {
		t.Fatal(err)
	}
	_, err := ledger.Submit(signedTransaction(t, "pay-2", 30))
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("got %v, want ErrInsufficientFunds", err)
	}
}

func TestSubmitRejectsInsufficientFunds(t *testing.T) {
	t.Parallel()
	ledger := NewLedger(testWallet())
	_, err := ledger.Submit(signedTransaction(t, "pay-2", 101))
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("got %v, want ErrInsufficientFunds", err)
	}
}

func TestSubmitRejectsReceiverOverflow(t *testing.T) {
	t.Parallel()
	accounts := wallet.Snapshot{Accounts: []wallet.Account{
		{Address: "alice", Secret: "alice-secret", BalanceUnits: 100},
		{Address: "bob", Secret: "bob-secret", BalanceUnits: math.MaxInt64},
	}}
	ledger := NewLedger(accounts)
	_, err := ledger.Submit(signedTransaction(t, "pay-overflow", 1))
	if !errors.Is(err, ErrBalanceOverflow) {
		t.Fatalf("got %v, want ErrBalanceOverflow", err)
	}
}
