package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"interview-chain/internal/blockchain"
)

type BlockStore interface {
	Checkpoint(context.Context) (int64, error)
	BeginCheckpointUpdate(context.Context) (CheckpointUpdate, error)
	SaveBlock(context.Context, blockchain.Block) error
}

type CheckpointUpdate interface {
	Checkpoint(context.Context) (int64, error)
	Save(context.Context, int64) error
	Commit() error
	Rollback() error
}

type BlockSource interface {
	Blocks(context.Context, int64, int) (blockchain.BlockPage, error)
}

type Service struct {
	store    BlockStore
	node     BlockSource
	pageSize int

	pollMu sync.Mutex
	stats  runtimeStats
}

type runtimeStats struct {
	blocks       int64
	transactions int64
}

func NewService(store BlockStore, node BlockSource, pageSize int) *Service {
	return &Service{store: store, node: node, pageSize: pageSize}
}

func (s *Service) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := s.PollOnce(ctx); err != nil {
			continue
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Service) PollOnce(ctx context.Context) error {
	s.pollMu.Lock()
	checkpointUpdate, err := s.store.BeginCheckpointUpdate(ctx)
	s.pollMu.Unlock()
	if err != nil {
		return err
	}
	defer checkpointUpdate.Rollback()

	checkpoint, err := checkpointUpdate.Checkpoint(ctx)
	if err != nil {
		return err
	}

	page, err := s.node.Blocks(context.Background(), checkpoint, s.pageSize)
	if err != nil {
		return err
	}
	if len(page.Blocks) == 0 {
		return nil
	}

	nextCheckpoint := page.NextHeight
	if page.HasMore {
		nextCheckpoint++
	}
	if err := checkpointUpdate.Save(ctx, nextCheckpoint); err != nil {
		return err
	}
	if err := checkpointUpdate.Commit(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	var firstErr error
	for _, block := range page.Blocks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.store.SaveBlock(ctx, block); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			s.stats.blocks++
			s.stats.transactions += int64(len(block.Transactions))
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return fmt.Errorf("save page after checkpoint %d: %w", checkpoint, firstErr)
	}
	logIndexedPage(page, nextCheckpoint)
	return nil
}
