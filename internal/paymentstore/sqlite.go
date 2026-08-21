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
    status TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS transactions (
    id TEXT PRIMARY KEY,
    payment_id TEXT NOT NULL UNIQUE,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    amount_units INTEGER NOT NULL,
    signature TEXT NOT NULL,
    state TEXT NOT NULL
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

func (s *SQLiteStore) LastImportedLine(ctx context.Context, sourcePath string) (int64, error) {
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

func (s *SQLiteStore) SaveImportedLine(ctx context.Context, sourcePath string, line int64) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO checkpoints(source, line_number) VALUES(?, ?)
ON CONFLICT(source) DO UPDATE SET line_number = excluded.line_number`, sourcePath, line)
	if err != nil {
		return fmt.Errorf("save import checkpoint: %w", err)
	}
	return nil
}

func (s *SQLiteStore) InsertPayment(ctx context.Context, item payment.Instruction) error {
	_, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO payments(
    id, from_address, to_address, amount_text, amount_units, raw_payload, status
) VALUES(?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.From, item.To, item.Amount, item.AmountUnits, item.RawJSON, payment.StatusReceived)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListReceivedPayments(ctx context.Context) ([]payment.Instruction, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, from_address, to_address, amount_text, amount_units, raw_payload, status
FROM payments WHERE status = ? ORDER BY id`, payment.StatusReceived)
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

func (s *SQLiteStore) SavePreparedTransaction(ctx context.Context, tx transaction.Signed) error {
	_, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO transactions(
    id, payment_id, from_address, to_address, amount_units, signature, state
) VALUES(?, ?, ?, ?, ?, ?, 'prepared')`,
		tx.ID, tx.PaymentID, tx.From, tx.To, tx.AmountUnits, tx.Signature)
	if err != nil {
		return fmt.Errorf("save prepared transaction: %w", err)
	}
	return nil
}

func (s *SQLiteStore) MarkSubmitted(ctx context.Context, paymentID, transactionID string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE transactions SET state = 'submitted' WHERE id = ?`, transactionID); err != nil {
		return fmt.Errorf("mark transaction submitted: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE payments SET status = ? WHERE id = ?`, payment.StatusSubmitted, paymentID); err != nil {
		return fmt.Errorf("mark payment submitted: %w", err)
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
