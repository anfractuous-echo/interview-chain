package sender

import (
	"context"
	"fmt"
	"log"
	"sync"

	"interview-chain/internal/payment"
)

type PaymentImportStore interface {
	ImportedThroughLine(context.Context, string) (int64, error)
	AdvanceImportedThroughLine(context.Context, string, int64) error
	InsertPayment(context.Context, payment.Instruction) error
}

const paymentImportWorkers = 8

type importResult struct {
	lineNumber int64
	paymentID  string
	err        error
}

type FileImporter struct {
	store PaymentImportStore
}

func NewFileImporter(store PaymentImportStore) *FileImporter { return &FileImporter{store: store} }

func (i *FileImporter) ImportFile(ctx context.Context, path string) error {
	importedThrough, err := i.store.ImportedThroughLine(ctx, path)
	if err != nil {
		return err
	}
	entries, err := ReadPaymentFile(ctx, path)
	if err != nil {
		return err
	}

	jobs := make(chan PaymentFileEntry)
	results := make(chan importResult, paymentImportWorkers)

	var workers sync.WaitGroup
	for worker := 0; worker < paymentImportWorkers; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			i.importPayments(ctx, path, jobs, results)
		}()
	}

	go func() {
		defer close(jobs)
		for _, entry := range entries {
			if entry.LineNumber > importedThrough {
				jobs <- entry
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	imported := 0
	var firstErr error
	for result := range results {
		if result.err != nil {
			log.Printf("payment import failed line=%d payment_id=%s error=%q", result.lineNumber, result.paymentID, result.err)
			if firstErr == nil {
				firstErr = result.err
			}
			continue
		}
		imported++
	}
	log.Printf("imported=%d source=%s", imported, path)
	return firstErr
}

func (i *FileImporter) importPayments(
	ctx context.Context,
	path string,
	jobs <-chan PaymentFileEntry,
	results chan<- importResult,
) {
	for entry := range jobs {
		result := importResult{lineNumber: entry.LineNumber, paymentID: entry.Instruction.ID}
		if err := i.store.InsertPayment(ctx, entry.Instruction); err != nil {
			result.err = fmt.Errorf("insert payment %s: %w", entry.Instruction.ID, err)
		} else if err := i.store.AdvanceImportedThroughLine(ctx, path, entry.LineNumber); err != nil {
			result.err = fmt.Errorf("advance imported through line %d: %w", entry.LineNumber, err)
		}
		results <- result
	}
}
