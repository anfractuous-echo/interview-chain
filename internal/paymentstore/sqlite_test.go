package paymentstore

import (
	"context"
	"path/filepath"
	"testing"

	"interview-chain/internal/payment"
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
	payments, err := store.ListReceivedPayments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 1 || payments[0].ID != item.ID {
		t.Fatalf("unexpected payments: %+v", payments)
	}
}
