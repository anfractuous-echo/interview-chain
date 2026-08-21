package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"interview-chain/internal/blockchain"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout%3d2000&_pragma=journal_mode%3dWAL"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open indexer database: %w", err)
	}
	db.SetMaxOpenConns(100)
	store := &SQLiteStore{db: db}
	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) init(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS blocks (
    height INTEGER PRIMARY KEY,
    hash TEXT NOT NULL,
    previous_hash TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS indexed_transactions (
    id TEXT PRIMARY KEY,
    block_height INTEGER NOT NULL,
    payment_id TEXT NOT NULL,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    amount_units INTEGER NOT NULL CHECK(amount_units > 0),
    signature TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS indexer_checkpoints (
    name TEXT PRIMARY KEY,
    height INTEGER NOT NULL
);`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize indexer database: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) Checkpoint(ctx context.Context) (int64, error) {
	var height int64
	err := s.db.QueryRowContext(ctx, `SELECT height FROM indexer_checkpoints WHERE name = 'blocks'`).Scan(&height)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read indexer checkpoint: %w", err)
	}
	return height, nil
}

func (s *SQLiteStore) SaveCheckpoint(ctx context.Context, height int64) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO indexer_checkpoints(name, height) VALUES('blocks', ?)
ON CONFLICT(name) DO UPDATE SET height = excluded.height`, height)
	if err != nil {
		return fmt.Errorf("save indexer checkpoint: %w", err)
	}
	return nil
}

func (s *SQLiteStore) TransactionIDs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM indexed_transactions`)
	if err != nil {
		return nil, fmt.Errorf("list indexed transaction IDs: %w", err)
	}
	result := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = struct{}{}
	}
	return result, rows.Err()
}

func (s *SQLiteStore) SaveBlock(ctx context.Context, block blockchain.Block, known map[string]struct{}) error {
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO blocks(height, hash, previous_hash) VALUES(?, ?, ?)`, block.Height, block.Hash, block.PreviousHash); err != nil {
		return fmt.Errorf("insert block %d: %w", block.Height, err)
	}
	for _, tx := range block.Transactions {
		if tx.AmountUnits <= 0 {
			return fmt.Errorf("transaction %s has non-positive amount", tx.ID)
		}
		if _, ok := known[tx.ID]; ok {
			continue
		}
		_, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO indexed_transactions(
    id, block_height, payment_id, from_address, to_address, amount_units, signature
) VALUES(?, ?, ?, ?, ?, ?, ?)`,
			tx.ID, block.Height, tx.PaymentID, tx.From, tx.To, tx.AmountUnits, tx.Signature)
		if err != nil {
			return fmt.Errorf("insert transaction %s: %w", tx.ID, err)
		}
	}
	return nil
}

func (s *SQLiteStore) Counts(ctx context.Context) (blocks, transactions int, err error) {
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM blocks`).Scan(&blocks); err != nil {
		return 0, 0, err
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM indexed_transactions`).Scan(&transactions); err != nil {
		return 0, 0, err
	}
	return blocks, transactions, nil
}
