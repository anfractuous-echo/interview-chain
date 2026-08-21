package sender

import (
	"context"
	"fmt"
	"sync"

	"interview-chain/internal/payment"
)

type PaymentImportStore interface {
	LastImportedLine(context.Context, string) (int64, error)
	SaveImportedLine(context.Context, string, int64) error
	InsertPayment(context.Context, payment.Instruction) error
}

type FileImporter struct {
	store PaymentImportStore
}

func NewFileImporter(store PaymentImportStore) *FileImporter { return &FileImporter{store: store} }

func (i *FileImporter) ImportFile(ctx context.Context, path string) error {
	checkpoint, err := i.store.LastImportedLine(ctx, path)
	if err != nil {
		return err
	}
	entries, err := ReadPaymentFile(ctx, path)
	if err != nil {
		return err
	}

	insertErrors := make(chan error)
	var inserts sync.WaitGroup
	for _, entry := range entries {
		if entry.LineNumber <= checkpoint {
			continue
		}
		if err := i.store.SaveImportedLine(ctx, path, entry.LineNumber); err != nil {
			return err
		}

		inserts.Add(1)
		go func(p payment.Instruction) {
			defer inserts.Done()
			if err := i.store.InsertPayment(ctx, p); err != nil {
				insertErrors <- fmt.Errorf("insert payment %s: %w", p.ID, err)
			}
		}(entry.Instruction)
	}

	go func() {
		inserts.Wait()
		close(insertErrors)
	}()

	var firstErr error
	for err := range insertErrors {
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
