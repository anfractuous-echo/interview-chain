package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"interview-chain/internal/blockchain"
	"interview-chain/internal/transaction"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

var (
	ErrBlockConflict      = errors.New("block conflicts with indexed block")
	ErrIndexedTransaction = errors.New("transaction conflicts with indexed transaction")
)

func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout%3d2000&_pragma=journal_mode%3dWAL"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open indexer database: %w", err)
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

type sqliteCheckpointUpdate struct {
	tx *sql.Tx
}

func (s *SQLiteStore) BeginCheckpointUpdate(ctx context.Context) (CheckpointUpdate, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin checkpoint update: %w", err)
	}
	return &sqliteCheckpointUpdate{tx: tx}, nil
}

func (u *sqliteCheckpointUpdate) Checkpoint(ctx context.Context) (int64, error) {
	var height int64
	err := u.tx.QueryRowContext(ctx, `SELECT height FROM indexer_checkpoints WHERE name = 'blocks'`).Scan(&height)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read checkpoint update: %w", err)
	}
	return height, nil
}

func (u *sqliteCheckpointUpdate) Save(ctx context.Context, height int64) error {
	_, err := u.tx.ExecContext(ctx, `
INSERT INTO indexer_checkpoints(name, height) VALUES('blocks', ?)
ON CONFLICT(name) DO UPDATE SET height = excluded.height`, height)
	if err != nil {
		return fmt.Errorf("save indexer checkpoint: %w", err)
	}
	return nil
}

func (u *sqliteCheckpointUpdate) Commit() error {
	if err := u.tx.Commit(); err != nil {
		return fmt.Errorf("commit checkpoint update: %w", err)
	}
	return nil
}

func (u *sqliteCheckpointUpdate) Rollback() error {
	return u.tx.Rollback()
}

func (s *SQLiteStore) SaveBlock(ctx context.Context, block blockchain.Block) error {
	if err := validateBlock(block); err != nil {
		return err
	}

	dbTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin block %d: %w", block.Height, err)
	}
	defer dbTx.Rollback()

	result, err := dbTx.ExecContext(ctx, `
INSERT INTO blocks(height, hash, previous_hash) VALUES(?, ?, ?)
ON CONFLICT(height) DO NOTHING`, block.Height, block.Hash, block.PreviousHash)
	if err != nil {
		return fmt.Errorf("insert block %d: %w", block.Height, err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check inserted block %d: %w", block.Height, err)
	}
	if inserted == 0 {
		var storedHash, storedPreviousHash string
		if err := dbTx.QueryRowContext(ctx, `SELECT hash, previous_hash FROM blocks WHERE height = ?`, block.Height).Scan(&storedHash, &storedPreviousHash); err != nil {
			return fmt.Errorf("read existing block %d: %w", block.Height, err)
		}
		if storedHash != block.Hash || storedPreviousHash != block.PreviousHash {
			return fmt.Errorf("%w: height %d", ErrBlockConflict, block.Height)
		}
	}

	for _, item := range block.Transactions {
		result, err := dbTx.ExecContext(ctx, `
INSERT INTO indexed_transactions(
    id, block_height, payment_id, from_address, to_address, amount_units, signature
) VALUES(?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING`,
			item.ID, block.Height, item.PaymentID, item.From, item.To, item.AmountUnits, item.Signature)
		if err != nil {
			return fmt.Errorf("insert transaction %s: %w", item.ID, err)
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check inserted transaction %s: %w", item.ID, err)
		}
		if inserted == 0 {
			stored, storedHeight, err := readIndexedTransaction(ctx, dbTx, item.ID)
			if err != nil {
				return err
			}
			if storedHeight != block.Height || !transaction.Equal(stored, item) {
				return fmt.Errorf("%w: %s", ErrIndexedTransaction, item.ID)
			}
		}
	}
	if err := dbTx.Commit(); err != nil {
		return fmt.Errorf("commit block %d: %w", block.Height, err)
	}
	return nil
}

func validateBlock(block blockchain.Block) error {
	if block.Height <= 0 || block.Hash == "" {
		return fmt.Errorf("block has invalid identity")
	}
	transactionIDs := make([]string, 0, len(block.Transactions))
	seen := make(map[string]struct{}, len(block.Transactions))
	for _, item := range block.Transactions {
		if item.ID == "" || item.PaymentID == "" || item.From == "" || item.To == "" || item.Signature == "" || item.AmountUnits <= 0 {
			return fmt.Errorf("transaction %s is invalid", item.ID)
		}
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("block %d contains duplicate transaction %s", block.Height, item.ID)
		}
		seen[item.ID] = struct{}{}
		transactionIDs = append(transactionIDs, item.ID)
	}
	expectedHash := blockchain.CalculateBlockHash(block.PreviousHash, block.Height, transactionIDs...)
	if block.Hash != expectedHash {
		return fmt.Errorf("block %d has invalid hash", block.Height)
	}
	return nil
}

func readIndexedTransaction(ctx context.Context, dbTx *sql.Tx, transactionID string) (transaction.Signed, int64, error) {
	var item transaction.Signed
	var blockHeight int64
	err := dbTx.QueryRowContext(ctx, `
SELECT id, block_height, payment_id, from_address, to_address, amount_units, signature
FROM indexed_transactions WHERE id = ?`, transactionID).Scan(
		&item.ID, &blockHeight, &item.PaymentID, &item.From, &item.To, &item.AmountUnits, &item.Signature,
	)
	if err != nil {
		return transaction.Signed{}, 0, fmt.Errorf("read indexed transaction %s: %w", transactionID, err)
	}
	return item, blockHeight, nil
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
