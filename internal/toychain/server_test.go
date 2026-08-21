package toychain

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"interview-chain/internal/blockchain"
)

func TestPostTransactionIsIdempotent(t *testing.T) {
	t.Parallel()
	ledger := NewLedger(testWallet())
	server := httptest.NewServer(NewServer(ledger).Handler())
	defer server.Close()
	tx := signedTransaction(t, "pay-http", 25)
	body, err := json.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	for attempt, wantStatus := range []int{http.StatusAccepted, http.StatusOK} {
		response, err := http.Post(server.URL+"/transactions", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != wantStatus {
			t.Fatalf("attempt %d status = %d, want %d", attempt+1, response.StatusCode, wantStatus)
		}
	}

	pending := getTransaction(t, server.URL, tx.ID)
	if pending.Status != blockchain.TransactionPending || pending.BlockHeight != nil {
		t.Fatalf("unexpected pending transaction: %+v", pending)
	}
	if page := ledger.BlocksAfter(0, 10); len(page.Blocks) != 0 {
		t.Fatalf("POST mined %d blocks, want 0", len(page.Blocks))
	}
	ledger.MineBlock()
	included := getTransaction(t, server.URL, tx.ID)
	if included.Status != blockchain.TransactionIncluded || included.BlockHeight == nil || *included.BlockHeight != 1 {
		t.Fatalf("unexpected included transaction: %+v", included)
	}
}

func getTransaction(t *testing.T, baseURL, transactionID string) blockchain.TransactionResult {
	t.Helper()
	response, err := http.Get(baseURL + "/transactions/" + transactionID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("transaction status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var result blockchain.TransactionResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}
