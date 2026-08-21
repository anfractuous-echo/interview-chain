package paymentstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"interview-chain/internal/payment"
	"interview-chain/internal/transaction"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

var (
	ErrPaymentConflict     = errors.New("payment ID conflicts with stored payment")
	ErrTransactionConflict = errors.New("transaction conflicts with stored transaction")
)

func OpenSQLite(path string) (*SQLiteStore, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout%3d2000&_pragma=journal_mode%3dWAL"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open payment database: %w", err)
	}
	db.SetMaxOpenConns(8)
	store := &SQLiteStore{db: db}
	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) init(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS payments (
    id TEXT PRIMARY KEY,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    amount_text TEXT NOT NULL,
    amount_units INTEGER NOT NULL,
    raw_payload BLOB NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('received', 'submitted'))
);
CREATE TABLE IF NOT EXISTS transactions (
    id TEXT PRIMARY KEY,
    payment_id TEXT NOT NULL UNIQUE,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    amount_units INTEGER NOT NULL,
    signature TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('prepared', 'submitted'))
);
CREATE TABLE IF NOT EXISTS checkpoints (
    source TEXT PRIMARY KEY,
    line_number INTEGER NOT NULL
);`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize payment database: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) ImportedThroughLine(ctx context.Context, sourcePath string) (int64, error) {
	var line int64
	err := s.db.QueryRowContext(ctx, `SELECT line_number FROM checkpoints WHERE source = ?`, sourcePath).Scan(&line)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read import checkpoint: %w", err)
	}
	return line, nil
}

func (s *SQLiteStore) AdvanceImportedThroughLine(ctx context.Context, sourcePath string, line int64) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO checkpoints(source, line_number) VALUES(?, ?)
	ON CONFLICT(source) DO UPDATE SET line_number = excluded.line_number
	WHERE excluded.line_number > checkpoints.line_number`, sourcePath, line)
	if err != nil {
		return fmt.Errorf("advance imported-through line: %w", err)
	}
	return nil
}

func (s *SQLiteStore) InsertPayment(ctx context.Context, item payment.Instruction) error {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO payments(
    id, from_address, to_address, amount_text, amount_units, raw_payload, status
) VALUES(?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING`,
		item.ID, item.From, item.To, item.Amount, item.AmountUnits, item.RawJSON, payment.StatusReceived)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check inserted payment: %w", err)
	}
	if inserted == 1 {
		return nil
	}

	stored, found, err := s.findPayment(ctx, item.ID)
	if err != nil {
		return err
	}
	if !found || !samePayment(stored, item) {
		return fmt.Errorf("%w: %s", ErrPaymentConflict, item.ID)
	}
	return nil
}

func (s *SQLiteStore) findPayment(ctx context.Context, paymentID string) (payment.Instruction, bool, error) {
	var item payment.Instruction
	err := s.db.QueryRowContext(ctx, `
SELECT id, from_address, to_address, amount_text, amount_units, raw_payload, status
FROM payments WHERE id = ?`, paymentID).Scan(
		&item.ID, &item.From, &item.To, &item.Amount, &item.AmountUnits, &item.RawJSON, &item.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return payment.Instruction{}, false, nil
	}
	if err != nil {
		return payment.Instruction{}, false, fmt.Errorf("find payment: %w", err)
	}
	return item, true, nil
}

func samePayment(left, right payment.Instruction) bool {
	return left.ID == right.ID &&
		left.From == right.From &&
		left.To == right.To &&
		left.AmountUnits == right.AmountUnits
}

func (s *SQLiteStore) ListReceivedPayments(ctx context.Context, afterID string, limit int) ([]payment.Instruction, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("payment batch limit must be positive")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, from_address, to_address, amount_text, amount_units, raw_payload, status
FROM payments
WHERE status = ? AND id > ?
ORDER BY id
LIMIT ?`, payment.StatusReceived, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list received payments: %w", err)
	}
	defer rows.Close()
	var result []payment.Instruction
	for rows.Next() {
		var item payment.Instruction
		if err := rows.Scan(&item.ID, &item.From, &item.To, &item.Amount, &item.AmountUnits, &item.RawJSON, &item.Status); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) FindTransactionByPaymentID(ctx context.Context, paymentID string) (transaction.Signed, string, bool, error) {
	var tx transaction.Signed
	var state string
	err := s.db.QueryRowContext(ctx, `
SELECT id, payment_id, from_address, to_address, amount_units, signature, state
FROM transactions WHERE payment_id = ?`, paymentID).Scan(
		&tx.ID, &tx.PaymentID, &tx.From, &tx.To, &tx.AmountUnits, &tx.Signature, &state,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return transaction.Signed{}, "", false, nil
	}
	if err != nil {
		return transaction.Signed{}, "", false, fmt.Errorf("find transaction: %w", err)
	}
	return tx, state, true, nil
}

func (s *SQLiteStore) GetOrCreatePreparedTransaction(ctx context.Context, candidate transaction.Signed) (transaction.Signed, string, error) {
	dbTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return transaction.Signed{}, "", fmt.Errorf("begin prepared transaction: %w", err)
	}
	defer dbTx.Rollback()

	if _, err := dbTx.ExecContext(ctx, `
INSERT INTO transactions(
    id, payment_id, from_address, to_address, amount_units, signature, state
) VALUES(?, ?, ?, ?, ?, ?, 'prepared')
ON CONFLICT DO NOTHING`,
		candidate.ID, candidate.PaymentID, candidate.From, candidate.To, candidate.AmountUnits, candidate.Signature); err != nil {
		return transaction.Signed{}, "", fmt.Errorf("save prepared transaction: %w", err)
	}

	var stored transaction.Signed
	var state string
	err = dbTx.QueryRowContext(ctx, `
SELECT id, payment_id, from_address, to_address, amount_units, signature, state
FROM transactions WHERE payment_id = ?`, candidate.PaymentID).Scan(
		&stored.ID, &stored.PaymentID, &stored.From, &stored.To, &stored.AmountUnits, &stored.Signature, &state,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return transaction.Signed{}, "", fmt.Errorf("%w: transaction ID %s", ErrTransactionConflict, candidate.ID)
	}
	if err != nil {
		return transaction.Signed{}, "", fmt.Errorf("read prepared transaction: %w", err)
	}
	if !transaction.Equal(stored, candidate) {
		return transaction.Signed{}, "", fmt.Errorf("%w: payment %s", ErrTransactionConflict, candidate.PaymentID)
	}
	if err := dbTx.Commit(); err != nil {
		return transaction.Signed{}, "", fmt.Errorf("commit prepared transaction: %w", err)
	}
	return stored, state, nil
}

func (s *SQLiteStore) MarkSubmitted(ctx context.Context, paymentID, transactionID string) error {
	dbTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin submitted transition: %w", err)
	}
	defer dbTx.Rollback()

	result, err := dbTx.ExecContext(ctx, `
UPDATE transactions
SET state = 'submitted'
WHERE id = ? AND payment_id = ? AND state IN ('prepared', 'submitted')`, transactionID, paymentID)
	if err != nil {
		return fmt.Errorf("mark transaction submitted: %w", err)
	}
	if err := requireOneRow(result, "transaction", transactionID); err != nil {
		return err
	}

	result, err = dbTx.ExecContext(ctx, `
UPDATE payments
SET status = ?
WHERE id = ? AND status IN (?, ?)`, payment.StatusSubmitted, paymentID, payment.StatusReceived, payment.StatusSubmitted)
	if err != nil {
		return fmt.Errorf("mark payment submitted: %w", err)
	}
	if err := requireOneRow(result, "payment", paymentID); err != nil {
		return err
	}

	if err := dbTx.Commit(); err != nil {
		return fmt.Errorf("commit submitted transition: %w", err)
	}
	return nil
}

func requireOneRow(result sql.Result, kind, id string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated %s %s: %w", kind, id, err)
	}
	if rows != 1 {
		return fmt.Errorf("%s %s is missing or in an unexpected state", kind, id)
	}
	return nil
}

func (s *SQLiteStore) Counts(ctx context.Context) (payments, transactions int, err error) {
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payments`).Scan(&payments); err != nil {
		return 0, 0, err
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&transactions); err != nil {
		return 0, 0, err
	}
	return payments, transactions, nil
}
