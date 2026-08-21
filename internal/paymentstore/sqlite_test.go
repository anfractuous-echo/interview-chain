package paymentstore

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"interview-chain/internal/payment"
	"interview-chain/internal/transaction"
)

func TestSQLiteStoreRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	item := payment.Instruction{ID: "pay-1", From: "alice", To: "bob", Amount: "1.000000", AmountUnits: 1_000_000, RawJSON: []byte(`{"payment_id":"pay-1"}`)}
	if err := store.InsertPayment(ctx, item); err != nil {
		t.Fatal(err)
	}
	payments, err := store.ListReceivedPayments(ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 1 || payments[0].ID != item.ID {
		t.Fatalf("unexpected payments: %+v", payments)
	}
}

func TestInsertPaymentDistinguishesReplayFromConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	item := payment.Instruction{ID: "pay-1", From: "alice", To: "bob", Amount: "1.000000", AmountUnits: 1_000_000, RawJSON: []byte(`{"payment_id":"pay-1"}`)}
	if err := store.InsertPayment(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertPayment(ctx, item); err != nil {
		t.Fatalf("identical replay: %v", err)
	}
	equivalent := item
	equivalent.Amount = "1"
	equivalent.RawJSON = []byte(`{"payment_id":"pay-1","amount":"1"}`)
	if err := store.InsertPayment(ctx, equivalent); err != nil {
		t.Fatalf("semantically equivalent replay: %v", err)
	}

	conflict := item
	conflict.To = "mallory"
	if err := store.InsertPayment(ctx, conflict); !errors.Is(err, ErrPaymentConflict) {
		t.Fatalf("conflict error = %v, want ErrPaymentConflict", err)
	}
}

func TestImportedThroughLineOnlyMovesForward(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	const source = "payments.jsonl"
	if err := store.AdvanceImportedThroughLine(ctx, source, 12); err != nil {
		t.Fatal(err)
	}
	if err := store.AdvanceImportedThroughLine(ctx, source, 11); err != nil {
		t.Fatal(err)
	}
	line, err := store.ImportedThroughLine(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	if line != 12 {
		t.Fatalf("imported through line = %d, want 12", line)
	}
}

func TestPreparedTransactionAndSubmissionAreIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "sender.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	item := payment.Instruction{ID: "pay-1", From: "alice", To: "bob", Amount: "1.000000", AmountUnits: 1_000_000, RawJSON: []byte(`{"payment_id":"pay-1"}`)}
	if err := store.InsertPayment(ctx, item); err != nil {
		t.Fatal(err)
	}
	candidate := transaction.Signed{ID: "tx-1", PaymentID: item.ID, From: item.From, To: item.To, AmountUnits: item.AmountUnits, Signature: "signature"}
	durable, state, err := store.GetOrCreatePreparedTransaction(ctx, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if state != "prepared" || !transaction.Equal(durable, candidate) {
		t.Fatalf("durable transaction = (%+v, %q)", durable, state)
	}

	conflict := candidate
	conflict.To = "mallory"
	if _, _, err := store.GetOrCreatePreparedTransaction(ctx, conflict); !errors.Is(err, ErrTransactionConflict) {
		t.Fatalf("conflict error = %v, want ErrTransactionConflict", err)
	}
	if err := store.MarkSubmitted(ctx, item.ID, candidate.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSubmitted(ctx, item.ID, candidate.ID); err != nil {
		t.Fatalf("repeat submission transition: %v", err)
	}

	var paymentState, transactionState string
	if err := store.db.QueryRowContext(ctx, `SELECT status FROM payments WHERE id = ?`, item.ID).Scan(&paymentState); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT state FROM transactions WHERE id = ?`, candidate.ID).Scan(&transactionState); err != nil {
		t.Fatal(err)
	}
	if paymentState != string(payment.StatusSubmitted) || transactionState != "submitted" {
		t.Fatalf("states = (%q, %q), want submitted", paymentState, transactionState)
	}
}
