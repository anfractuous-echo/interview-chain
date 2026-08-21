package sender

import (
	"context"

	"interview-chain/internal/blockchain"
	"interview-chain/internal/payment"
	"interview-chain/internal/transaction"
)

type TransactionStore interface {
	FindTransactionByPaymentID(context.Context, string) (transaction.Signed, string, bool, error)
	GetOrCreatePreparedTransaction(context.Context, transaction.Signed) (transaction.Signed, string, error)
	MarkSubmitted(context.Context, string, string) error
}

type TransactionSubmitter interface {
	Submit(context.Context, transaction.Signed) (blockchain.SubmissionResult, error)
}

type Processor struct {
	store       TransactionStore
	node        TransactionSubmitter
	signingKeys map[string]string
}

func NewProcessor(store TransactionStore, node TransactionSubmitter, signingKeys map[string]string) *Processor {
	return &Processor{store: store, node: node, signingKeys: signingKeys}
}

// BuildSignedTransaction converts one payment into the exact transaction defined in BLOCKCHAIN.md.
// TODO(candidate): implement materialization, the toy signature, and the stable transaction ID.
func BuildSignedTransaction(item payment.Instruction, signingKey string) (transaction.Signed, error) {
	return transaction.Signed{}, ErrNotImplemented
}

// ProcessPayment must materialize, durably store, submit, and record one payment.
// TODO(candidate): implement the orchestration while considering replay and partial failure.
func (p *Processor) ProcessPayment(ctx context.Context, item payment.Instruction) error {
	return ErrNotImplemented
}
