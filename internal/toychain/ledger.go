package toychain

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"interview-chain/internal/blockchain"
	"interview-chain/internal/transaction"
	"interview-chain/internal/wallet"
)

var (
	ErrInvalidTransaction  = errors.New("invalid transaction")
	ErrInvalidSignature    = errors.New("invalid signature")
	ErrInvalidID           = errors.New("invalid transaction id")
	ErrUnknownAccount      = errors.New("unknown account")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrBalanceOverflow     = errors.New("receiver balance overflow")
	ErrTransactionConflict = errors.New("transaction id conflict")
	ErrTransactionNotFound = errors.New("transaction not found")
)

type includedTransaction struct {
	Transaction transaction.Signed
	BlockHeight int64
}

type Ledger struct {
	mu           sync.RWMutex
	balances     map[string]int64
	projected    map[string]int64
	secrets      map[string]string
	pending      []transaction.Signed
	pendingByID  map[string]transaction.Signed
	transactions map[string]includedTransaction
	blocks       []blockchain.Block
}

func NewLedger(source wallet.Snapshot) *Ledger {
	balances := make(map[string]int64, len(source.Accounts))
	secrets := make(map[string]string, len(source.Accounts))
	for _, account := range source.Accounts {
		balances[account.Address] = account.BalanceUnits
		secrets[account.Address] = account.Secret
	}
	return &Ledger{
		balances:     balances,
		projected:    cloneBalances(balances),
		secrets:      secrets,
		pendingByID:  make(map[string]transaction.Signed),
		transactions: make(map[string]includedTransaction),
	}
}

func (l *Ledger) Submit(tx transaction.Signed) (blockchain.SubmissionResult, error) {
	if tx.ID == "" || tx.PaymentID == "" || tx.From == "" || tx.To == "" || tx.AmountUnits <= 0 || tx.Signature == "" {
		return blockchain.SubmissionResult{}, ErrInvalidTransaction
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if existing, ok := l.transactions[tx.ID]; ok {
		if !transaction.Equal(existing.Transaction, tx) {
			return blockchain.SubmissionResult{}, ErrTransactionConflict
		}
		height := existing.BlockHeight
		return blockchain.SubmissionResult{
			TransactionID: tx.ID,
			Status:        blockchain.TransactionIncluded,
			BlockHeight:   &height,
			Duplicate:     true,
		}, nil
	}
	if existing, ok := l.pendingByID[tx.ID]; ok {
		if !transaction.Equal(existing, tx) {
			return blockchain.SubmissionResult{}, ErrTransactionConflict
		}
		return blockchain.SubmissionResult{
			TransactionID: tx.ID,
			Status:        blockchain.TransactionPending,
			Duplicate:     true,
		}, nil
	}

	secret, ok := l.secrets[tx.From]
	if !ok {
		return blockchain.SubmissionResult{}, fmt.Errorf("sender %q: %w", tx.From, ErrUnknownAccount)
	}
	if _, ok := l.balances[tx.To]; !ok {
		return blockchain.SubmissionResult{}, fmt.Errorf("receiver %q: %w", tx.To, ErrUnknownAccount)
	}
	expectedSignature, err := calculateExpectedSignature(tx, secret)
	if err != nil || tx.Signature != expectedSignature {
		return blockchain.SubmissionResult{}, ErrInvalidSignature
	}
	expectedID, err := calculateExpectedID(tx)
	if err != nil || tx.ID != expectedID {
		return blockchain.SubmissionResult{}, ErrInvalidID
	}
	if l.projected[tx.From] < tx.AmountUnits {
		return blockchain.SubmissionResult{}, ErrInsufficientFunds
	}
	if tx.From != tx.To && l.projected[tx.To] > math.MaxInt64-tx.AmountUnits {
		return blockchain.SubmissionResult{}, ErrBalanceOverflow
	}

	l.projected[tx.From] -= tx.AmountUnits
	l.projected[tx.To] += tx.AmountUnits
	l.pending = append(l.pending, tx)
	l.pendingByID[tx.ID] = tx
	return blockchain.SubmissionResult{
		TransactionID: tx.ID,
		Status:        blockchain.TransactionPending,
	}, nil
}

func (l *Ledger) MineBlock() blockchain.Block {
	l.mu.Lock()
	defer l.mu.Unlock()

	height := int64(len(l.blocks) + 1)
	previousHash := ""
	if len(l.blocks) > 0 {
		previousHash = l.blocks[len(l.blocks)-1].Hash
	}
	transactions := append([]transaction.Signed{}, l.pending...)
	transactionIDs := make([]string, len(transactions))
	for index, tx := range transactions {
		transactionIDs[index] = tx.ID
		l.balances[tx.From] -= tx.AmountUnits
		l.balances[tx.To] += tx.AmountUnits
		l.transactions[tx.ID] = includedTransaction{Transaction: tx, BlockHeight: height}
		delete(l.pendingByID, tx.ID)
	}
	block := blockchain.Block{
		Height:       height,
		Hash:         blockchain.CalculateBlockHash(previousHash, height, transactionIDs...),
		PreviousHash: previousHash,
		Transactions: transactions,
	}
	l.blocks = append(l.blocks, block)
	l.pending = nil
	return block
}

func (l *Ledger) Account(address string) (blockchain.AccountBalance, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	balance, ok := l.balances[address]
	if !ok {
		return blockchain.AccountBalance{}, ErrUnknownAccount
	}
	return blockchain.AccountBalance{Address: address, BalanceUnits: balance}, nil
}

func (l *Ledger) Transaction(id string) (blockchain.TransactionResult, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if pending, ok := l.pendingByID[id]; ok {
		return blockchain.TransactionResult{
			Transaction: pending,
			Status:      blockchain.TransactionPending,
		}, nil
	}
	included, ok := l.transactions[id]
	if !ok {
		return blockchain.TransactionResult{}, ErrTransactionNotFound
	}
	height := included.BlockHeight
	return blockchain.TransactionResult{
		Transaction: included.Transaction,
		Status:      blockchain.TransactionIncluded,
		BlockHeight: &height,
	}, nil
}

func (l *Ledger) BlocksAfter(after int64, limit int) blockchain.BlockPage {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if after < 0 {
		after = 0
	}
	start := int(after)
	if start > len(l.blocks) {
		start = len(l.blocks)
	}
	end := start + limit
	if end > len(l.blocks) {
		end = len(l.blocks)
	}
	blocks := append([]blockchain.Block{}, l.blocks[start:end]...)
	next := after
	if len(blocks) > 0 {
		next = blocks[len(blocks)-1].Height
	}
	return blockchain.BlockPage{Blocks: blocks, NextHeight: next, HasMore: end < len(l.blocks)}
}

func cloneBalances(source map[string]int64) map[string]int64 {
	result := make(map[string]int64, len(source))
	for address, balance := range source {
		result[address] = balance
	}
	return result
}
