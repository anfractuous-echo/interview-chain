package sender

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadPaymentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payments.jsonl")
	firstLine := `{"payment_id":"pay-1","from":"alice","to":"bob","amount":"1.250000"}`
	secondLine := `{"payment_id":"pay-max","from":"alice","to":"bob","amount":"9223372036854.775807"}`
	if err := os.WriteFile(path, []byte(firstLine+"\n"+secondLine+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := ReadPaymentFile(context.Background(), path)
	if errors.Is(err, ErrNotImplemented) {
		t.Skip("candidate exercise is not implemented")
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].LineNumber != 1 || entries[0].Instruction.AmountUnits != 1_250_000 {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if string(entries[0].Instruction.RawJSON) != firstLine {
		t.Fatalf("unexpected raw JSON: %q", entries[0].Instruction.RawJSON)
	}
	if entries[1].LineNumber != 2 || entries[1].Instruction.AmountUnits != math.MaxInt64 {
		t.Fatalf("unexpected maximum amount entry: %+v", entries[1])
	}
}

func TestReadPaymentFileReportsInvalidAmountLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payments.jsonl")
	line := `{"payment_id":"pay-1","from":"alice","to":"bob","amount":"1.0000001"}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ReadPaymentFile(context.Background(), path)
	if errors.Is(err, ErrNotImplemented) {
		t.Skip("candidate exercise is not implemented")
	}
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("got error %v, want an error containing line 1", err)
	}
}
