package sender

import (
	"context"

	"interview-chain/internal/payment"
)

type PaymentFileEntry struct {
	LineNumber  int64
	Instruction payment.Instruction
}

// ReadPaymentFile reads payment input from path.
// TODO(candidate): implement.
func ReadPaymentFile(ctx context.Context, path string) ([]PaymentFileEntry, error) {
	return nil, ErrNotImplemented
}
